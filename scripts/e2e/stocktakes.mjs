// 跑法：node scripts/e2e/stocktakes.mjs
import { launchStack, openPage, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
let failed = false
try {
  await openApp(page)
  for (let i = 0; i < 20; i++) {
    if (await page.ev("!!document.querySelector('.statusbar')")) break
    await sleep(300)
  }
  await clickTab(page, '商品')
  await sleep(900)

  // 造数据：先入库建库存行 → 盘点单 #A（录差异后关单）→ 盘点单 #B（保持进行中）
  const seeded = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const products = await (await fetch('/pos/v1/products', { headers: h })).json()
    const p = products[0]
    const stock = await (await fetch('/pos/v1/stock', { headers: h })).json()
    let purchaseStatus = 0
    if (!stock.some((s) => String(s.productId) === String(p.id))) {
      const pr = await fetch('/pos/v1/purchases', { method: 'POST', headers: h, body: JSON.stringify({ supplier: 'e2e', note: 'seed', operator: '', items: [{ productId: Number(p.id), qty: 2, unitCost: 1000 }] }) })
      purchaseStatus = pr.status
    }
    const a = await (await fetch('/pos/v1/stocktakes', { method: 'POST', headers: h, body: JSON.stringify({ note: 'e2e-已关单', operator: 'admin' }) })).json()
    const items = (a.items || []).filter((x) => String(x.productId) === String(p.id))
    let putStatus = 0
    if (items.length) {
      const ur = await fetch('/pos/v1/stocktakes/' + a.id + '/items', { method: 'PUT', headers: h, body: JSON.stringify({ items: [{ productId: Number(p.id), actualQty: items[0].systemQty + 2 }] }) })
      putStatus = ur.status
    }
    await fetch('/pos/v1/stocktakes/' + a.id + '/close', { method: 'POST', headers: h, body: JSON.stringify({ operator: 'admin' }) })
    const b = await (await fetch('/pos/v1/stocktakes', { method: 'POST', headers: h, body: JSON.stringify({ note: 'e2e-进行中', operator: 'admin' }) })).json()
    return { productId: p.id, productName: p.name, stockRows: stock.length, purchaseStatus, aItems: (a.items || []).length, systemQty: items.length ? items[0].systemQty : null, putStatus, doneId: a.id, openId: b.id }
  })()`)
  console.log('  造数：' + JSON.stringify(seeded))
  assert(seeded && seeded.doneId && seeded.openId, '造出两条盘点单：done #' + (seeded && seeded.doneId) + ' / open #' + (seeded && seeded.openId))

  // 切页再切回，让库存列与列表反映刚造的数据（reload 会丢当前页状态）
  await clickTab(page, '收银')
  await sleep(300)
  await clickTab(page, '商品')
  await sleep(900)

  // 打开盘点菜单 → 查看历史
  await page.ev("[...document.querySelectorAll('.orders-head button')].find((b) => b.textContent.trim().startsWith('盘点')).click()")
  await sleep(400)
  assert(await page.ev("!!document.querySelector('.stock-menu')"), '「盘点」弹出菜单（开始新盘点 / 查看历史）')
  await page.shot('e2e-stocktake-menu.png')
  assert(
    (await page.ev("[...document.querySelectorAll('.stock-menu button')].map((b) => b.textContent.trim()).join('|')")) ===
      '开始新盘点|查看历史',
    '菜单两项文案正确'
  )
  await page.ev("[...document.querySelectorAll('.stock-menu button')].find((b) => b.textContent.trim() === '查看历史').click()")
  await sleep(900)

  const histErr = await page.ev("(document.querySelector('.orders-error') || {}).textContent || ''")
  if (histErr) console.log('  历史错误：' + histErr)
  const rows = await page.ev(`(() => {
    const list = [...document.querySelectorAll('.history-row')]
    return list.map((r) => {
      const c = r.children
      return { id: c[0].textContent.trim(), status: c[1].textContent.trim(), time: c[2].textContent.trim(), op: c[3].textContent.trim(), diff: Number(c[4].textContent.trim()) }
    })
  })()`)
  assert(Array.isArray(rows) && rows.length >= 2, '历史列表有 ' + (rows && rows.length) + ' 行')
  assert(
    rows.some((r) => r.status === '已关单') && rows.some((r) => r.status === '进行中'),
    '列表同时出现「已关单」与「进行中」：' + rows.map((r) => r.status).join(' / ')
  )
  const doneRow = rows.find((r) => r.status === '已关单')
  assert(doneRow.diff >= 1, '已关单那条差异条数 = ' + doneRow.diff)
  assert(doneRow.op === 'admin' && doneRow.time.length >= 10, '操作人与时间已展示：' + doneRow.op + ' / ' + doneRow.time)
  await page.shot('e2e-stocktake-history.png')

  // 点已关单那行 → 详情只读
  await page.ev("[...document.querySelectorAll('.history-row')].find((r) => r.children[1].textContent.includes('已关单')).click()")
  await sleep(800)
  const doneTitle = await page.ev("(document.querySelector('.modal h3') || {}).textContent || ''")
  assert(doneTitle.includes('已关单'), '已关单详情标题标注只读：' + doneTitle.trim())
  const doneDisabled = await page.ev(`(() => {
    const inputs = [...document.querySelectorAll('.modal input[type=number]')]
    const btns = [...document.querySelectorAll('.modal-actions button')].filter((b) => ['保存实盘', '关单并调账'].includes(b.textContent.trim()))
    return { inputs: inputs.every((i) => i.disabled), btns: btns.every((b) => b.disabled), n: btns.length }
  })()`)
  assert(doneDisabled.inputs && doneDisabled.btns && doneDisabled.n === 2, '已关单：实盘输入与「保存实盘/关单并调账」全部置灰')
  await page.shot('e2e-stocktake-detail-done.png')
  await page.ev("[...document.querySelectorAll('.modal-actions button')].find((b) => b.textContent.trim() === '关闭').click()")
  await sleep(400)

  // 重新打开历史 → 点进行中那行 → 可继续录
  await page.ev("[...document.querySelectorAll('.orders-head button')].find((b) => b.textContent.trim().startsWith('盘点')).click()")
  await sleep(300)
  await page.ev("[...document.querySelectorAll('.stock-menu button')].find((b) => b.textContent.trim() === '查看历史').click()")
  await sleep(900)
  await page.ev("[...document.querySelectorAll('.history-row')].find((r) => r.children[1].textContent.includes('进行中')).click()")
  await sleep(800)
  const openState = await page.ev(`(() => {
    const inputs = [...document.querySelectorAll('.modal input[type=number]')]
    const btns = [...document.querySelectorAll('.modal-actions button')].filter((b) => ['保存实盘', '关单并调账'].includes(b.textContent.trim()))
    return { inputs: inputs.every((i) => !i.disabled), btns: btns.every((b) => !b.disabled), n: btns.length }
  })()`)
  assert(openState.inputs && openState.btns && openState.n === 2, '进行中：实盘输入与按钮可编辑（可继续录）')
  await page.shot('e2e-stocktake-detail-open.png')
  await page.ev("[...document.querySelectorAll('.modal-actions button')].find((b) => b.textContent.trim() === '关闭').click()")
  await sleep(300)

  // GET /stocktakes 原始 JSON（列表接口不带明细）
  const raw = await page.ev(
    "(async () => { const tk = localStorage.getItem('pos.token'); const r = await fetch('/pos/v1/stocktakes?limit=3', { headers: { Authorization: 'Bearer ' + tk } }); return { code: r.status, body: await r.json() } })()"
  )
  console.log('  GET /pos/v1/stocktakes → ' + raw.code + ' ' + JSON.stringify(raw.body))
  assert(raw.code === 200 && Array.isArray(raw.body) && raw.body.length >= 2, 'GET /stocktakes 200 且至少两条')

  console.log('\n盘点历史验收通过')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
