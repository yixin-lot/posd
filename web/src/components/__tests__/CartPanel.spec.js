// 金额与抹零的纯计算抽在 src/lib/money.js，与 App.vue 用的是同一份实现。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CartPanel from '../CartPanel.vue'
import { cartTotal, roundPayable } from '../../lib/money.js'

const ITEMS = [
  { id: 1, name: '珍珠奶茶', price: 13.87, listPrice: 13.87, qty: 3 },
  { id: 2, name: '椰果奶茶', price: 12.5, listPrice: 12.5, qty: 1 }
]

function mountCart(items) {
  return mount(CartPanel, { props: { items } })
}

describe('CartPanel 交互', () => {
  it('加购后渲染行、加减删按钮 emit 带对应行；清空后回到空态', async () => {
    const items = JSON.parse(JSON.stringify(ITEMS))
    const wrapper = mountCart(items)

    expect(wrapper.findAll('.cart-row')).toHaveLength(2)
    // v03 收银台购物车按「件」计数（原「行」为旧文案）
    expect(wrapper.find('.hint').text()).toBe('2 件')
    expect(wrapper.text()).toContain('¥13.87 × 3')

    const firstRow = wrapper.findAll('.cart-row')[0]
    const btns = firstRow.findAll('.qty button')
    expect(btns).toHaveLength(2) // − +

    await btns[0].trigger('click')
    expect(wrapper.emitted('dec')[0][0]).toEqual(items[0])

    await btns[1].trigger('click')
    expect(wrapper.emitted('inc')[0][0]).toEqual(items[0])

    // v03：改价是行内铅笔图标，移除是行右侧 ×
    await firstRow.find('.repricing-ico').trigger('click')
    expect(wrapper.emitted('reprice')[0][0]).toEqual(items[0])

    await firstRow.find('.row-del').trigger('click')
    expect(wrapper.emitted('remove')[0][0]).toEqual(items[0])

    // v03 P2：清空在底部四键最右（挂单/取单/折扣/清空）
    await wrapper.findAll('.cart-foot button')[3].trigger('click')
    expect(wrapper.emitted('clear')).toHaveLength(1)

    const empty = mountCart([])
    expect(empty.find('.empty-title').text()).toBe('购物车是空的')
    expect(empty.findAll('.cart-foot button').every((b) => b.attributes('disabled') !== undefined)).toBe(true)
  })
})

describe('CartPanel 金额与应付', () => {
  it('合计 54.11 元：不抹零 / 抹分 / 抹角 三个分支都对得上', () => {
    const total = cartTotal(ITEMS)
    expect(total).toBeCloseTo(54.11, 2)

    // 不抹零
    expect(roundPayable(total, 'none')).toBe(54.11)
    // 抹分：13.87 这类分位不整的会被抹到角
    expect(roundPayable(total, 'jiao')).toBe(54.1)
    // 抹角：抹到元
    expect(roundPayable(total, 'yuan')).toBe(54)
  })

  it('加减删之后金额与应付同步变化（抹分分支）', () => {
    const afterInc = ITEMS.map((it, i) => (i === 0 ? { ...it, qty: it.qty + 1 } : it))
    expect(cartTotal(afterInc)).toBeCloseTo(67.98, 2)
    expect(roundPayable(cartTotal(afterInc), 'jiao')).toBe(67.9)

    const afterDec = ITEMS.map((it, i) => (i === 0 ? { ...it, qty: it.qty - 1 } : it))
    expect(cartTotal(afterDec)).toBeCloseTo(40.24, 2)
    expect(roundPayable(cartTotal(afterDec), 'jiao')).toBe(40.2)

    const afterRemove = [ITEMS[0]]
    expect(cartTotal(afterRemove)).toBeCloseTo(41.61, 2)
    expect(roundPayable(cartTotal(afterRemove), 'yuan')).toBe(41)
  })

  it('改价后按改后单价计，并展示原价标记', () => {
    const repriced = [{ id: 1, name: '珍珠奶茶', price: 10, listPrice: 13.87, qty: 2 }]
    const wrapper = mountCart(repriced)
    expect(wrapper.find('.reprice-tag').text()).toContain('原价 ¥13.87')
    expect(cartTotal(repriced)).toBe(20)
    expect(roundPayable(cartTotal(repriced), 'jiao')).toBe(20)
  })
})
