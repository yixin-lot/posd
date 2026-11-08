// 跑法：node scripts/e2e/d14.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage()
let failed = false
try {
  // 前端走 embed（8080，与发布一致）；vite dev 在本机偶发白屏（#app 空）不影响本用例
  await page.ev("location.href = 'http://127.0.0.1:8080/'")
  await sleep(2000)
  await login(page)
  // 登录偶发不落地（首屏挂载快慢）：重试一次，仍不行就把 PIN 框值与错误文案打出来
  if (!(await page.ev("!!localStorage.getItem('pos.token')"))) {
    console.log('login retry: pinFields=' + JSON.stringify(await page.ev("[...document.querySelectorAll('.login-field input')].map((i) => i.value)").catch(() => [])))
    await sleep(600)
    await login(page)
  }
  if (!(await page.ev("!!localStorage.getItem('pos.token')"))) {
    console.log('login failed: err=' + String((await page.ev("(document.querySelector('.login-error') || {}).textContent || ''")) || ''))
  }
  await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const s = await (await fetch('/pos/v1/settings', { headers: h })).json()
    await fetch('/pos/v1/settings', { method: 'PUT', headers: h, body: JSON.stringify({ ...s, setup_done: 'true' }) })
    return true
  })()`)
  await page.ev("location.reload()")
  // 首屏偶发加载慢：轮询到状态条或登录框出现再断言，别用固定等待
  for (let i = 0; i < 20; i++) {
    if (await page.ev("!!document.querySelector('.statusbar') || !!document.querySelector('.login-btn')")) break
    await sleep(500)
  }
  // 断言失败时给出停在哪个页面、token 在不在，避免只看到一个 ✗
  console.log('debug token=' + (await page.ev("!!localStorage.getItem('pos.token')")) + ' body=' + String((await page.ev("document.body.innerText")) || '').replace(/\s+/g, ' ').slice(0, 120))
  assert(await page.ev("!!document.querySelector('.statusbar')"), '登录成功，进入收银台')

  // ---- 造数：1 paid + 1 hold + 桌台 A1（开台自建挂单绑定） ----
  const seed = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const h = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const r1 = await fetch('/pos/v1/orders', { method: 'POST', headers: h, body: JSON.stringify({ cashMinor: 2000, items: [{ productId: '1', qty: 1 }] }) })
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
  await page.ev("document.querySelectorAll('.filters-row select')[0].value = 'today'")
  await page.ev("document.querySelectorAll('.filters-row select')[0].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  const todayRows = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  const qToday = await page.ev("location.search")
  assert(todayRows >= 1 && qToday.includes('date=today'), 'T1 切「今天」数量变化且 URL=?date=today（' + todayRows + ' 笔 ' + qToday + '）')
  await page.ev("document.querySelectorAll('.filters-row select')[0].value = ''")
  await page.ev("document.querySelectorAll('.filters-row select')[0].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  assert((await page.ev("location.search")).includes('date=') === false, 'T1 切回「全部」URL query 清空')

  // v03 P4 稿面只有三个胶囊（日期 / 支付方式 / 收银员）+ 搜索，状态胶囊已按稿移除
  const selCount = await page.ev("document.querySelectorAll('.filters-row select').length")
  assert(selCount === 3, 'T1 筛选胶囊与稿面一致（日期/支付/收银员 共 3 个），实际 ' + selCount)
  await page.ev("document.querySelectorAll('.filters-row select')[1].value = 'cash'")
  await page.ev("document.querySelectorAll('.filters-row select')[1].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(600)
  const cashRows = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  assert(cashRows >= 1 && cashRows <= 3, 'T1 支付方式筛选「现金」→ 行数收敛（' + cashRows + ' 笔）')
  await page.ev("document.querySelectorAll('.filters-row select')[1].value = ''")
  await page.ev("document.querySelectorAll('.filters-row select')[1].dispatchEvent(new Event('change', { bubbles: true }))")
  await sleep(400)

  // 收银员列必须有值（V31 起 operator 落到订单行，不再是全「—」）
  const cashierVals = await page.ev(`(() => {
    const rows = [...document.querySelectorAll('.orders-table tbody tr')].filter((r) => r.children.length >= 8)
    return rows.map((r) => r.children[5].textContent.trim())
  })()`)
  assert(Array.isArray(cashierVals) && cashierVals.some((v) => v && v !== '—'), 'T1 订单「收银员」列有值：' + JSON.stringify(cashierVals))

  // ---- T3 设置页打印区块（v03 分组版：左导航卡 + 打印设置分组默认可见） ----
  // 管理页有左侧 nav64（订单→设置直接点）；收银台无侧栏，退回 ☰ 菜单
  const t3nav = await page.ev(`(() => {
    const nav = [...document.querySelectorAll('.nav64 .it')].find((b) => b.textContent.includes('设置'))
    if (nav) { nav.click(); return 'nav64' }
    const more = document.querySelector('.more-btn')
    if (!more) return 'none'
    more.click()
    return 'drawer'
  })()`)
  await sleep(500)
  if (t3nav === 'drawer') {
    await page.ev("[...document.querySelectorAll('.menu-drawer .mi')].find((b) => b.textContent.includes('设置')).click()")
  }
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.settings-nav button.active')"), 'T3 设置页左侧导航卡渲染')
  assert(await page.ev("[...document.querySelectorAll('.print-mode-opt')].length === 3"), 'T3 打印模式三选项网格')
  const secText = String((await page.ev("document.querySelector('.settings-body') ? document.querySelector('.settings-body').innerText : ''")) || '')
  assert(secText.includes('本地直连') && secText.includes('不打印'), 'T3 打印模式三档齐全（本地直连/云打印 API/不打印）')
  assert(secText.includes('打印机') && secText.includes('后厨打印') && secText.includes('打印队列'), 'T3 三卡片齐全（打印机/后厨/队列）')
  // 打印队列已接真实接口（GET /pos/v1/print/jobs），不再是 TODO 占位
  assert(
    !secText.includes('TODO') &&
      (secText.includes('排队中') || secText.includes('已完成') || secText.includes('还没有打印任务')),
    'T3 打印队列接真实数据（无 TODO 占位）'
  )
  // 打印测试交互（先把端口切「手动填写」才出现输入框，再填 tcp:// 断连地址 → 错误消息）
  await page.ev(`(() => {
    const sel = document.querySelector('.printer-port select')
    if (!sel) return false
    const set = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set
    set.call(sel, 'custom')
    sel.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`)
  await sleep(400)
  // 设置页（v03 P7 分组卡）：打印 / 后厨 / 打印队列三张卡齐全
  const setText = String((await page.ev("document.querySelector('.settings-body') ? document.querySelector('.settings-body').innerText : ''")) || '')
  assert(setText.includes('打印设置') && setText.includes('后厨打印') && setText.includes('打印队列'), 'T3 三卡片齐全（打印设置/后厨打印/打印队列）')
  // 打印队列：已接真实接口。要么有任务行，要么空态「还没有打印任务」；
  // 但绝不能再出现 TODO 占位文案。
  const queueText = await page.ev(`(() => {
    const t = [...document.querySelectorAll('.shift-card h3')].find((h) => h.textContent.includes('打印队列'))
    return t ? t.parentElement.innerText : ''
  })()`)
  assert(queueText.includes('单号') && queueText.includes('状态'), 'T3 打印队列表格已渲染')
  assert(!queueText.includes('TODO') && !queueText.includes('暂未提供'), 'T3 打印队列不再是占位')
  await page.ev(`(() => {
    const el = document.querySelector('.printer-row input')
    if (!el) return false
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
    set.call(el, 'tcp://127.0.0.1:1')
    el.dispatchEvent(new Event('input', { bubbles: true }))
    return true
  })()`)
  await sleep(300)
  await page.click('测试打印')
  await sleep(2000)
  const afterTest = String((await page.ev("document.querySelector('.settings-body') ? document.querySelector('.settings-body').innerText : ''")) || '')
  assert(afterTest.includes('✗') || afterTest.includes('连不上'), 'T3 打印测试交互返回错误消息：' + afterTest.replace(/\n/g, ' ').slice(0, 80))

  // ---- T5 导航：单一清单 10 项 + 任意管理页可直达 + 退出登录常驻顶栏 ----
  const railLabels = await page.ev("[...document.querySelectorAll('.nav64 .it')].map((b) => b.textContent.trim())")
  assert(Array.isArray(railLabels) && railLabels.length === 10, 'T5 管理页侧栏 10 项，实际 ' + (Array.isArray(railLabels) ? railLabels.join('/') : 'none'))
  const wantLabels = ['收银台', '订单查询', '桌台', '商品管理', '交接班', '会员', '报表', '审计', '员工', '设置']
  assert(wantLabels.every((l) => railLabels.includes(l)), 'T5 侧栏补齐桌台/会员/报表/审计/员工五个入口')
  // 设置页一次点侧栏「报表」直达，不回收银台、不开 ☰
  await page.ev("[...document.querySelectorAll('.nav64 .it')].find((b) => b.textContent.includes('报表')).click()")
  await sleep(900)
  const rptTitle = String((await page.ev("document.querySelector('.orders-head h2') ? document.querySelector('.orders-head h2').textContent.trim() : ''")) || '')
  assert(rptTitle.includes('报表'), 'T5 设置页 → 报表 一次可达（不跳中间页），标题 ' + rptTitle)
  // 退出登录常驻顶栏：连切三页仍可见可点
  const logoutPages = []
  for (const k of ['审计', '员工', '交接班']) {
    await page.ev(`[...document.querySelectorAll('.nav64 .it')].find((b) => b.textContent.includes('${k}')).click()`)
    await sleep(800)
    const v = await page.ev("(() => { const b = document.querySelector('.statusbar .sb-logout'); if (!b) return 'no'; const r = b.getBoundingClientRect(); return r.width > 20 && r.right <= window.innerWidth + 1 ? 'ok' : 'hidden' })()")
    logoutPages.push(k + ':' + v)
  }
  assert(logoutPages.every((s) => s.endsWith(':ok')), 'T5 退出登录常驻顶栏（审计/员工/交接班均可见）：' + logoutPages.join(', '))
  // ☰ 仍只是收银台快捷入口，且不再承载退出登录
  await page.ev("[...document.querySelectorAll('.nav64 .it')].find((b) => b.textContent.includes('收银台')).click()")
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.more-btn')"), 'T5 收银台 ☰ 入口仍在')
  await page.ev("(() => { const b = document.querySelector('.more-btn'); if (b) b.click(); return !!b })()")
  await sleep(600)
  const drawerLabels = await page.ev("[...document.querySelectorAll('.menu-drawer .mi')].map((b) => (b.querySelector('b') ? b.querySelector('b').textContent : ''))")
  assert(Array.isArray(drawerLabels) && drawerLabels.length >= 10, 'T5 ☰ 抽屉与侧栏同一份清单，实际 ' + (Array.isArray(drawerLabels) ? drawerLabels.length : 0) + ' 项')
  assert((await page.ev("!!document.querySelector('.menu-drawer .logout')")) === false, 'T5 退出登录只在顶栏，抽屉内不再重复')
  await page.ev("(() => { const d = document.querySelector('.menu-drawer'); if (d) d.click(); return 1 })()")
  await sleep(400)

  // ---- T4 375px 移动端（三截图 + 无横向滚动；嵌定 8080 embed：
  //     5173 跨 origin 无登录态且偶发白屏，断言无意义） ----
  await page.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 667, deviceScaleFactor: 2, mobile: true })
  await page.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 1 })
  await page.ev("location.href = 'http://127.0.0.1:8080/'")
  for (let i = 0; i < 20; i++) {
    if (await page.ev("!!document.querySelector('.statusbar') || !!document.querySelector('.login-btn')")) break
    await sleep(500)
  }
  // 移动端刷新后登录态可能只存在内存里而回落登录页，补一次登录再继续断言
  if (await page.ev("!!document.querySelector('.login-btn')")) await login(page)
  await sleep(800)
  // v03 侧栏是主导航；旧 .tabs 已 display:none 仅作回退
  await page.ev(`(() => {
    const nav = [...document.querySelectorAll('.nav64 .it')].find((b) => b.textContent.includes('收银台'))
    if (nav) return nav.click()
    const t = [...document.querySelectorAll('.tabs button')].find((b) => b.textContent.trim() === '收银')
    if (t) return t.click()
    return false
  })()`)
  await sleep(900)
  const w375 = await page.ev("document.documentElement.clientWidth")
  assert(w375 === 375, '视口宽 375px（实际 ' + w375 + '）')
  const cashNoH = await page.ev(`(() => {
    const sw = document.documentElement.scrollWidth
    const cw = document.documentElement.clientWidth
    const bad = []
    if (sw > cw) {
      for (const el of document.querySelectorAll('*')) {
        const r = el.getBoundingClientRect()
        if (r.width && r.right > cw + 0.5) bad.push(String(el.className || el.tagName).slice(0, 24) + '@' + Math.round(r.right))
      }
    }
    return { sw, cw, bad: bad.slice(0, 8) }
  })()`)
  assert(cashNoH.sw <= cashNoH.cw, 'T4 收银台 375px 无横向滚动条（scrollWidth=' + cashNoH.sw + ' clientWidth=' + cashNoH.cw + ' 溢出项=' + JSON.stringify(cashNoH.bad) + '）')
  const gcols = await page.ev("document.querySelector('.grid') ? getComputedStyle(document.querySelector('.grid')).gridTemplateColumns : 'NO-GRID'")
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

  // 桌台页复核（375px，移动端 ☰ 抽屉进入）
  await page.ev(`(() => {
    const more = document.querySelector('.more-btn')
    if (more) more.click()
    return !!more
  })()`)
  await sleep(500)
  await page.ev("[...document.querySelectorAll('.menu-drawer .mi')].find((b) => b.textContent.includes('桌台')).click()")
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
