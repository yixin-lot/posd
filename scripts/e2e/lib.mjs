// E2E 公共库：启动 posd + vite，起无头 Chrome，提供最小 CDP 封装。
//
// 用法：
//   node scripts/e2e/smoke.mjs
//   node scripts/e2e/import.mjs
//
// 约定：脚本自己拉进程、自己清理；失败退出码非 0。
import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, mkdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import net from 'node:net'

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const CHROME = process.env.E2E_CHROME || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
const ROOT = process.env.E2E_ROOT || process.cwd()
const POSD = process.env.E2E_POSD || join(ROOT, 'dist', 'posd.exe')

/** 端口是否可连接。 */
function isPortOpen(port) {
  return new Promise((resolve) => {
    const sock = net.createConnection({ host: '127.0.0.1', port }, () => {
      sock.destroy()
      resolve(true)
    })
    sock.on('error', () => resolve(false))
    setTimeout(() => { try { sock.destroy() } catch {} ; resolve(false) }, 1000)
  })
}

async function waitPort(port, timeoutMs = 30000) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    const ok = await new Promise((res) => {
      const s = net.connect(port, '127.0.0.1')
      s.on('connect', () => { s.destroy(); res(true) })
      s.on('error', () => res(false))
      s.setTimeout(500, () => { s.destroy(); res(false) })
    })
    if (ok) return true
    await sleep(300)
  }
  return false
}

/** 起 posd（临时库）+ vite，返回 { stop() } */
/** 等端口释放（上一个用例的 posd 可能还没退干净）。 */
async function waitPortClosed(port, timeoutMs = 8000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (!(await isPortOpen(port))) return true
    await sleep(200)
  }
  return false
}

export async function launchStack({ withPrinter = false } = {}) {
  // 端口没释放就起新实例，会连到上一个进程的库 —— 用例会随机失败。
  // Windows 上 posd.kill() 常常杀不掉，这里先按端口强杀一次（E2E 专用）。
  if (process.platform === 'win32') {
    // Windows 上 posd 常被 node 遗留为孤儿；按端口找到 PID，用 taskkill /T 连子进程一起杀
    for (const port of [8080, 5173]) {
      for (let attempt = 0; attempt < 3; attempt++) {
        const out = spawnSync('powershell', ['-NoProfile', '-Command',
          `(Get-NetTCPConnection -LocalPort ${port} -State Listen -ErrorAction SilentlyContinue).OwningProcess`],
          { encoding: 'utf8' })
        const pids = String(out.stdout || '').split(/\s+/).filter(Boolean)
        if (!pids.length) break
        for (const pid of pids) {
          if (pid === '0') continue
          spawnSync('taskkill', ['/PID', pid, '/T', '/F'], { stdio: 'ignore' })
        }
        await sleep(400)
      }
    }
  }
  const free8080 = await waitPortClosed(8080, 20000)
  const free5173 = await waitPortClosed(5173, 20000)
  if (!free8080 || !free5173) {
    throw new Error('端口仍被占用（8080=' + free8080 + ', 5173=' + free5173 + '），先结束残留的 posd/vite')
  }
  const dir = mkdtempSync(join(tmpdir(), 'pos-e2e-'))
  const args = ['--addr', '127.0.0.1:8080', '--db', join(dir, 'pos.db')]
  if (withPrinter) args.push('--printer', 'POS-58')
  const posd = spawn(POSD, args, { stdio: 'ignore' })
  const vite = spawn(process.execPath, [join(ROOT, 'web', 'node_modules', 'vite', 'bin', 'vite.js'), '--port', '5173'], {
    cwd: join(ROOT, 'web'), stdio: 'ignore'
  })
  if (!(await waitPort(8080)) || !(await waitPort(5173))) {
    posd.kill(); vite.kill()
    throw new Error('服务没起来：posd 8080 / vite 5173')
  }
  // 关键：确认 8080 上跑的就是本次 spawn 的实例（不是残留进程）
  try {
    const info = await (await fetch('http://127.0.0.1:8080/version')).json()
    const want = String(info.db || '').toLowerCase()
    const got = join(dir, 'pos.db').toLowerCase()
    if (want && want !== got) {
      posd.kill(); vite.kill()
      throw new Error('连到了别的实例：期望 ' + got + '，实际 ' + want + '（残留进程未清干净）')
    }
  } catch (e) {
    if (String(e.message).includes('连到了别的实例')) throw e
  }
  await sleep(1200)
  return {
    dataDir: dir,
    async stop() {
      try { posd.kill() } catch {}
      try { vite.kill() } catch {}
      // Windows 上 kill 可能只杀父进程；确认端口没释放就强杀进程树
      await sleep(800)
      if (!(await waitPortClosed(8080, 1500))) {
        try { spawn('taskkill', ['/PID', String(posd.pid), '/T', '/F'], { stdio: 'ignore' }) } catch {}
      }
      if (!(await waitPortClosed(5173, 1500))) {
        try { spawn('taskkill', ['/PID', String(vite.pid), '/T', '/F'], { stdio: 'ignore' }) } catch {}
      }
      await waitPortClosed(8080, 15000)
      await waitPortClosed(5173, 15000)
      try { rmSync(dir, { recursive: true, force: true }) } catch {}
    }
  }
}

/** 起无头 Chrome 并连上页面，返回 { ev, click, setValue, setFile, shot, close } */
export async function openPage(port = 9500) {
  const profile = mkdtempSync(join(tmpdir(), 'pos-e2e-prof-'))
  const chrome = spawn(CHROME, [
    '--headless=new', '--disable-gpu', '--hide-scrollbars',
    `--remote-debugging-port=${port}`, '--window-size=1440,900',
    `--user-data-dir=${profile}`, 'http://127.0.0.1:5173/'
  ], { stdio: 'ignore' })
  // 不把子进程句柄挂在事件循环上：用例结束后 node 能正常退出
  try { chrome.unref() } catch {}

  let target = null
  for (let i = 0; i < 80 && !target; i++) {
    await sleep(250)
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()
      target = list.find((x) => x.type === 'page' && x.url.startsWith('http://127.0.0.1:5173'))
    } catch {}
  }
  if (!target) { chrome.kill(); throw new Error('Chrome 没起来') }

  const ws = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej })
  let id = 0
  const pending = new Map()
  ws.onmessage = (e) => {
    const m = JSON.parse(e.data)
    if (m.id && pending.has(m.id)) {
      const { res, rej } = pending.get(m.id)
      pending.delete(m.id)
      m.error ? rej(new Error(JSON.stringify(m.error))) : res(m.result)
    }
  }
  const send = (method, params) => {
    const mid = ++id
    ws.send(JSON.stringify({ id: mid, method, params: params || {} }))
    return new Promise((res, rej) => pending.set(mid, { res, rej }))
  }
  await send('DOM.enable')

  const ev = async (expr) => {
    const r = await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })
    return r.result ? r.result.value : null
  }
  const setValue = async (selector, value, index = 0) => ev(`(() => {
    const el = document.querySelectorAll('${selector}')[${index}]
    if (!el) return false
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    set.call(el, ${JSON.stringify(value)})
    el.dispatchEvent(new Event('input', { bubbles: true }))
    return true
  })()`)
  const click = async (text) => ev(`(() => {
    const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim().includes(${JSON.stringify(text)}))
    if (!b || b.disabled) return false
    b.click(); return true
  })()`)
  const clickSelector = async (sel) => ev(`(() => { const el = document.querySelector(${JSON.stringify(sel)}); if (!el) return false; el.click(); return true })()`)
  const setFile = async (selector, file) => {
    const doc = await send('DOM.getDocument')
    const node = await send('DOM.querySelector', { nodeId: doc.root.nodeId, selector })
    if (!node.nodeId) return false
    await send('DOM.setFileInputFiles', { files: [file], nodeId: node.nodeId })
    return true
  }
  const shot = async (file) => {
    const s = await send('Page.captureScreenshot', { format: 'png' })
    const { writeFileSync } = await import('node:fs')
    writeFileSync(file, Buffer.from(s.data, 'base64'))
  }
  const close = () => { try { ws.close() } catch {}; try { chrome.kill() } catch {} }

  return { ev, click, clickSelector, setValue, setFile, shot, close, send }
}

/** 登录（默认 admin/1234） */
export async function login(page, user = 'admin', pin = '1234') {

  for (let i = 0; i < 40; i++) {
    if (await page.ev("!!document.querySelector('.login-btn')")) break
    await sleep(300)
  }
  await page.setValue('.login-field input', user, 0)
  await page.setValue('.login-field input', pin, 1)
  await page.clickSelector('.login-btn')
  await sleep(2500)
}


/** 断言：不满足就退出码 1 */
export function assert(cond, msg) {
  if (!cond) {
    console.error('✗ ' + msg)
    process.exitCode = 1
    throw new Error(msg)
  }
  console.log('✓ ' + msg)
}


/** 点顶栏页签：常驻页签直接点，收进「更多」的先展开再点。 */
export async function clickTab(page, label) {
  /* v03 导航有三套入口，脚本按同一口径切页：
     1) 状态条里隐藏的 .tabs（收银台前三项，保留给脚本/快捷键）
     2) 管理页左侧 .nav64
     3) 收银台右上「☰ 菜单」抽屉 .menu-drawer .mi
     文案按「包含」匹配：脚本写"商品"也能点中"商品管理"。 */
  const clickIn = (sel) => page.ev(`(() => {
    const b = [...document.querySelectorAll(${JSON.stringify(sel)})]
      .find((x) => x.textContent.includes(${JSON.stringify(label)}))
    if (!b) return false
    b.click(); return true
  })()`)

  if (await clickIn('.tabs button')) return true
  if (await clickIn('.nav64 button, .nav64 a')) return true

  const opened = await page.ev(`(() => {
    const b = document.querySelector('.more-btn') || document.querySelector('.more-nav summary')
    if (!b) return false
    b.click(); return true
  })()`)
  if (!opened) return false
  await sleep(250)
  return await clickIn('.menu-drawer .mi, .more-menu button')
}



/**
 * 打开应用并登录（所有 E2E 的统一入口）。
 *
 * 为什么需要它：Chrome profile 会复用，页面可能带着**上次运行的旧 token**
 * 直接进入"已登录"状态；此时 login() 找不到登录框、静默失败，
 * 后续所有请求 401，看起来像服务端问题。所以先清 token 再重载。
 */
export async function openApp(page, user = 'admin', pin = '1234') {
  await page.ev('localStorage.clear()')
  await page.ev('location.reload()')
  await sleep(1200)
  await login(page, user, pin)
  // 首跑会弹初始化向导，遮住主界面：点掉它再继续，否则各页面读不到数据
  for (let i = 0; i < 6; i++) {
    const closed = await page.ev(`(() => {
      const b = [...document.querySelectorAll('button')].find((x) => ['跳过', '完成', '开始使用'].includes(x.textContent.trim()))
      if (!b || b.disabled) return false
      b.click()
      return true
    })()`)
    if (!closed) break
    await sleep(400)
  }
  await sleep(600)
}
