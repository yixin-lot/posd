// 生成手册用截图（1440 宽，明暗两种主题各一套核心页）。
// 跑法：node scripts/e2e/shots.mjs    产物：docs/shots/*.png
import { launchStack, openPage, openApp, clickTab, sleep } from './lib.mjs'
import { mkdirSync } from 'node:fs'

mkdirSync('docs/shots', { recursive: true })

const stack = await launchStack()
const page = await openPage()
try {
  await openApp(page)
  for (let i = 0; i < 20; i++) {
    if (await page.ev("!!document.querySelector('.statusbar')")) break
    await sleep(300)
  }

  // 先造一笔单，报表页才有内容
  await page.ev("document.querySelector('.grid .card').click()")
  await sleep(400)
  await page.ev("[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === '结算')?.click()")
  await sleep(600)
  await page.ev("[...document.querySelectorAll('.modal button')].find((b) => b.textContent.includes('确认收款'))?.click()")
  await sleep(1200)

  await page.shot('docs/shots/cashier.png')

  await clickTab(page, '商品')
  await sleep(800)
  await page.shot('docs/shots/products.png')

  // 盘点弹窗
  await page.ev("[...document.querySelectorAll('.orders-head button')].find((b) => b.textContent.includes('盘点'))?.click()")
  await sleep(300)
  await page.ev("[...document.querySelectorAll('button')].find((b) => b.textContent.includes('开始新盘点'))?.click()")
  await sleep(700)
  await page.shot('docs/shots/stocktake.png')
  await page.ev("[...document.querySelectorAll('.modal-actions button')].find((b) => b.textContent.includes('关闭'))?.click()")
  await sleep(300)

  await clickTab(page, '报表')
  await sleep(900)
  await page.shot('docs/shots/reports.png')

  console.log('截图已生成：docs/shots/{cashier,products,stocktake,reports}.png')
} finally {
  await stack.stop()
}
