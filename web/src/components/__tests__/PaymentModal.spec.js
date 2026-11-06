// 结算弹窗（v03 P3 四档胶囊 + 会员支付）：找零计算、现金不足禁用、会员/积分按钮的显示条件、确认时上报的支付方式。
// 这些是"钱算错"最直接的一层，放在组件测试里比 E2E 快得多。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PaymentModal from '../PaymentModal.vue'

function mountPay(props) {
  return mount(PaymentModal, { props })
}

describe('PaymentModal', () => {
  it('默认现金：实收=应收、找零按分计算、快捷额固定 50/100/其他', async () => {
    const w = mountPay({ total: 13 })
    expect(w.find('.pay-amount').text()).toBe('¥13.00')
    expect(w.findAll('.quicks button').map((b) => b.text())).toEqual(['50', '100', '其他'])
    expect(w.find('.change').text()).toContain('¥0.00')

    await w.findAll('.quicks button')[0].trigger('click') // 50
    expect(w.find('.change').text()).toContain('¥37.00')

    // 13.87 这类小数不应该出现浮点误差
    const w2 = mountPay({ total: 13.87 })
    await w2.findAll('.quicks button')[0].trigger('click')
    expect(w2.find('.change').text()).toContain('¥36.13')
  })

  it('现金不足时确认按钮禁用，补足后可点', async () => {
    const w = mountPay({ total: 20 })
    await w.find('.cash-input').setValue('10')
    const confirm = w.findAll('.modal-actions button')[1]
    expect(confirm.attributes('disabled')).toBeDefined()

    await w.find('.cash-input').setValue('50')
    expect(confirm.attributes('disabled')).toBeUndefined()
    await confirm.trigger('click')
    const payload = w.emitted('confirm')[0][0]
    expect(payload.method).toBe('cash')
    expect(payload.cash).toBe(50)
  })

  it('会员余额总在；积分够本单才出现（100 积分=1 元）', async () => {
    const w = mountPay({ total: 13, member: { name: '张三', points: 500, balance: 100 } })
    const labels = w.findAll('.pay-methods button').map((b) => b.text())
    expect(labels).toContain('会员余额')
    expect(labels).not.toContain('积分抵扣') // 500 < 1300 分

    const w2 = mountPay({ total: 13, member: { name: '李四', points: 2000, balance: 100 } })
    expect(w2.findAll('.pay-methods button').map((b) => b.text())).toContain('积分抵扣')

    // 选积分后确认，上报 method=points
    const pointsBtn = w2.findAll('.pay-methods button').find((b) => b.text() === '积分抵扣')
    await pointsBtn.trigger('click')
    await w2.findAll('.modal-actions button')[1].trigger('click')
    const payload = w2.emitted('confirm')[0][0]
    expect(payload.method).toBe('points')
    // 积分抵扣不挂会员余额，memberId 回 0（后端只按积分核销）
    expect(payload.memberId).toBe(0)
  })

  it('会员余额支付带 memberId，余额不足时确认禁用', async () => {
    const w = mountPay({ total: 200, member: { id: 7, name: '王五', points: 0, balance: 100 } })
    await w.findAll('.pay-methods button').find((b) => b.text() === '会员余额').trigger('click')
    expect(w.findAll('.modal-actions button')[1].attributes('disabled')).toBeDefined()

    const w2 = mountPay({ total: 50, member: { id: 7, name: '王五', points: 0, balance: 100 } })
    await w2.findAll('.pay-methods button').find((b) => b.text() === '会员余额').trigger('click')
    expect(w2.findAll('.modal-actions button')[1].attributes('disabled')).toBeUndefined()
    await w2.findAll('.modal-actions button')[1].trigger('click')
    expect(w2.emitted('confirm')[0][0].method).toBe('member')
    expect(w2.emitted('confirm')[0][0].memberId).toBe(7)
  })

  it('扫码只记账：提示语包含应付金额，确认上报对应方式', async () => {
    const w = mountPay({ total: 26 })
    const scan = w.findAll('.pay-methods button').find((b) => b.text() === '扫码')
    await scan.trigger('click')
    expect(w.find('.hint').text()).toContain('¥26.00')
    await w.findAll('.modal-actions button')[1].trigger('click')
    expect(w.emitted('confirm')[0][0].method).toBe('wechat')
  })
})
