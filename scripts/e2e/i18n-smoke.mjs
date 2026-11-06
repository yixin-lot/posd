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
