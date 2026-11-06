// UI 优化验收：复现用户两张截图场景（1855×900 正常态 / 1860×447 矮视口）
// 验证：顶栏页签收敛单排、更多菜单不重复、空车态、禁用结算、矮视口不碎
// 跑法：node scripts/e2e/ui-shot.mjs（launchStack 自起 8080 posd embed）
import { launchStack, openPage, login, sleep, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
try {
  await page.ev("location.href = 'http://127.0.0.1:8080/'")
  await sleep(2000)
  await openApp(page)
  await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const s = await (await fetch('/pos/v1/settings', { headers: h })).json()
    await fetch('/pos/v1/settings', { method: 'PUT', headers: h, body: JSON.stringify({ ...s, setup_done: 'true' }) })
    return true
  })()`)
  await page.ev("location.reload()")
  await sleep(2000)

  // 场景一：正常态 1855×900
  await page.send('Emulation.setDeviceMetricsOverride', { width: 1855, height: 900, deviceScaleFactor: 1, mobile: false })
  await sleep(1500)
  const n1 = await page.ev(`(() => ({
    tabs: [...document.querySelectorAll('.tabs button')].map((b) => b.textContent.trim()),
    more: [...document.querySelectorAll('.more-menu button')].map((b) => b.textContent.trim()),
    moreVisible: getComputedStyle(document.querySelector('.more-menu')).display !== 'none',
    empty: document.querySelector('.empty') ? document.querySelector('.empty').innerText.replace(/\\n/g, ' / ') : 'NO-EMPTY',
    payDisabled: !!document.querySelector('.pay').disabled,
    statusH: getComputedStyle(document.querySelector('.statusbar')).height,
    viewport: innerWidth + 'x' + innerHeight
  }))()`)
  console.log('正常态:', JSON.stringify(n1))
  await page.shot('ui-normal.png')

  // 打开「更多」菜单：确认不重复导航
  await page.ev("document.querySelector('.more-nav .more-btn').click()")
  await sleep(500)
  const n2 = await page.ev(`(() => ({
    moreVisible: getComputedStyle(document.querySelector('.more-menu')).display !== 'none',
    moreItems: [...document.querySelectorAll('.more-menu button')].map((b) => b.textContent.trim()),
    tabsCount: document.querySelectorAll('.tabs button').length
  }))()`)
  console.log('更多菜单:', JSON.stringify(n2))
  await page.shot('ui-more-open.png')
  await page.ev("document.querySelector('.more-nav .more-btn').click()") // 收起

  // 场景二：矮视口 1860×447（崩溃态复现）
  await page.send('Emulation.setDeviceMetricsOverride', { width: 1860, height: 447, deviceScaleFactor: 1, mobile: false })
  await sleep(1500)
  const s2 = await page.ev(`(() => ({
    viewport: innerWidth + 'x' + innerHeight,
    statusH: getComputedStyle(document.querySelector('.statusbar')).height,
    tabsRows: (() => { const t = document.querySelector('.tabs'); return t ? getComputedStyle(t).flexWrap + ' / ' + t.children.length + ' tabs' : 'NO-TABS' })(),
    cols: (() => { const c = document.querySelector('.cols'); return c ? getComputedStyle(c).gridTemplateColumns : 'NO-COLS' })(),
    noScrollX: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
    noScrollY: document.documentElement.scrollHeight <= document.documentElement.clientHeight,
    hintHidden: (() => { const h = document.querySelector('.hint'); return h ? getComputedStyle(h).display === 'none' : 'NO-HINT' })(),
    bodyOverflow: getComputedStyle(document.documentElement).overflow
  }))()`)
  console.log('矮视口:', JSON.stringify(s2))
  await page.shot('ui-short.png')

  // 场景三：有商品后结算可点 + 更多菜单正常
  await page.send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false })
  await sleep(800)
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(500)
  const s3 = await page.ev(`(() => ({
    payDisabled: !!document.querySelector('.pay').disabled,
    cartRows: document.querySelectorAll('.cart-row').length,
    actions: [...document.querySelectorAll('.actions button')].map((b) => b.textContent.trim())
  }))()`)
  console.log('加购后:', JSON.stringify(s3))
  await page.shot('ui-with-items.png')
} catch (e) {
  console.error('✗ ' + e.message)
} finally {
  page.close()
  await stack.stop()
}
