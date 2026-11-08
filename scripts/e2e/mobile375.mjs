// 本版由「只采集」改为「会失败」：白名单里的关键元素只要有一块越出视口，进程就 exit 1。
// 白名单外溢出的元素仍只记录、不判失败（装饰与容器误报太多）。
// 跑法：node scripts/e2e/mobile375.mjs
import { launchStack, openPage, openApp, clickTab, sleep, assert } from './lib.mjs'

const W = 375
const H = 812

// 关键元素白名单：手机上真正要点的东西，必须整块落在视口内。
// 商品行内的「库存/编辑/下架/删除」、报表的「日结/月结/今天/昨天/近 7 天」。
const KEY_TARGETS = {
  商品页: '.orders-table .ops button',
  报表页: '.quick button'
}

const stack = await launchStack()
const page = await openPage()
const problems = []
const failures = []
let failed = false

// 采集：横向滚动、越界元素、过小按钮、过小字号
const PROBE = `(() => {
  const vw = window.innerWidth
  const de = document.documentElement
  const overs = [...document.querySelectorAll('*')]
    .map((el) => ({ el, r: el.getBoundingClientRect() }))
    .filter((o) => o.r.width > 0 && o.r.right > vw + 1)
    .map((o) => ({
      tag: o.el.tagName.toLowerCase(),
      cls: String(o.el.className || '').slice(0, 44),
      w: Math.round(o.r.width),
      right: Math.round(o.r.right)
    }))
  const smalls = [...document.querySelectorAll('button')]
    .map((b) => ({ b, r: b.getBoundingClientRect() }))
    .filter((o) => o.r.width > 0 && o.r.height > 0 && (o.r.width < 40 || o.r.height < 40))
    .map((o) => ({ text: o.b.textContent.trim().slice(0, 10), w: Math.round(o.r.width), h: Math.round(o.r.height) }))
  const tiny = [...document.querySelectorAll('*')].filter((el) => {
    if (el.children.length) return false
    if (!el.textContent.trim()) return false
    return parseFloat(getComputedStyle(el).fontSize) < 12
  }).length
  return {
    vw,
    docW: de.scrollWidth,
    bodyW: document.body.scrollWidth,
    hScroll: de.scrollWidth > vw + 1,
    overflowCount: overs.length,
    overflow: overs.slice(0, 12),
    smallCount: smalls.length,
    small: smalls.slice(0, 12),
    tinyTextCount: tiny
  }
})()`

// 白名单断言：返回所有越出视口左/右边界的关键元素及其坐标
async function keyOverflow(sel) {
  return page.ev(`(() => {
    const vw = window.innerWidth
    return [...document.querySelectorAll(${JSON.stringify(sel)})].map((b) => {
      const r = b.getBoundingClientRect()
      return {
        text: (b.textContent || '').trim().slice(0, 12) || String(b.className || '').slice(0, 12),
        left: Math.round(r.left),
        right: Math.round(r.right),
        w: Math.round(r.width),
        h: Math.round(r.height)
      }
    }).filter((x) => x.left < -1 || x.right > vw + 1)
  })()`)
}

async function capture(name, file) {
  await sleep(500)
  const p = await page.ev(PROBE)
  await page.shot(file)
  console.log('\n== ' + name + ' → ' + file)
  console.log('  视口 ' + p.vw + ' | 文档宽 ' + p.docW + ' | 横向滚动 ' + (p.hScroll ? '有' : '无'))
  console.log('  越界元素 ' + p.overflowCount + ' 个；按钮小于 40px ' + p.smallCount + ' 个；字号<12px 文本 ' + p.tinyTextCount + ' 处')
  if (p.overflow.length) console.log('  越界示例：' + JSON.stringify(p.overflow.slice(0, 6)))
  if (p.small.length) console.log('  小按钮示例：' + JSON.stringify(p.small.slice(0, 8)))
  problems.push({ name, ...p })

  const sel = KEY_TARGETS[name]
  if (!sel) return
  const total = await page.ev(`document.querySelectorAll(${JSON.stringify(sel)}).length`)
  const bad = await keyOverflow(sel)
  console.log('  关键元素「' + sel + '」共 ' + total + ' 个，越界 ' + bad.length + ' 个')
  if (!bad.length) return
  failures.push({ name, sel, total, bad })
  for (const b of uniqueByText(bad)) {
    console.log('    ✗ ' + name + ' ' + sel + ' → "' + b.text + '" left=' + b.left + ' right=' + b.right +
      ' (' + b.w + '×' + b.h + ')，视口右边界 ' + p.vw)
  }
}

// 同一行内按钮在多行里重复，按文案去重后再打印，避免几十行一样的输出
function uniqueByText(list) {
  const seen = new Set()
  return list.filter((x) => {
    if (seen.has(x.text)) return false
    seen.add(x.text)
    return true
  })
}

try {
  await openApp(page)
  let ready = false
  for (let i = 0; i < 20; i++) {
    ready = await page.ev("!!document.querySelector('.statusbar')")
    if (ready) break
    await sleep(300)
  }
  assert(ready, '1440 视口登录成功')

  // 先在桌面视口落一笔现金单，让报表页有数据（否则 375 只能看到空态）
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.clickSelector('button.pay')
  await sleep(800)
  await page.click('确认收款')
  await sleep(1800)
  const orderNo = await page.ev("(async () => { const r = await fetch('/pos/v1/orders', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); const l = await r.json(); return l[0] && l[0].orderNo })()")
  assert(!!orderNo, '造数：订单已落库 ' + orderNo)

  // 切到 375×812 移动端模拟
  await page.send('Emulation.setDeviceMetricsOverride', {
    width: W, height: H, deviceScaleFactor: 2, mobile: true
  })
  await page.ev('location.reload()')
  await sleep(1500)
  await openApp(page)
  const vw = await page.ev('window.innerWidth')
  assert(vw === W, '移动端模拟生效：innerWidth=' + vw)
  for (let i = 0; i < 20; i++) {
    if (await page.ev("!!document.querySelector('.statusbar')")) break
    await sleep(300)
  }

  // 1) 收银台
  await capture('收银台', 'e2e-375-cashier.png')

  // 2) 商品页
  assert(await clickTab(page, '商品'), '切到商品页')
  await sleep(900)
  await capture('商品页', 'e2e-375-products.png')

  // 3) 盘点弹窗（商品页「盘点 ▾」→ 开始新盘点）
  const opened = await page.ev(`(() => {
    const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim().startsWith('盘点'))
    if (!b) return 'no-menu-btn'
    b.click()
    return 'menu-clicked'
  })()`)
  await sleep(400)
  const started = await page.ev(`(() => {
    const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim().includes('开始新盘点'))
    if (!b) return 'no-start-btn'
    b.click()
    return 'start-clicked'
  })()`)
  await sleep(900)
  const sheetOpen = await page.ev("!!document.querySelector('.sheet') || !!document.querySelector('.modal')")
  assert(sheetOpen, '盘点弹窗已打开（' + opened + '/' + started + '）')
  await capture('盘点弹窗', 'e2e-375-stocktake.png')

  // 关掉弹窗，避免影响后续
  await page.ev(`(() => {
    const b = [...document.querySelectorAll('button')].find((x) => ['取消', '关闭'].includes(x.textContent.trim()))
    if (b) b.click()
  })()`)
  await sleep(400)

  // 4) 报表页
  assert(await clickTab(page, '报表'), '切到报表页')
  await sleep(1200)
  await capture('报表页', 'e2e-375-reports.png')

  console.log('\n---- 汇总 ----')
  for (const p of problems) {
    console.log(`${p.name}: 横向滚动=${p.hScroll ? '有' : '无'} 越界=${p.overflowCount} 小按钮=${p.smallCount} 小字号=${p.tinyTextCount}`)
  }
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}

// 白名单断言结论：任何一个关键元素越界都判失败
if (failures.length) {
  failed = true
  console.error('\n---- 断言失败：关键元素越出 375 视口（白名单）----')
  for (const f of failures) {
    console.error(`✗ ${f.name} ${f.sel}：${f.bad.length}/${f.total} 个不在视口内`)
    for (const b of uniqueByText(f.bad)) {
      console.error(`   - "${b.text}" left=${b.left} right=${b.right} (${b.w}×${b.h})`)
    }
  }
} else if (!failed) {
  console.log('\n✓ 白名单关键元素全部落在 375 视口内')
}

process.exit(failed ? 1 : 0)
