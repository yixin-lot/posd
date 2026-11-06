// 跑法：node scripts/e2e/purchase.mjs
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
  assert(
    await page.ev("[...document.querySelectorAll('.orders-head button')].some((b) => b.textContent.trim() === '入库')"),
    '商品页工具栏出现「入库」按钮'
  )

  await page.ev(
    "[...document.querySelectorAll('.orders-head button')].find((b) => b.textContent.trim() === '入库').click()"
  )
  await sleep(600)
  assert(await page.ev("!!document.querySelector('.purchase-head')"), '入库弹窗打开（供应商 / 备注 / 明细行）')

  // 明细行：选商品、填数量 3、进价 12.5 元
  const pickedName = await page.ev(`(() => {
    const s = document.querySelector('.modal select')
    if (!s || s.options.length < 2) return ''
    s.value = s.options[1].value
    s.dispatchEvent(new Event('change', { bubbles: true }))
    return s.options[1].textContent.trim()
  })()`)
  assert(!!pickedName, '明细行已选中商品：' + pickedName)
  await sleep(200)

  // 入库前库存：按刚选中的商品名去表格里找（新库该商品还没有库存行，显示「未跟踪」→ null）
  const before = await page.ev(`(() => {
    const rows = [...document.querySelectorAll('.orders-table tbody tr')]
    const r = rows.find((x) => x.children[0] && x.children[0].textContent.includes(${JSON.stringify(pickedName)}))
    if (!r) return null
    const n = Number(r.children[4].textContent.trim())
    return { text: r.children[4].textContent.trim(), qty: Number.isFinite(n) ? n : null }
  })()`)
  assert(before, '在商品表里找到该商品行')
  await page.setValue('.modal input[type=number]', '3', 0)
  await page.setValue('.modal input[type=number]', '12.5', 1)
  await sleep(300)

  const total = await page.ev("(document.querySelector('.purchase-total') || {}).textContent || ''")
  assert(total.trim() === '¥37.50', '合计按元计算（3 × 12.5 = 37.50）：' + total.trim())
  await page.shot('e2e-purchase-form.png')

  const saveDisabled = await page.ev(
    "!![...document.querySelectorAll('.modal .primary')].find((b) => b.textContent.includes('保存入库') && b.disabled)"
  )
  assert(!saveDisabled, '校验通过时「保存入库」可点')

  await page.ev("[...document.querySelectorAll('.modal .primary')].find((b) => b.textContent.includes('保存入库')).click()")
  await sleep(1500)

  const toastText = await page.ev("(document.querySelector('.orders-toast') || {}).textContent || ''")
  assert(toastText.includes('入库成功'), '保存后 toast 提示：' + toastText.trim())

  // 入库后同一商品库存 +3（入库前「未跟踪」按 0 计）
  const after = await page.ev(`(() => {
    const rows = [...document.querySelectorAll('.orders-table tbody tr')]
    const r = rows.find((x) => x.children[0] && x.children[0].textContent.includes(${JSON.stringify(pickedName)}))
    if (!r) return null
    return { qty: Number(r.children[4].textContent.trim()) }
  })()`)
  const base = before.qty === null ? 0 : before.qty
  assert(after && after.qty === base + 3, `库存 ${before.text} → ${after.qty}（+3）`)
  await page.shot('e2e-purchase-after.png')

  // 进货单已落库
  const list = await page.ev(
    "(async () => { const tk = localStorage.getItem('pos.token'); const r = await fetch('/pos/v1/purchases?limit=5', { headers: { Authorization: 'Bearer ' + tk } }); return await r.json() })()"
  )
  assert(Array.isArray(list) && list.length > 0, 'GET /purchases 有记录：' + JSON.stringify(list && list[0] && { id: list[0].id, totalCost: list[0].totalCost, operator: list[0].operator }))

  // 校验分支：清空商品后按钮禁用
  await page.ev(
    "[...document.querySelectorAll('.orders-head button')].find((b) => b.textContent.trim() === '入库').click()"
  )
  await sleep(500)
  const hint = await page.ev("[...document.querySelectorAll('.modal p')].map((p) => p.textContent.trim()).join(' | ')")
  const disabled = await page.ev(
    "!![...document.querySelectorAll('.modal .primary')].find((b) => b.textContent.includes('保存入库') && b.disabled)"
  )
  assert(disabled, '空明细行时「保存入库」禁用')
  if (hint.trim()) console.log('  弹窗提示：' + hint.trim())
  await page.ev("[...document.querySelectorAll('.modal button')].find((b) => b.textContent.trim() === '取消').click()")
  await sleep(300)

  await page.shot('e2e-purchase.png')
  console.log('\n采购入库验收通过，截图 e2e-purchase.png')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
