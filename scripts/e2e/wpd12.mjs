// 用法：node scripts/e2e/wpd12.mjs  （E2E_POSD 指向 worktree dist/posd.exe）
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'

const OUT = process.env.E2E_OUT || join(process.env.TEMP || '/tmp', 'pos_d12')
mkdirSync(OUT, { recursive: true })

const BASE = 'http://127.0.0.1:8080/pos/v1'
const ok = (r) => r.status >= 200 && r.status < 300
async function apiLogin() {
  const r = await fetch(BASE + '/auth/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'admin', pin: '1234' })
  })
  if (!r.ok) throw new Error('api login fail ' + r.status)
  return (await r.json()).token
}
async function call(path, method = 'GET', body, token) {
  const r = await fetch(BASE + path, {
    method, headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
    body: body ? JSON.stringify(body) : undefined
  })
  const text = await r.text()
  let data = null
  try { data = text ? JSON.parse(text) : null } catch {}
  return { status: r.status, data, text }
}

// ---------- 1. API 预备数据 ----------
const stack = await launchStack()
const token = await apiLogin()
console.log('token ok')

// 桌台：新建 + 开台 + 加菜
let tid
{
  const r = await call('/tables', 'POST', { name: 'A桌' }, token)
  tid = r.data && r.data.id
  if (!tid) { console.warn('create table:', r.status, r.text); tid = r.data && r.data.table && r.data.table.id }
}
assert(tid, '新建桌台 A桌')
{
  const r = await call('/tables/' + tid + '/open', 'POST', { guests: 2, items: [{ productId: '1', qty: 2 }] }, token)
  assert(ok(r), '开台（珍珠奶茶 x2）')
}
let addRes
{
  const r = await call('/tables/' + tid + '/items', 'POST', { items: [{ productId: '5', qty: 1 }] }, token)
  addRes = r.data
  assert(ok(r) && addRes && addRes.order, '加菜（杨枝甘露 x1）')
  const payableYuan = Number(addRes.order.Payable || addRes.order.payable || 0) / 100
  assert(payableYuan === 44, '加菜后应收 44 元（实际 ' + payableYuan + '）')
}

// 审计动作：改设置(round_mode) / 改价 / 作废
{
  const r = await call('/settings', 'PUT', { round_mode: 'jiao' }, token)
  assert(ok(r), '改设置抹零 jiao')
}
{
  const r = await call('/audit?limit=50', 'GET', null, token)
  const rows = r.data || []
  const auditList = Array.isArray(rows) ? rows : (rows.items || [])
  console.log('audit rows:', auditList.length, auditList.map((x) => x.action + '/' + x.operator + '/' + x.detail).join(' | '))
  const sc = auditList.find((x) => x.action === 'setting_change')
  assert(!!sc, '审计 API 含 setting_change')
  assert(sc && JSON.stringify(sc.detail).includes('round_mode') && JSON.stringify(sc.detail).includes('jiao'), '审计 detail 含 round_mode: -> jiao')
  assert(sc && (sc.operator === '店长' || sc.operator === 'admin'), '审计操作人为店长')
}
let priceNo
{
  const r = await call('/orders', 'POST', {
    idempotencyKey: 'wpd12-' + Date.now(),
    cashMinor: 1200, payments: [{ method: 'cash', amountMinor: 1200 }],
    items: [{ productId: '1', qty: 1, priceMinor: 1200 }]
  }, token)
  priceNo = r.data && r.data.orderNo
  assert(ok(r) && !!priceNo, '改价订单（珍珠奶茶 13→12）')
}
if (priceNo) {
  const r = await call('/orders/' + encodeURIComponent(priceNo) + '/void', 'POST', { reason: 'E2E 测试作废' }, token)
  assert(ok(r), '作废订单 ' + priceNo)
}

// 会员：A 无流水（停用）、B 有流水（删除→409）
let mA, mB
{
  const r = await call('/members', 'POST', { phone: '13800000001', name: '测试停用' }, token)
  mA = r.data; assert(mA && mA.id, '开卡会员 A（无流水）')
}
{
  const r = await call('/members', 'POST', { phone: '13800000002', name: '测试删除' }, token)
  mB = r.data; assert(mB && mB.id, '开卡会员 B')
  const t = await call('/members/' + mB.id + '/topup', 'POST', { amount: 100, note: 'E2E 充值' }, token)
  assert(ok(t), '会员 B 充值 100（有流水）')
}

// ---------- 2. 页面验收 ----------
const page = await openPage()
try {
  await openApp(page)
  // 首跑向导弹窗会遮挡内容：优先「下一步」推进，最后「完成/开始配置」关闭
  for (let i = 0; i < 8; i++) {
    const hit = await page.ev(`(() => {
      const order = ['下一步', '完成', '跳过并完成', '开始配置', '跳过']
      for (const t of order) {
        const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === t || x.textContent.trim().includes(t))
        if (b && !b.disabled) { b.click(); return true }
      }
      return false
    })()`)
    if (!hit) break
    await sleep(600)
  }
  await sleep(500)

  // T1 报表：日结 → 月结
  await clickTab(page, '报表')
  await sleep(900)
  await page.shot(join(OUT, 'reports-daily.png'))
  await page.click('月结')
  await sleep(1100)
  await page.shot(join(OUT, 'reports-monthly.png'))
  let txt = await page.ev('document.body.innerText')
  assert(txt.includes('净销售') && txt.includes('收款构成'), 'T1 月结页渲染指标卡+收款构成')
  assert(txt.includes('Top 20') || txt.includes('商品销量'), 'T1 月结页渲染商品 Top20')

  // T1b 切上月（2026-10）：空数据不报错
  await page.ev(`(() => {
    const el = document.querySelector('input[type="month"]')
    if (!el) return false
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    set.call(el, '2026-10')
    el.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`)
  await sleep(1100)
  txt = await page.ev('document.body.innerText')
  assert(!txt.includes('统计失败'), 'T1 切上月不报错')
  assert(txt.includes('这个月没有销售') || txt.includes('¥0.00'), 'T1 上月为空数据正确')
  await page.shot(join(OUT, 'reports-monthly-empty.png'))

  // T1c 导出 CSV：API 验证 BOM + Content-Disposition（Excel 可打开 = UTF-8 BOM）
  {
    const r = await fetch('http://127.0.0.1:8080/pos/v1/reports/monthly.csv?month=2026-11', {
      headers: { Authorization: 'Bearer ' + token }
    })
    const buf = Buffer.from(await r.arrayBuffer())
    assert(r.headers.get('content-disposition') || '' .includes('monthly-2026-11.csv'), 'T1 CSV Content-Disposition monthly-2026-11.csv')
    assert(buf.length > 3 && buf[0] === 0xEF && buf[1] === 0xBB && buf[2] === 0xBF, 'T1 CSV 带 UTF-8 BOM（Excel 可打开）')
    assert(buf.length > 100, 'T1 CSV 有内容')
  }

  // T2 审计页
  await clickTab(page, '审计')
  await sleep(1100)
  await page.shot(join(OUT, 'audit.png'))
  txt = await page.ev('document.body.innerText')
  assert(txt.includes('改设置') && txt.includes('jiao'), 'T2 审计页出现 setting_change/改设置')
  assert(txt.includes('round_mode:') || txt.includes('round_mode'), 'T2 detail 含 round_mode: -> jiao')
  assert(txt.includes('店长') || txt.includes('admin'), 'T2 操作人为店长')

  // T3 会员：停用标灰 + 删除 409
  await clickTab(page, '会员')
  await sleep(900)
  await page.ev('(() => { const tr = [...document.querySelectorAll("tbody tr")].find((x) => x.textContent.includes("13800000001")); if (!tr) return false; tr.click(); return true })()')
  await sleep(500)
  await page.click('停用会员')
  await sleep(1100)
  txt = await page.ev('document.body.innerText')
  assert(txt.includes('已停用'), 'T3 停用后列表出现「已停用」')
  await page.shot(join(OUT, 'members-off.png'))
  // 删除有流水会员 B → 409
  await page.ev('(() => { const tr = [...document.querySelectorAll("tbody tr")].find((x) => x.textContent.includes("13800000002")); if (!tr) return false; tr.click(); return true })()')
  await sleep(500)
  await page.ev('window.confirm = () => true')
  await page.click('删除')
  await sleep(1000)
  txt = await page.ev('document.body.innerText')
  assert(txt.includes('有流水的会员只能停用'), 'T3 删除有流水会员弹 409「只能停用」')
  await page.shot(join(OUT, 'members-del409.png'))

  // T4 桌台：加菜后应收提示
  await page.click('桌台')
  await sleep(900)
  await page.shot(join(OUT, 'tables-before.png'))
  await page.click('加菜')
  await sleep(500)
  await page.ev(`(() => {
    const item = [...document.querySelectorAll('.tbl-pick-item')].find((x) => x.textContent.includes('杨枝甘露'))
    if (!item) return false
    const el = item.querySelector('input')
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    set.call(el, '1')
    el.dispatchEvent(new Event('input', { bubbles: true }))
    return true
  })()`)
  await sleep(300)
  await page.click('确认加菜')
  await sleep(1200)
  txt = await page.ev('document.body.innerText')
  assert(txt.includes('已加菜') && txt.includes('应收'), 'T4 加菜后提示「已加菜，本桌应收」')
  await page.shot(join(OUT, 'tables-add.png'))
  // 结账弹窗金额=最新应收（44 + 18 = 62）
  await page.click('结账')
  await sleep(900)
  txt = await page.ev('document.body.innerText')
  const m = txt.match(/¥(\d+\.\d{2})/)
  assert(m && m[1] === '62.00', 'T4 结账应收为 62.00（实际 ' + (m && m[1]) + '）')
  await page.shot(join(OUT, 'tables-checkout.png'))

  console.log('\n全部通过，截图目录：' + OUT)
} finally {
  page.close()
  await stack.stop()
}
