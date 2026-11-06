// 跑法：node scripts/e2e/d14.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
let failed = false
try {
  // 前端走 embed（8080，与发布一致）；vite dev 在本机偶发白屏（#app 空）不影响本用例
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
  assert(await page.ev("!!document.querySelector('.statusbar')"), '登录成功，进入收银台')

  // ---- 造数：1 paid + 1 hold + 桌台 A1（开台自建挂单绑定） ----
  const seed = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const r1 = await fetch('/pos/v1/orders', { method: 'POST', headers: h, body: JSON.stringify({ cash: 20, items: [{ productId: '1', qty: 1 }] }) })
    const r2 = await fetch('/pos/v1/orders', { method: 'POST', headers: h, body: JSON.stringify({ hold: true, items: [{ productId: '2', qty: 1 }] }) })
    const t = await fetch('/pos/v1/tables', { method: 'POST', headers: h, body: JSON.stringify({ name: 'A1' }) })
    let o = ''
    if (t.ok) {
      const tid = (await (await fetch('/pos/v1/tables', { headers: h })).json())[0]?.id
      const op = await fetch('/pos/v1/tables/' + tid + '/open', { method: 'POST', headers: h, body: JSON.stringify({ items: [{ productId: '1', qty: 1 }], guests: 2 }) })
      o = op.ok ? 'ok' : 'open-' + op.status
    }
    return (r1.ok ? 'ok' : r1.status) + '/' + (r2.ok ? 'ok' : r2.status) + '/' + (t.ok ? 'ok' : t.status) + '/' + (o || 'no-t')
  })()`)
  assert(seed === 'ok/ok/ok/ok', '造数成功（paid/hold/桌台/开台）：' + seed)

  // ---- T1/T2 订单页 ----
  await clickTab(page, '订单')
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.filters-row')"), 'T1 筛选行渲染（日期+桌台下拉）')
  const row0 = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  assert(row0 === 3, '全部订单 3 笔（paid+hold+桌台挂单），实际 ' + row0)

  // 挂单时长列（T2）
  assert(await page.ev("!!document.querySelector('.held-duration')"), 'T2 挂单时长列渲染')
  const durText = String((await page.ev("(document.querySelector('.held-duration')||{}).textContent||''")) || '')
  assert(durText.includes('已挂'), 'T2 挂单时长格式「已挂 X 分钟」：' + durText)
  assert((await page.ev("!!document.querySelector('.orders-warn')")) === false, '无 >30min 挂单时不显示超时提示（条件正确）')

  // 日期筛选 → URL query 持久化
  await page.ev("document.querySelectorAll('.f-cell select')[0].value = 'today'")
  await page.ev("document.querySelectorAll('.f-cell select')[0].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  const todayRows = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  const qToday = await page.ev("location.search")
  assert(todayRows >= 1 && qToday.includes('date=today'), 'T1 切「今天」数量变化且 URL=?date=today（' + todayRows + ' 笔 ' + qToday + '）')
  await page.ev("document.querySelectorAll('.f-cell select')[0].value = ''")
  await page.ev("document.querySelectorAll('.f-cell select')[0].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  assert((await page.ev("location.search")).includes('date=') === false, 'T1 切回「全部」URL query 清空')

  // 桌台筛选 → URL query
  await page.ev("document.querySelectorAll('.f-cell select')[1].value = '1'")
  await page.ev("document.querySelectorAll('.f-cell select')[1].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  const tableRows = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  const qTable = await page.ev("location.search")
  assert(tableRows === 1 && qTable.includes('table=1'), 'T1 桌台筛选 A1 → 1 笔且 URL=?table=1（' + tableRows + ' 笔 ' + qTable + '）')

  // ---- T3 设置页打印区块（页签收敛后走「更多」菜单进入） ----
  await page.ev(`(() => {
    const more = document.querySelector('.more-nav summary')
    if (more) more.click()
    return !!more
  })()`)
  await sleep(400)
  await page.ev("[...document.querySelectorAll('.more-menu button')].find((b) => b.textContent.trim() === '设置').click()")
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.print-section-title')"), 'T3 打印区块标题渲染')
  const secText = await page.ev("document.querySelector('.print-section-title') ? document.querySelector('.print-section-title').nextElementSibling ? document.querySelector('.print-section-title').parentElement.innerText : '' : ''")
  assert(secText.includes('小票打印机') && secText.includes('后厨打印') && secText.includes('打印队列'), 'T3 三卡片齐全（小票/后厨/队列）')
  // 打印队列：已接真实接口（2026-11-05）。要么有任务行，要么空态「还没有打印任务」；
  // 但绝不能再出现 TODO 占位文案。
  const queueText = await page.ev(`(() => {
    const t = [...document.querySelectorAll('.shift-card h3')].find((h) => h.textContent.includes('打印队列'))
    return t ? t.parentElement.innerText : ''
  })()`)
  assert(queueText.includes('单号') && queueText.includes('状态'), 'T3 打印队列表格已渲染')
  assert(!queueText.includes('TODO') && !queueText.includes('暂未提供'), 'T3 打印队列不再是占位')
  // 打印测试交互（tcp:// 断连地址 → 错误消息）
  await page.ev(`(() => {
    const el = document.querySelectorAll('.print-section-title ~ .shift-card input')[0]
    if (!el) return false
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    set.call(el, 'tcp://127.0.0.1:1')
    el.dispatchEvent(new Event('input', { bubbles: true }))
    return true
  })()`)
  await page.click('打印测试')
  await sleep(1500)
  const testMsg = String((await page.ev("(document.querySelector('.print-section-title ~ .shift-card button.primary') || {}).nextElementSibling ? document.querySelector('.print-section-title ~ .shift-card button.primary').nextElementSibling.textContent : ''")) || '')
  assert(testMsg.includes('✗') || testMsg.includes('连不上'), 'T3 打印测试交互返回错误消息：' + testMsg.slice(0, 60))

  // ---- T4 375px 移动端（三截图 + 无横向滚动；嵌定 8080 embed：
  //     5173 跨 origin 无登录态且偶发白屏，断言无意义） ----
  await page.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 667, deviceScaleFactor: 2, mobile: true })
  await page.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 1 })
  await page.ev("location.href = 'http://127.0.0.1:8080/'")
  await sleep(2000)
  // 换 origin（5173 → 8080）会丢登录态，必须重新登录
  await login(page)
  await clickTab(page, '收银')
  await sleep(900)
  const w375 = await page.ev("document.documentElement.clientWidth")
  assert(w375 === 375, '视口宽 375px（实际 ' + w375 + '）')
  const cashNoH = await page.ev("document.documentElement.scrollWidth <= document.documentElement.clientWidth")
  assert(cashNoH, 'T4 收银台 375px 无横向滚动条')
  const gcols = await page.ev("document.querySelector('.grid') ? getComputedStyle(document.querySelector('.grid')).gridTemplateColumns : 'NO-GRID:' + (document.body.innerText||'').replace(/\\s+/g,' ').slice(0,160)")
  assert(String(gcols).split(' ').length === 2, 'T4 商品 2 列（实际 ' + gcols + '）')
  assert(await page.ev("!!document.querySelector('.cart-toggle') && getComputedStyle(document.querySelector('.cart-toggle')).display !== 'none'"), 'T4 移动端购物车按钮可见')
  await page.shot('d14-cashier-375.png')
  console.log('  截图 d14-cashier-375.png')

  // 加购 → 购物车抽屉
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(500)
  await page.clickSelector('.cart-toggle')
  await sleep(700)
  assert(await page.ev("!!document.querySelector('.cols > .panel:nth-child(2).open')"), 'T4 购物车底部抽屉展开')
  const cartNoH = await page.ev("document.documentElement.scrollWidth <= document.documentElement.clientWidth")
  assert(cartNoH, 'T4 购物车抽屉 375px 无横向滚动条')
  await page.shot('d14-cart-375.png')
  console.log('  截图 d14-cart-375.png')

  // 结算 → 支付弹窗
  await page.clickSelector('button.pay')
  await sleep(800)
  assert(await page.ev("!!document.querySelector('.pay-methods')"), 'T4 支付弹窗打开')
  const payNoH = await page.ev("document.documentElement.scrollWidth <= document.documentElement.clientWidth")
  assert(payNoH, 'T4 支付弹窗 375px 无横向滚动条')
  assert(await page.ev("getComputedStyle(document.querySelector('.modal-actions')).gridTemplateColumns.split(' ').length === 2"), 'T4 支付弹窗按钮全宽 2 列')
  await page.shot('d14-pay-375.png')
  console.log('  截图 d14-pay-375.png')

  // 桌台页复核（375px，移动端「更多」菜单进入）
  await page.ev(`(() => {
    const more = document.querySelector('.more-nav summary')
    if (more) more.click()
    return !!more
  })()`)
  await sleep(400)
  await page.ev("[...document.querySelectorAll('.more-menu button')].find((b) => b.textContent.trim() === '桌台').click()")
  await sleep(900)
  const tblNoH = await page.ev("document.documentElement.scrollWidth <= document.documentElement.clientWidth")
  assert(tblNoH, 'T4 桌台页 375px 无横向滚动条（复核）')

  console.log('\nT5 E2E 全部通过，截图 3 张（d14-*-375.png）')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
