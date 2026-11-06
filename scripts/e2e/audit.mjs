// 审计：改一次设置 → 审计接口应出现 setting_change；若审计页已交付则连页面一起验。
// 跑法：node scripts/e2e/audit.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'

const stack = await launchStack()
const page = await openPage(9504)
try {
  await openApp(page)

  // 改一次抹零设置（会产生审计）
  const put = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/settings', { method: 'PUT', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify({ round_mode: 'jiao' }) })
    return r.status
  })()`)
  assert(put === 200, '改设置 200')

  const rows = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/audit?limit=10', { headers: { Authorization: 'Bearer ' + tk } })
    return JSON.stringify(await r.json())
  })()`)
  const parsed = JSON.parse(rows)
  const list = Array.isArray(parsed) ? parsed : (parsed.items || [])
  assert(Array.isArray(list) && list.length >= 1, '审计有记录')
  const hit = list.find((x) => x.action === 'setting_change' && x.detail.includes('round_mode'))
  assert(!!hit, '审计含 setting_change / round_mode：' + (hit ? hit.detail : '无'))
  assert(hit.operator && hit.operator.length > 0, '审计有操作人：' + hit.operator)
  console.log('  审计记录：' + hit.operator + ' 改了 ' + hit.detail)

  await clickTab(page, '审计')
  await sleep(1000)
  const hasPage = await page.ev("!!document.querySelector('.audit-table, .audit-view') || document.body.innerText.includes('改设置')")
  if (hasPage) {
    const text = await page.ev("document.body.innerText")
    assert(text.includes('round_mode') || text.includes('改设置'), '审计页显示变更内容')
    await page.shot('e2e-audit.png')
    console.log('\n审计 E2E：接口 + 页面 均通过')
  } else {
    console.log('\n审计 E2E：接口已通过；页面尚未实现，本次跳过 UI 断言')
  }
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  page.close()
  await stack.stop()
}
