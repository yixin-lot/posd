// 调试：375px 模拟下查 .grid 列数与页面状态（定位 T4「商品 2 列」回归）
// 跑法：node scripts/e2e/grid-check.mjs（先起 8080 posd embed）
import { launchStack, openPage, login, sleep } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
try {
  await page.ev("location.href = 'http://127.0.0.1:8080/'")
  await sleep(2000)
  await login(page)
  await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const s = await (await fetch('/pos/v1/settings', { headers: h })).json()
    await fetch('/pos/v1/settings', { method: 'PUT', headers: h, body: JSON.stringify({ ...s, setup_done: 'true' }) })
    return true
  })()`)
  await page.ev("location.reload()")
  await sleep(2000)

  await page.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 667, deviceScaleFactor: 2, mobile: true })
  await page.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 1 })
  await sleep(1200)

  // 复现 d14.mjs T4：切 5173 vite dev
  await page.ev("location.href = 'http://127.0.0.1:5173/'")
  await sleep(2500)

  const info = await page.ev(`(() => {
    const grid = document.querySelector('.grid')
    const cs = grid ? getComputedStyle(grid) : null
    return {
      appEmpty: document.getElementById('app').childElementCount === 0,
      statusbar: !!document.querySelector('.statusbar'),
      gridExists: !!grid,
      cols: cs ? cs.gridTemplateColumns : null,
      gridDisplay: cs ? cs.display : null,
      viewport: document.documentElement.clientWidth + 'x' + document.documentElement.clientHeight,
      bodyText: document.body.innerText.slice(0, 80)
    }
  })()`)
  console.log(JSON.stringify(info, null, 2))
  await page.shot('grid-debug.png')
} catch (e) {
  console.error('✗ ' + e.message)
} finally {
  page.close()
  await stack.stop()
}
