// 会员：开卡 → 充值 → 用会员余额结算 → 余额扣减。
// 跑法：node scripts/e2e/member.mjs
import { launchStack, openPage, login, assert, sleep, openApp } from './lib.mjs'


// 结算区按钮（避免命中顶部「会员」页签）
// v03 起「会员」入口并入结算区的优惠行（.pay-note），旧的 .actions 区块已移除
const clickActions = (text) => page.ev(`(() => {
  const note = document.querySelector('.pay-note')
  if (note) { note.click(); return true }
  const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim().includes(${JSON.stringify(text)}))
  if (!b || b.disabled) return false
  b.click()
  return true
})()`)

const stack = await launchStack()
const page = await openPage(9502)
const phone = '138' + String(Date.now()).slice(-8)

try {
  await openApp(page)

  // 打开会员弹窗
  await clickActions('会员')
  await sleep(700)
  assert(await page.ev("!!document.querySelector('.member-modal')"), '会员弹窗打开')

  // 查一个不存在的号 → 开卡
  await page.setValue('.member-search input', phone, 0)
  await page.click('查询')
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.member-empty')"), '未开卡提示出现')
  await page.setValue('.member-search input', 'E2E会员', 1)
  await page.click('开卡')
  await sleep(1200)
  const balance0 = await page.ev("(document.querySelector('.member-balance')||{}).textContent||''")
  assert(balance0.includes('0.00'), '开卡成功，余额 0（' + balance0.trim() + '）')

  // 充值 100
  await page.setValue('.member-topup input', '100', 0)
  await page.click('充值')
  await sleep(1500)
  const balance1 = await page.ev("(document.querySelector('.member-balance')||{}).textContent||''")
  assert(balance1.includes('100.00'), '充值成功，余额 100（' + balance1.trim() + '）')

  // 选中会员并关闭
  await page.click('用这位会员结算')
  await sleep(800)
  assert(!(await page.ev("!!document.querySelector('.member-modal')")), '弹窗关闭')

  // 加商品 → 结算 → 选会员余额
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.clickSelector('button.pay')
  await sleep(700)
  const methods = await page.ev("[...document.querySelectorAll('.pay-methods button')].map((b) => b.textContent.trim())")
  assert(methods.includes('会员余额'), '支付方式出现「会员余额」：' + methods.join('/'))
  await page.ev("[...document.querySelectorAll('.pay-methods button')].find((b) => b.textContent.trim() === '会员余额').click()")
  await sleep(300)
  await page.click('确认收款')
  await sleep(2000)

  // 余额应扣掉
  await clickActions('E2E会员')
  await sleep(700)
  await page.setValue('.member-search input', phone, 0)
  await page.click('查询')
  await sleep(1000)
  const balance2 = await page.ev("(document.querySelector('.member-balance')||{}).textContent||''")
  assert(!balance2.includes('100.00'), '余额已扣减（' + balance2.trim() + '）')
  await page.shot('e2e-member.png')
  console.log('\n会员流程通过')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  page.close()
  await stack.stop()
}
