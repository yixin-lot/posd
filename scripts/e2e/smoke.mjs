// 冒烟：登录 → 现金单 → 挂单 → 取单收款 → 退款 → 作废 → 看报表数字。
// 跑法：node scripts/e2e/smoke.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
let failed = false
try {
  await openApp(page)
  // 登录后可能有一次性向导/过渡，等主界面出现（最多 6s）再断言
  let ready = false
  for (let i = 0; i < 20; i++) {
    ready = await page.ev("!!document.querySelector('.statusbar')")
    if (ready) break
    await sleep(300)
  }
  assert(ready, '登录成功，进入收银台')

  // 现金单：点第一个商品，结算
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.clickSelector('button.pay')
  await sleep(700)
  assert(await page.ev("!!document.querySelector('.pay-methods')"), '支付弹窗打开')
  // v03 起支付方式含组合，数量不再是固定 3；断言"至少 3 种且含现金"
  const payLabels = await page.ev("[...document.querySelectorAll('.pay-methods button')].map((b) => b.textContent.trim()).join('|')")
  assert(payLabels.split('|').length >= 3 && payLabels.includes('现金'), '支付方式齐全：' + payLabels)

  // 现金：默认金额即应收，直接确认
  await page.click('确认收款')
  await sleep(1800)
  const orderNo = await page.ev("(async () => { const r = await fetch('/pos/v1/orders', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); const l = await r.json(); return l[0] && l[0].orderNo })()")
  assert(!!orderNo, '订单已落库：' + orderNo)
  const toast = await page.ev("(document.querySelector('.toast')||{}).textContent||''")
  assert(toast.length > 0, '出票提示出现：' + toast.trim())

  // 挂单 → 挂单条 → 收款
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.click('挂单')
  await sleep(1500)
  assert((await page.ev("document.querySelectorAll('.held-item').length")) === 1, '挂单条出现')
  await page.clickSelector('.held-item')
  await sleep(700)
  await page.click('微信')
  await page.click('确认收款')
  await sleep(1800)
  assert((await page.ev("document.querySelectorAll('.held-item').length")) === 0, '取单后挂单条消失')

  // 退款：订单页第一单
  await clickTab(page, '订单')
  await sleep(900)
  assert((await page.ev("document.querySelectorAll('.orders-table tbody tr').length")) > 0, '订单页有数据')
  const refunded = await page.ev("(async () => { const tk = localStorage.getItem('pos.token'); const r = await fetch('/pos/v1/orders', { headers: { Authorization: 'Bearer ' + tk } }); const l = await r.json(); const o = l.find((x) => x.status === 'paid' && !x.refundAmount); if (!o) return ''; const res = await fetch('/pos/v1/orders/' + o.orderNo + '/refund', { method: 'POST', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify({ reason: 'e2e' }) }); return res.status + ':' + o.orderNo })()")
  assert(refunded.startsWith('200:'), '退款接口 200（' + refunded + '）')

  // 报表数字
  await clickTab(page, '报表')
  await sleep(1200)
  const cards = await page.ev("[...document.querySelectorAll('.stat-card')].map((c) => c.innerText.replace(/\\n/g, ' ')).join(' | ')")
  assert(cards.includes('退款') && cards.includes('作废'), '报表出现退款/作废卡片')
  console.log('  报表：' + cards)

  await page.shot('e2e-smoke.png')
  console.log('\n冒烟通过，截图 e2e-smoke.png')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
