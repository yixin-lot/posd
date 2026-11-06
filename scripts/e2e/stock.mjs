// 库存界面验收：库存列 / 未跟踪 / 设置后显示 / 低库存提醒
// 跑法：node scripts/e2e/stock.mjs
// 说明：通过接口造数据、页面断言渲染结果（弹窗交互由人工验证，见 README 已知问题）。
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage(9506)
try {
  await openApp(page)
  // 登录后商品可能还没加载完：等到有商品（或补一次重载）
  for (let i = 0; i < 20; i++) {
    if (await page.ev("document.querySelectorAll('.orders-table tbody tr, .grid .card').length > 0")) break
    await sleep(300)
  }

  const seeded = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const H = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const list = await (await fetch('/pos/v1/products', { headers: H })).json()
    const p = list && list.length ? list[0] : await (await fetch('/pos/v1/products', { method: 'POST', headers: H, body: JSON.stringify({ name: '库存测试品', category: '测试', price: 10 }) })).json()
    return JSON.stringify({ id: p.id, name: p.name })
  })()`)
  const prod = JSON.parse(seeded)
  console.log('  seeded:', seeded)

  await clickTab(page, '商品')
  for (let i = 0; i < 20; i++) {
    if (await page.ev("document.querySelectorAll('.orders-table tbody tr').length > 0")) break
    await sleep(300)
  }
  const headers = await page.ev("[...document.querySelectorAll('.orders-table thead th')].map((t) => t.textContent.trim()).join('|')")
  assert(headers.includes('库存'), '商品页有「库存」列')

  // 商品列表偶发晚到：重进一次页面再等
  let row0 = ''
  for (let attempt = 0; attempt < 3; attempt++) {
    row0 = await page.ev("[...document.querySelectorAll('.orders-table tbody tr')].slice(0,1).map((r) => r.innerText.replace(/\\s+/g,' ')).join('')")
    if (row0 && !row0.includes('还没有商品')) break
    await clickTab(page, '收银')
    await sleep(300)
    await clickTab(page, '商品')
    for (let i = 0; i < 10; i++) {
      const n = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
      if (n > 0 && !(await page.ev("document.body.innerText.includes('还没有商品')"))) break
      await sleep(300)
    }
  }
  assert(row0.includes('未跟踪'), '未建库存时显示「未跟踪」（实际：' + row0 + '）')

  // 通过接口建库存：2 件、阈值 3 → 应触发低库存提醒
  const setStatus = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/stock/${prod.id}', { method: 'PUT', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify({ qty: 2, warnQty: 3 }) })
    return JSON.stringify({ status: r.status, body: (await r.text()).slice(0, 120), tokenLen: (tk || '').length, url: '/pos/v1/stock/' + ${prod.id} })
  })()`)
  console.log('  setStatus: ' + setStatus)
  const setRes = JSON.parse(setStatus)
  assert(setRes.status === 200, '设置库存接口 200（实际 ' + setRes.status + ' ' + setRes.body + ' tokenLen=' + setRes.tokenLen + '）')

  // 重新进商品页触发加载（先刷新，确保拿到最新库存）
  await openApp(page)
  await clickTab(page, '商品')
  for (let i = 0; i < 20; i++) {
    if (await page.ev("document.body.innerText.includes('库存偏低')")) break
    await sleep(300)
  }
  const row1 = await page.ev("[...document.querySelectorAll('.orders-table tbody tr')].slice(0,1).map((r) => r.innerText.replace(/\\s+/g,' ')).join('')")
  assert(/\b2\b/.test(row1), '库存列显示 2（实际：' + row1 + '）')
  assert(await page.ev("!!document.querySelector('.stock-low')"), '低库存标红')
  const warn = await page.ev("(document.querySelector('.orders-warn')||{}).textContent||''")
  assert(warn.includes('库存偏低') && warn.includes(prod.name), '补货提醒含商品名（实际：' + warn.slice(0, 60) + '）')

  // 报损 1 → 1，仍在提醒里
  const adjStatus = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/stock/${prod.id}/adjust', { method: 'POST', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify({ delta: -1, reason: 'E2E 报损' }) })
    return r.status
  })()`)
  assert(adjStatus === 200, '报损接口 200（实际 ' + adjStatus + '）')
  await openApp(page)
  await clickTab(page, '商品')
  for (let i = 0; i < 20; i++) {
    if (await page.ev("document.body.innerText.includes('库存偏低')")) break
    await sleep(300)
  }
  const row2 = await page.ev("[...document.querySelectorAll('.orders-table tbody tr')].slice(0,1).map((r) => r.innerText.replace(/\\s+/g,' ')).join('')")
  assert(/\b1\b/.test(row2), '报损后显示 1（实际：' + row2 + '）')

  await page.shot('e2e-stock.png')
  console.log('\n库存界面验收通过：库存列 / 未跟踪 / 数值渲染 / 低库存提醒 / 报损')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  await stack.stop()
  // CDP 的 WebSocket 仍可能挂住事件循环：显式退出，避免 driver 判超时
  process.exit(process.exitCode || 0)
}
