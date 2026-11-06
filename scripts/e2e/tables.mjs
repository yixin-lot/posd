// 跑法：node scripts/e2e/tables.mjs
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

  // 进入桌台页
  await clickTab(page, '桌台')
  await sleep(900)
  assert(await page.ev("!!document.querySelector('.tbl-grid') || [...document.querySelectorAll('button')].some((b) => b.textContent.trim() === '新建桌台')"), '桌台页打开')

  // 新建 A桌、B桌（弹窗 + 输入）
  for (const name of ['A桌', 'B桌']) {
    await page.click('新建桌台')
    await sleep(400)
    await page.setValue('.modal input', name, 0)
    await sleep(200)
    await page.click('创建')
    await sleep(900)
  }
  assert((await page.ev("document.querySelectorAll('.tbl-card').length")) === 2, '两张桌台已建')
  await page.shot(OUT + '/tables-free.png')
  console.log('  空闲态截图 tables-free.png')
  const freeTxt = await page.ev("[...document.querySelectorAll('.tbl-card')].map((c) => c.innerText.replace(/\\n/g, ' ')).join(' | ')")
  console.log('  空闲态：' + freeTxt)

  // A桌 开台（人数 2，珍珠奶茶 x1）
  await page.click('开台')
  await sleep(600)
  await page.setValue('.modal .product-form input', '2', 0)
  await page.setValue('.tbl-pick-item input', '1', 0)
  await sleep(300)
  await page.click('确认开台')
  await sleep(1500)
  const occ = await page.ev("(async () => { const r = await fetch('/pos/v1/tables', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); return JSON.stringify(await r.json()) })()")
  assert(occ.toLowerCase().includes('"status":"occupied"'), 'A桌已占用（接口返回）')
  console.log('  占用态接口：' + occ)
  await page.shot(OUT + '/tables-occupied.png')
  console.log('  占用态截图 tables-occupied.png')
  const occTxt = await page.ev("[...document.querySelectorAll('.tbl-card')].map((c) => c.innerText.replace(/\\n/g, ' ')).join(' | ')")
  console.log('  占用态：' + occTxt)

  // A桌 结账（PaymentModal 现金确认）
  await page.click('结账')
  await sleep(700)
  assert(await page.ev("!!document.querySelector('.pay-methods')"), '结账复用 PaymentModal')
  const payAmt = await page.ev("(document.querySelector('.pay-amount')||{}).textContent||''")
  console.log('  应收显示：' + payAmt)
  await page.click('确认收款')
  await sleep(1800)
  const afterPay = await page.ev("(async () => { const r = await fetch('/pos/v1/tables', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); return JSON.stringify(await r.json()) })()")
  assert(afterPay.toLowerCase().includes('"status":"free"') && afterPay.includes('"id":1'), 'A桌结账后清台为空闲（接口返回）')
  console.log('  结账后接口：' + afterPay)
  await page.shot(OUT + '/tables-after-pay.png')
  console.log('  结账后截图 tables-after-pay.png')
  const afterTxt = await page.ev("[...document.querySelectorAll('.tbl-card')].map((c) => c.innerText.replace(/\\n/g, ' ')).join(' | ')")
  console.log('  结账后：' + afterTxt)

  // B桌 开台 → 清台（验证清台流程与挂单保留提示）
  await page.click('开台')
  await sleep(600)
  await page.setValue('.modal .product-form input', '1', 0)
  await page.setValue('.tbl-pick-item input', '1', 0)
  await sleep(300)
  await page.click('确认开台')
  await sleep(1500)
  await page.click('清台')
  await sleep(600)
  assert(await page.ev("[...document.querySelectorAll('.modal')].some((m) => m.innerText.includes('挂单不会删除'))"), '清台二次确认含挂单保留提示')
  await page.click('确认清台')
  await sleep(1500)
  const afterRelease = await page.ev("(async () => { const r = await fetch('/pos/v1/tables', { headers: { Authorization: 'Bearer ' + localStorage.getItem('pos.token') } }); return JSON.stringify(await r.json()) })()")
  assert(afterRelease.toLowerCase().includes('"status":"free"') && afterRelease.includes('"id":2'), 'B桌清台后为空闲（接口返回）')
  console.log('  清台后接口：' + afterRelease)

  console.log('\n桌台三态 + 清台验收通过，截图 tables-free / tables-occupied / tables-after-pay')
} catch (e) {
  failed = true
  console.error('✗ 失败：' + e.message)
} finally {
  page.close()
  await stack.stop()
}
process.exit(failed ? 1 : 0)
