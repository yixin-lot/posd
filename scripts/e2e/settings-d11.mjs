// 跑法：node scripts/e2e/settings-d11.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
let failed = false
const OUT = process.env.E2E_OUT || '.'
try {
  await openApp(page)
  assert(await page.ev("!!document.querySelector('.statusbar')"), '登录成功，进入收银台')
  // 关掉首跑向导（若弹出：循环点「完成」→「跳过并完成」→「跳过」直到消失）
  await page.ev(`(async () => {
    for (let i = 0; i < 8; i++) {
      const bs = [...document.querySelectorAll('button')]
      const done = bs.find((b) => b.textContent.trim() === '完成')
      const finish = bs.find((b) => b.textContent.trim() === '跳过并完成')
      const skip = bs.find((b) => b.textContent.trim() === '跳过')
      const t = done || finish || skip
      if (!t) return true
      t.click()
      await new Promise((r) => setTimeout(r, 500))
    }
    return false
  })()`)
  await sleep(600)

  // 进设置页
  await clickTab(page, '设置')
  await sleep(1000)
  assert(await page.ev("[...document.querySelectorAll('h3')].some((h) => h.textContent.trim() === '后厨档口')"), '设置页出现「后厨档口」区块')
  assert(await page.ev("document.body.innerText.includes('留空表示不出后厨单')"), '后厨说明行存在')

  // 填后厨打印机 + 类目并保存（后厨区块 = 第 2 个 .settings-form）
  await page.ev(`(() => {
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    const printer = document.querySelector('input[placeholder*="厨房打印机"]')
    const cats = document.querySelector('input[placeholder*="奶茶,小吃"]')
    if (!printer || !cats) return false
    set.call(printer, '厨房打印机'); printer.dispatchEvent(new Event('input', { bubbles: true }))
    set.call(cats, '奶茶,小吃'); cats.dispatchEvent(new Event('input', { bubbles: true }))
    return true
  })()`)
  await sleep(300)
  await page.ev("document.querySelector('.settings-actions .primary').click()")
  await sleep(1500)
  const saved = await page.ev("(async () => { const r = await fetch('/pos/v1/settings', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); return JSON.stringify(await r.json()) })()")
  assert(saved.includes('"kitchen_printer":"厨房打印机"') && saved.includes('"kitchen_categories":"奶茶,小吃"'), 'GET /settings 返回后厨两键')
  console.log('  设置接口：' + saved)
  await page.shot(OUT + '/settings-kitchen.png')
  console.log('  设置页截图 settings-kitchen.png')

  // 订单页退款弹窗文案（现金单 → 退款 → 弹窗文案）
  await clickTab(page, '收银')
  await sleep(800)
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.clickSelector('button.pay')
  await sleep(700)
  await page.click('确认收款')
  await sleep(1800)
  await clickTab(page, '订单')
  await sleep(1500)
  for (let i = 0; i < 12; i++) {
    if (await page.ev("document.querySelectorAll('.orders-table tbody tr').length > 0")) break
    await sleep(500)
  }
  const rows = await page.ev("[...document.querySelectorAll('.orders-table tbody tr')].length")
  console.log('  订单行数：' + rows)
  const btns = await page.ev("[...document.querySelectorAll('.orders-table button')].map((b) => b.textContent.trim() + (b.disabled ? '(禁用)' : '')).join('|')")
  console.log('  订单按钮：' + btns)
  const refundBtns = await page.ev("[...document.querySelectorAll('button')].filter((b) => b.textContent.trim().includes('退款')).map((b) => b.outerHTML.slice(0, 120)).join(' || ')")
  console.log('  含退款按钮：' + refundBtns)
  const clicked = await page.ev("(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '退款'); if (!b || b.disabled) return false; b.click(); return true })()")
  console.log('  退款点击（精确）：' + clicked)
  await sleep(1200)
  const maskCount = await page.ev("document.querySelectorAll('.mask').length")
  const modalCount = await page.ev("document.querySelectorAll('.modal').length")
  const bodyHasRefund = await page.ev("document.body.innerText.includes('退款 ·')")
  const bodyHasText = await page.ev("document.body.innerText.includes('会员余额支付的金额会退回会员余额')")
  console.log('  mask=' + maskCount + ' modal=' + modalCount + ' 含退款标题=' + bodyHasRefund + ' 含会员文案=' + bodyHasText)
  const modals = await page.ev("[...document.querySelectorAll('.modal')].map((m) => m.innerText.replace(/\\n+/g, ' ').slice(0, 60)).join(' || ')")
  console.log('  弹窗：' + modals)
  const refundText = await page.ev("([...document.querySelectorAll('.modal')].find((m) => m.innerText.includes('退款'))||{innerText:''}).innerText")
  assert(refundText.includes('会员余额支付的金额会退回会员余额'), '退款弹窗含会员余额退回说明')
  console.log('  退款弹窗文案：' + refundText.replace(/\n+/g, ' ').slice(0, 160))

  console.log('\n设置后厨 + 退款文案验收通过')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
