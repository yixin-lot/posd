// 跑法：node scripts/e2e/d13.mjs
// 设计：全部走 HTTP，不依赖浏览器 DOM 文案、不依赖 python 直写库（旧版两处都会导致假失败）。
import { launchStack, assert } from './lib.mjs'

const stack = await launchStack()
try {
  const base = 'http://127.0.0.1:8080'
  const call = async (method, path, body, token) => {
    const res = await fetch(base + path, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: 'Bearer ' + token } : {})
      },
      body: body === undefined ? undefined : JSON.stringify(body)
    })
    let data = null
    try { data = await res.json() } catch (e) { /* 204 */ }
    return { status: res.status, data }
  }

  const login = await call('POST', '/pos/v1/auth/login', { username: 'admin', pin: '1234' })
  assert(login.status === 200 && login.data.token, '登录成功')
  const tk = login.data.token

  const shift = await call('POST', '/pos/v1/shifts/open', { openingCash: 100 }, tk)
  assert(shift.status === 200 || shift.status === 201, '开班')

  // ---- 并台：两张占用桌 ----
  const a1 = (await call('POST', '/pos/v1/tables', { name: 'A1' }, tk)).data
  const a2 = (await call('POST', '/pos/v1/tables', { name: 'A2' }, tk))
  assert(a2.status === 201, '建桌 A1 / A2')
  const a2t = a2.data
  assert((await call('POST', '/pos/v1/tables/' + a1.id + '/open', { guests: 2, items: [{ productId: '1', qty: 2 }] }, tk)).status === 200, 'A1 开台（2 杯）')
  assert((await call('POST', '/pos/v1/tables/' + a2t.id + '/open', { guests: 2, items: [{ productId: '1', qty: 1 }] }, tk)).status === 200, 'A2 开台（1 杯）')

  const merged = await call('POST', '/pos/v1/tables/' + a1.id + '/merge', { intoTableId: a2t.id }, tk)
  assert(merged.status === 200, '并台接口 200（实际 ' + merged.status + '）')
  assert(merged.data.from.status === 'free', '并台后原桌空闲')
  assert(Number(merged.data.order.payableMinor) === 3900, '并台后目标桌应收 39（实际 ' + merged.data.order.payableMinor + '）')
  assert(Array.isArray(merged.data.order.items) && merged.data.order.items.length === 2, '并台后 2 行明细')

  // ---- 转台：换到空桌 ----
  const b1 = (await call('POST', '/pos/v1/tables', { name: 'B1' }, tk)).data
  const moved = await call('POST', '/pos/v1/tables/' + a2t.id + '/transfer', { toTableId: b1.id }, tk)
  assert(moved.status === 200, '转台接口 200')
  assert(moved.data.from.status === 'free' && moved.data.to.status === 'occupied', '转台后原桌空闲 / 目标桌占用')
  const orderOnB1 = moved.data.to.orderNo
  assert(orderOnB1, '转台后目标桌带订单号')

  // ---- 会员等级折扣进入交接班统计 ----
  assert((await call('PUT', '/pos/v1/settings', { member_tiers: '0:普通会员:0:1,100:银卡会员:10:1' }, tk)).status === 200, '设置银卡 10% 折扣')
  const mb = (await call('POST', '/pos/v1/members', { name: '折扣测试', phone: '13800000001' }, tk)).data
  assert((await call('POST', '/pos/v1/members/' + mb.id + '/topup', { amount: 300 }, tk)).status === 200, '会员充值 300')

  const first = await call('POST', '/pos/v1/orders', {
    cashMinor: 10400, memberId: mb.id,
    items: [{ productId: '1', qty: 8 }],
    payments: [{ method: 'member', amountMinor: 10400, memberId: mb.id }]
  }, tk)
  assert(first.status === 201, '会员首单 104（累计消费升级）')
  const second = await call('POST', '/pos/v1/orders', {
    cashMinor: 9360, memberId: mb.id,
    items: [{ productId: '1', qty: 8 }],
    payments: [{ method: 'member', amountMinor: 9360, memberId: mb.id }]
  }, tk)
  assert(second.status === 201, '会员第二单 93.6（银卡价）')
  assert(Number(second.data.memberDiscountMinor) === 1040, '第二单会员折扣 10.40（实际 ' + second.data.memberDiscount + '）')

  const latest = await call('GET', '/pos/v1/shifts/latest', undefined, tk)
  assert(latest.status === 200, '读交接班')
  const disc = Number(latest.data.summary.memberDiscountTotalMinor) / 100
  assert(Math.abs(disc - 10.4) < 0.001, '交接班会员折扣合计 10.40（实际 ' + disc + '）')

  // ---- 档口打印机地址校验 ----
  const bad = await call('POST', '/pos/v1/kitchen/stations', { name: '水吧', printer: 'tcp://192.168.1.50', categories: '奶茶' }, tk)
  assert(bad.status === 400, '缺端口的 tcp 地址被拒（实际 ' + bad.status + '）')
  const good = await call('POST', '/pos/v1/kitchen/stations', { name: '水吧', printer: 'tcp://192.168.1.50:9100', categories: '奶茶' }, tk)
  assert(good.status === 201, '合法 tcp 地址创建成功（实际 ' + good.status + '）')

  // ---- 到期接口（无数据也要返回空数组）----
  const exp = await call('GET', '/pos/v1/members/expiring?days=7', undefined, tk)
  assert(exp.status === 200 && Array.isArray(exp.data), '到期查询返回数组')

  console.log('\nD13 回归（纯 API）通过：并台 / 转台 / 会员折扣 / 档口校验 / 到期接口')
} catch (e) {
  console.error('✗ 失败：' + e.message)
  process.exitCode = 1
} finally {
  await stack.stop()
}
