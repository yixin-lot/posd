// 结算弹窗：找零计算、现金不足禁用、会员/积分按钮的显示条件、确认时上报的支付方式。
// 这些是"钱算错"最直接的一层，放在组件测试里比 E2E 快得多。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PaymentModal from '../PaymentModal.vue'

function mountPay(props) {
  return mount(PaymentModal, { props })
}

describe('PaymentModal', () => {
  it('默认现金、快捷金额只显示 ≥ 应收，找零按分计算', async () => {
    const w = mountPay({ total: 13 })
    expect(w.find('.pay-amount').text()).toBe('¥13.00')
    // 快捷项去重并过滤掉小于应收的
    const quick = w.findAll('.quicks button').map((b) => b.text())
    expect(quick).toEqual(['13', '20', '50', '100'])
    // 默认实收 = 应收 → 找零 0
    expect(w.find('.change b').text()).toBe('¥0.00')

    await w.find('.quicks button:nth-child(2)').trigger('click') // 20
    expect(w.find('.change b').text()).toBe('¥7.00')
    // 13.87 这类小数不应该出现浮点误差
    const w2 = mountPay({ total: 13.87 })
    await w2.findAll('.quicks button')[1].trigger('click')
    expect(w2.find('.change b').text()).toBe('¥6.13')
  })

  it('现金不足时确认按钮禁用，补足后可点', async () => {
    const w = mountPay({ total: 20 })
    await w.find('input[type=number]').setValue(10)
    const confirm = w.findAll('.modal-actions button')[1]
    expect(confirm.attributes('disabled')).toBeDefined()

    await w.find('input[type=number]').setValue(50)
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

    // 选积分后确认，上报 method=points 且带 memberId
    const pointsBtn = w2.findAll('.pay-methods button').find((b) => b.text() === '积分抵扣')
    await pointsBtn.trigger('click')
    await w2.findAll('.modal-actions button')[1].trigger('click')
    const payload = w2.emitted('confirm')[0][0]
    expect(payload.method).toBe('points')
    // 积分抵扣不挂会员余额，memberId 回 0（后端只按积分核销）
    expect(payload.memberId).toBe(0)
  })

  it('扫码只记账：提示语包含应付金额，确认上报对应方式', async () => {
    const w = mountPay({ total: 26 })
    const wechat = w.findAll('.pay-methods button').find((b) => b.text() === '微信')
    await wechat.trigger('click')
    expect(w.find('.hint').text()).toContain('¥26.00')
    await w.findAll('.modal-actions button')[1].trigger('click')
    expect(w.emitted('confirm')[0][0].method).toBe('wechat')
  })
})
