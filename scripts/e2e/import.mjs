// 商品批量导入：走 UI 对话框（选文件 → 预览 → 导入），再核对商品表。
// 跑法：node scripts/e2e/import.mjs
import { launchStack, openPage, login, assert, sleep, clickTab, openApp } from './lib.mjs'
import { writeFileSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const dir = mkdtempSync(join(tmpdir(), 'pos-csv-'))
const csvPath = join(dir, 'products.csv')
writeFileSync(csvPath, '名称,条码,类目,售价,成本,热销\nE2E商品A,770000000001,测试,9.9,3,是\nE2E商品B,770000000002,测试,12,4,否\n', 'utf8')

const stack = await launchStack()
const page = await openPage(9501)
try {
  await openApp(page)
  await clickTab(page, '商品')
  await sleep(900)
  await page.click('导入')
  await sleep(800)
  assert(await page.ev("!!document.querySelector('.idlg-mask')"), '导入对话框打开')

  const before = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  assert(await page.setFile('#csv-input', csvPath), '选择 CSV 文件')
  await sleep(800)
  const preview = await page.ev("document.querySelectorAll('.idlg-table tbody tr').length")
  assert(preview === 2, '预览 2 行（实际 ' + preview + '）')

  await page.clickSelector('.idlg-btn.primary')
  await sleep(2000)
  await page.clickSelector('.idlg-x')
  await sleep(1000)
  const after = await page.ev("document.querySelectorAll('.orders-table tbody tr').length")
  assert(after === before + 2, '商品数 +2（' + before + ' → ' + after + '）')
  console.log('\n导入通过')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  page.close()
  await stack.stop()
}
