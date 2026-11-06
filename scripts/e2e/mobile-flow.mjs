// 手机全流程：375×812 下 登录 → 加购 → 现金结算 → 订单页，全程不允许横向溢出。
// 跑法：node scripts/e2e/mobile-flow.mjs
import { launchStack, openPage, openApp, clickTab, sleep, assert } from './lib.mjs'

const W = 375, H = 812
const stack = await launchStack()
const page = await openPage(9503)

const overflow = async (label) => {
  const v = JSON.parse(await page.ev('JSON.stringify({sw:document.documentElement.scrollWidth,cw:document.documentElement.clientWidth})'))
  assert(v.sw <= v.cw, label + ' 无横向溢出（' + v.sw + ' <= ' + v.cw + '）')
  return v
}

try {
  await page.send('Emulation.setDeviceMetricsOverride', { width: W, height: H, deviceScaleFactor: 2, mobile: true })
  await page.ev('location.reload()')
  await sleep(1200)

  // 登录页：卡片必须完整在 375 内（这条曾经真的坏过）
  await overflow('登录页')
  await openApp(page)
  await overflow('收银台')

  // 加一个商品
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(500)
  const total = await page.ev("document.querySelector('.total') ? document.querySelector('.total').textContent.trim() : ''")
  assert(/¥\s*\d/.test(total), '购物车有应收：' + total)

  // 现金结算
  await page.clickSelector('button.pay')
  await sleep(700)
  await overflow('支付弹窗')
  await page.click('确认收款')
  await sleep(1800)

  const ok = await page.ev("document.body.innerText.includes('已收款') || document.body.innerText.includes('收款成功') || !document.querySelector('.pay-methods')")
  assert(ok, '结算完成（支付弹窗已关闭）')
  await overflow('结算后收银台')

  // 订单页
  await clickTab(page, '订单')
  await sleep(900)
  await overflow('订单页')
  const rows = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  assert(rows > 0, '订单页有数据（' + rows + ' 行）')

  await page.shot('mobile-flow-375.png')
  console.log('\n手机全流程通过：登录 → 加购 → 现金结算 → 订单页，全程无横向溢出')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  await stack.stop()
  process.exit(process.exitCode || 0)
}
