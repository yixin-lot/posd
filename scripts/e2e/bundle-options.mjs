// 套餐可选项验收：收银台点套餐 → 选组内一个 → 加入购物车 → 挂单 → 组件快照正确
// 跑法：node scripts/e2e/bundle-options.mjs
import { launchStack, openPage, openApp, assert, sleep, clickTab } from './lib.mjs'

const stack = await launchStack()
const page = await openPage(9507)
try {
  await openApp(page)

  // 造数：两个可选项 + 一个固定组件
  const seeded = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const H = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const mk = async (body) => (await (await fetch('/pos/v1/products', { method: 'POST', headers: H, body: JSON.stringify(body) })).json())
    const tea = await mk({ name: 'E2E珍珠奶茶', category: 'E2E奶茶', price: 13 })
    const fruit = await mk({ name: 'E2E果茶', category: 'E2E果茶', price: 15 })
    const cake = await mk({ name: 'E2E蛋挞', category: 'E2E甜点', price: 6 })
    const bundle = await mk({ name: 'E2E自选套餐', category: 'E2E套餐', price: 20, kind: 'bundle', bundle: [
      { componentId: String(tea.id), qty: 1, groupId: 1 },
      { componentId: String(fruit.id), qty: 1, groupId: 1 },
      { componentId: String(cake.id), qty: 1, groupId: 0 }
    ] })
    return JSON.stringify({ bundleName: bundle.name, fruitName: fruit.name, teaName: tea.name })
  })()`)
  const seed = JSON.parse(seeded)
  assert(seed.bundleName === 'E2E自选套餐', '造数成功（套餐 + 两个可选项）')

  await clickTab(page, '收银')
  await sleep(800)
  // 重新拉商品（刚通过接口创建）
  await page.ev('location.reload()')
  await sleep(1500)
  await openApp(page)
  await sleep(800)

  // 类目默认选中第一个，会过滤掉新商品：先切「全部」
  await page.ev("[...document.querySelectorAll('.cats button')].find((b) => b.textContent.trim() === '全部')?.click()")
  await sleep(500)
  for (let i = 0; i < 20; i++) {
    if (await page.ev("document.querySelectorAll('.grid .card').length > 0")) break
    await sleep(300)
  }
  const cardCount = await page.ev("document.querySelectorAll('.grid .card').length")
  assert(cardCount > 0, '收银台商品卡片已加载（实际 ' + cardCount + '）')

  const names = await page.ev("[...document.querySelectorAll('.grid .card')].map((c) => c.innerText.replace(/\\s+/g,' ')).join(' | ')")
  console.log('  卡片：' + names)

  const clicked = await page.ev(`(() => {
    const cards = [...document.querySelectorAll('.grid .card')]
    const c = cards.find((x) => x.textContent.includes('E2E自选套餐'))
    if (!c) return false
    c.click()
    return true
  })()`)
  assert(clicked, '找到套餐卡片并点击')

  let modalOpen = false
  for (let i = 0; i < 20; i++) {
    modalOpen = await page.ev("!!document.querySelector('.modal h3') && document.querySelector('.modal h3').textContent.includes('选择套餐内容')")
    if (modalOpen) break
    await sleep(200)
  }
  const diag = await page.ev("JSON.stringify({ masks: document.querySelectorAll('.mask').length, carts: (document.querySelector('.cart')||document.body).innerText.replace(/\\s+/g,' ').slice(0,80) })")
  console.log('  诊断：' + diag)
  assert(modalOpen, '套餐可选项弹窗打开（' + diag + '）')

  // 选果茶
  const picked = await page.ev(`(() => {
    const b = [...document.querySelectorAll('.modal button')].find((x) => x.textContent.trim() === 'E2E果茶')
    if (!b) return false
    b.click()
    return true
  })()`)
  assert(picked, '选择果茶')
  await page.ev("[...document.querySelectorAll('.modal button')].find((b) => b.textContent.trim() === '加入购物车').click()")
  await sleep(600)

  const cartText = await page.ev("(document.querySelector('.cart')||document.body).innerText.replace(/\\s+/g,' ')")
  assert(cartText.includes('（E2E果茶）'), '购物车显示所选组件（实际：' + cartText.slice(0, 120) + '）')

  // 挂单，再回读订单校验组件快照
  await page.ev("[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === '挂单').click()")
  await sleep(1500)

  const check = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const list = await (await fetch('/pos/v1/orders?limit=5', { headers: { Authorization: 'Bearer ' + tk } })).json()
    const o = list.find((x) => x.status === 'held')
    if (!o) return JSON.stringify({ ok: false, why: 'no held order' })
    const item = o.items.find((i) => i.name === 'E2E自选套餐')
    if (!item || !item.components) return JSON.stringify({ ok: false, why: 'no components' })
    return JSON.stringify({ ok: true, names: item.components.map((c) => c.name) })
  })()`)
  const res = JSON.parse(check)
  assert(res.ok, '挂单里能读到套餐组件（' + (res.why || '') + '）')
  assert(res.names.includes('E2E果茶'), '组件含所选果茶（实际：' + res.names.join(',') + '）')
  assert(!res.names.includes('E2E珍珠奶茶'), '组件不含未选的奶茶')
  assert(res.names.includes('E2E蛋挞'), '固定组件仍在')

  await page.shot('e2e-bundle-options.png')
  console.log('\n套餐可选项验收通过：选组内一个 / 购物车标注 / 组件快照正确')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  await stack.stop()
}
