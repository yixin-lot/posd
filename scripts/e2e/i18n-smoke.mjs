// 英文界面冒烟：登录后关键文案必须是英文（E2E_LOCALE=en-US）。
// 跑法：E2E_LOCALE=en-US node scripts/e2e/i18n-smoke.mjs
import { launchStack, openPage, openApp, assert, sleep } from './lib.mjs'

const stack = await launchStack()
const page = await openPage(9505)
try {
  await openApp(page)
  await sleep(600)
  const text = await page.ev('document.body.innerText')
  for (const want of ['Sign out', 'Cart', 'Total due', 'Payment method']) {
    assert(text.includes(want), '英文界面包含「' + want + '」')
  }
  assert(!text.includes('退出登录') && !text.includes('购物车'), '英文界面不含中文导航/购物车')
  assert(text.includes('Pearl Milk Tea') && !text.includes('珍珠奶茶'), '示例商品为英文（Pearl Milk Tea）')
  // 报表：从 ☰ Menu 抽屉进入，断言新增成本/损耗文案为英文
  await page.ev("[...document.querySelectorAll('button')].find((b) => b.textContent.includes('Menu'))?.click()")
  await sleep(400)
  await page.ev("[...document.querySelectorAll('.mi')].find((b) => b.textContent.includes('Reports'))?.click()")
  await sleep(1000)
  const rtext = await page.ev('document.body.innerText')
  for (const want of ['Cost (snapshot)', 'Loss', 'Real profit']) {
    assert(rtext.includes(want), '报表包含「' + want + '」')
  }
  assert(!text.includes('演示模式') && text.includes('Demo'), '演示条为英文')
  assert(!text.includes('选业态'), '向导步骤不含中文')
  assert(text.includes('The Tea House'), '示例门店为英文')
  const sw = JSON.parse(await page.ev('JSON.stringify({sw:document.documentElement.scrollWidth,cw:document.documentElement.clientWidth})'))
  assert(sw.sw <= sw.cw, '英文界面无横向溢出')
  console.log('\n英文界面冒烟通过')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  await stack.stop()
  process.exit(process.exitCode || 0)
}
