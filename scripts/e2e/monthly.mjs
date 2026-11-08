// 月度报表：API 数字 + CSV 导出；若「月度」页已就绪则连页面一起验。
// 跑法：node scripts/e2e/monthly.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage(9503)
try {
  await openApp(page)

  // 造两单（一单现金、一单会员余额），让月度有数
  const mk = (body) => page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/orders', { method: 'POST', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify(${JSON.stringify(body)}) })
    return r.status
  })()`)
  await mk({ cashMinor: 5000, items: [{ productId: '1', qty: 2 }] })
  await sleep(600)
  await mk({ payments: [{ method: 'wechat', amountMinor: 1300 }], items: [{ productId: '1', qty: 1 }] })
  await sleep(600)

  const month = new Date().toISOString().slice(0, 7)
  const rep = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/reports/monthly?month=${month}', { headers: { Authorization: 'Bearer ' + tk } })
    return JSON.stringify({ status: r.status, body: await r.json() })
  })()`)
  const m = JSON.parse(rep)
  assert(m.status === 200, '月度接口 200')
  assert(m.body.orderCount === 2, '月度订单数 = 2（实际 ' + m.body.orderCount + '）')
  assert(Math.abs(m.body.grossSales - 39) < 0.01, '月度销售 = 39（实际 ' + m.body.grossSales + '）')
  assert(Array.isArray(m.body.byMethod) && m.body.byMethod.length >= 2, '月度含支付方式拆分')

  // CSV：BOM + 关键行
  const csv = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/reports/monthly.csv?month=${month}', { headers: { Authorization: 'Bearer ' + tk } })
    const buf = new Uint8Array(await r.arrayBuffer())
    return JSON.stringify({ status: r.status, bom: buf[0] === 0xEF && buf[1] === 0xBB, len: buf.length })
  })()`)
  const c = JSON.parse(csv)
  assert(c.status === 200 && c.bom && c.len > 100, '月度 CSV 带 BOM 且非空（' + c.len + ' 字节）')

  await clickTab(page, '报表')
  await sleep(900)
  const hasMonthly = await page.ev("!!document.querySelector('[data-view=\"monthly\"], .monthly-switch, .month-report') || [...document.querySelectorAll('button,label')].some((b) => /月度|月结/.test(b.textContent))")
  if (hasMonthly) {
    await page.ev("[...document.querySelectorAll('button,label')].find((b) => /月度|月结/.test(b.textContent)).click()")
    await sleep(1200)
    const cards = await page.ev("document.querySelectorAll('.stat-card').length")
    assert(cards >= 4, '月报页至少 4 张指标卡（实际 ' + cards + '）')
    await page.shot('e2e-monthly.png')
    console.log('\n月度 E2E：接口 + 页面 均通过')
  } else {
    console.log('\n月度 E2E：接口与 CSV 已通过；页面尚未交付（工具 T1），本次跳过 UI 断言')
  }
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  page.close()
  await stack.stop()
}
