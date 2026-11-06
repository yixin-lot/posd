// 目的：防止"保存把配置打回默认"这类回归（读回来的值和提交的值不一致）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// 只 mock 前端 api 层，不碰真实 HTTP
vi.mock('../../api/index.js', () => ({
  api: {
    getSettings: vi.fn(),
    saveSettings: vi.fn(),
    listStations: vi.fn(),
    listPrintJobs: vi.fn()
  }
}))

import { api } from '../../api/index.js'
import SettingsView from '../SettingsView.vue'

const FULL = {
  shop_name: '楼下奶茶',
  shop_phone: '13800001111',
  shop_footer: '谢谢惠顾',
  receipt_footer: '谢谢惠顾，欢迎再来',
  round_mode: 'jiao',
  points_earn_per_yuan: '1',
  points_per_yuan: '100',
  points_expire_months: '12',
  member_tiers: '0:普通会员:0:1,1000:银卡会员:2:2',
  topup_bonus_rules: '500:50',
  kitchen_printer: '厨房打印机',
  kitchen_categories: '奶茶,小吃',
  perm_refund_cashier: 'true',
  perm_discount_cashier: 'true',
  perm_change_price_cashier: 'false',
  perm_void_cashier: 'false'
}

function mountView() {
  return mount(SettingsView, { props: { role: 'manager' } })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.listStations.mockResolvedValue([])
  api.listPrintJobs.mockResolvedValue({ jobs: [] })
})

describe('SettingsView 回填', () => {
  it('GET /settings 返回完整对象后，文本字段、抹零方式、权限开关全部回填', async () => {
    api.getSettings.mockResolvedValue(FULL)
    const wrapper = mountView()
    await flushPromises()

    const byLabel = (text) => {
      const label = wrapper.findAll('label').find((l) => l.text().includes(text))
      expect(label, '找不到 label: ' + text).toBeTruthy()
      return label.find('input, select')
    }

    expect(byLabel('店名').element.value).toBe('楼下奶茶')
    expect(byLabel('小票页脚').element.value).toBe('谢谢惠顾，欢迎再来')
    expect(byLabel('电话').element.value).toBe('13800001111')
    expect(byLabel('脚注').element.value).toBe('谢谢惠顾')
    expect(byLabel('抹零方式').element.value).toBe('jiao')
    expect(byLabel('每消费 1 元得积分').element.value).toBe('1')
    expect(byLabel('多少积分抵 1 元').element.value).toBe('100')
    expect(byLabel('积分有效期').element.value).toBe('12')
    expect(byLabel('会员等级规则').element.value).toBe(FULL.member_tiers)
    expect(byLabel('储值赠送').element.value).toBe('500:50')
    expect(byLabel('后厨打印机名').element.value).toBe('厨房打印机')
    expect(byLabel('出后厨单的类目').element.value).toBe('奶茶,小吃')

    // 国际化区块也新增了一个「价格已含税」勾选框，权限勾选按 .perm-row 限定
    const checks = wrapper.findAll('.perm-row input[type="checkbox"]').map((c) => c.element.checked)
    expect(checks).toEqual([true, true, false, false])

    expect(wrapper.find('.orders-error').exists()).toBe(false)
  })

  it('点保存时提交的是回填后的值，不是默认值', async () => {
    api.getSettings.mockResolvedValue(FULL)
    api.saveSettings.mockResolvedValue(FULL)
    const wrapper = mountView()
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === '保存')
    expect(saveBtn).toBeTruthy()
    await saveBtn.trigger('click')
    await flushPromises()

    const payload = api.saveSettings.mock.calls[0][0]
    expect(payload.shop_name).toBe('楼下奶茶')
    expect(payload.receipt_footer).toBe('谢谢惠顾，欢迎再来')
    expect(payload.round_mode).toBe('jiao')
    expect(payload.kitchen_printer).toBe('厨房打印机')
    expect(payload.perm_refund_cashier).toBe('true')
    expect(payload.perm_void_cashier).toBe('false')
    expect(payload.points_expire_months).toBe('12')
  })

  it('读取失败时给出错误提示且不清空表单', async () => {
    api.getSettings.mockRejectedValue(new Error('db locked'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.orders-error').text()).toContain('读取设置失败')
    const byLabel = (text) => wrapper.findAll('label').find((l) => l.text().includes(text)).find('input, select')
    expect(byLabel('抹零方式').element.value).toBe('none')
  })
})
