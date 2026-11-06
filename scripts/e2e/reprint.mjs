// 重打小票：验证「重新读订单」这条路径上的字段完整（会员折扣/积分/支付方式）。
// 背景：Get 曾经少 SELECT 字段，重打就会丢会员折扣行；本用例守的是纸面内容。
// 跑法：node scripts/e2e/reprint.mjs
import { readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { launchStack, openPage, login, assert, sleep, openApp } from './lib.mjs'

function readPrint() {
  if (!existsSync(PRINT_FILE)) return ''
  const buf = readFileSync(PRINT_FILE)
  return new TextDecoder('gbk').decode(buf)
}

const stack = await launchStack()
// 无打印机时 posd 把字节写到「数据库同目录」的 print.bin
const PRINT_FILE = join(stack.dataDir, 'print.bin')
const page = await openPage(9505)
try {
  await openApp(page)

  // 让所有会员享 10% 折扣，便于验证「会员折扣」行
  const put = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/settings', { method: 'PUT', headers: { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }, body: JSON.stringify({ member_tiers: '0:测试会员:10:1' }) })
    return r.status
  })()`)
  assert(put === 200, '设置会员折扣 10%')

  const order = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const H = { Authorization: 'Bearer ' + tk, 'Content-Type': 'application/json' }
    const mk = await fetch('/pos/v1/members', { method: 'POST', headers: H, body: JSON.stringify({ phone: '13900001234', name: '重打测试' }) })
    const m = await mk.json()
    await fetch('/pos/v1/members/' + m.id + '/topup', { method: 'POST', headers: H, body: JSON.stringify({ amount: 100 }) })
    const r = await fetch('/pos/v1/orders', { method: 'POST', headers: H, body: JSON.stringify({
      memberId: m.id,
      payments: [{ method: 'member', amount: 23.40, memberId: m.id }],
      items: [{ productId: '1', qty: 2 }]
    }) })
    const o = await r.json()
    return JSON.stringify({ status: r.status, no: o.orderNo, total: o.total, payable: o.payable, discount: o.memberDiscount, points: o.pointsEarned })
  })()`)
  const o = JSON.parse(order)
  assert(o.status === 201, '下单成功（201）')
  assert(o.payable === 23.4, '应收 = 23.40（实际 ' + o.payable + '）')
  assert(o.discount === 2.6, '会员折扣 = 2.60（实际 ' + o.discount + '）')
  assert(o.points === 23, '本单积分 = 23（实际 ' + o.points + '）')

  await sleep(1500)
  const first = readPrint()
  assert(first.includes('会员折扣') && first.includes('-2.60'), '首打小票含会员折扣 -2.60')
  assert(first.includes('本单积分') && first.includes('+23'), '首打小票含本单积分 +23')
  assert(first.includes('会员余额'), '首打小票含支付方式「会员余额」')

  // 重打：这条路径会重新 Get 订单，字段缺失就会掉行
  const before = readPrint().length
  const re = await page.ev(`(async () => {
    const tk = localStorage.getItem('pos.token')
    const r = await fetch('/pos/v1/orders/${o.no}/reprint', { method: 'POST', headers: { Authorization: 'Bearer ' + tk } })
    return JSON.stringify({ status: r.status, body: await r.text() })
  })()`)
  const rr = JSON.parse(re)
  assert(rr.status === 202 || rr.status === 200, '重打接口受理（' + rr.status + '）')

  await sleep(1500)
  const after = readPrint()
  assert(after.length > before, '重打确实又出了一张（字节数增加）')
  const tail = after.slice(before)
  assert(tail.includes('会员折扣') && tail.includes('-2.60'), '重打小票仍含会员折扣')
  assert(tail.includes('本单积分') && tail.includes('+23'), '重打小票仍含本单积分')
  assert(tail.includes('会员余额'), '重打小票仍含支付方式')

  console.log('\n重打小票 E2E 通过：会员折扣 / 本单积分 / 支付方式 三条都在')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  page.close()
  await stack.stop()
}
