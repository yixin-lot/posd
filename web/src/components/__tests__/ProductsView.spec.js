// 商品页：列表渲染 + 盘点弹窗的关键行为（差异计算、关单调用）。
// 目的是给"重建库存"这类高风险操作留一条回归线，不必每次都开浏览器。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// vi.mock 会被提升到文件顶部，mock 里引用的变量必须用 vi.hoisted 一起提升
const { api } = vi.hoisted(() => ({
  api: {
    listProducts: vi.fn(),
    listStock: vi.fn(),
    stockAlerts: vi.fn(),
    listStocktakes: vi.fn(),
    createStocktake: vi.fn(),
    setStocktakeItems: vi.fn(),
    closeStocktake: vi.fn(),
    setStock: vi.fn(),
    adjustStock: vi.fn()
  }
}))
vi.mock('../../api/index.js', () => ({ api }))

import ProductsView from '../ProductsView.vue'

const PRODUCTS = [
  { id: '1', name: '珍珠奶茶', category: '奶茶', price: 13, kind: 'single', active: true },
  { id: '2', name: '蛋挞', category: '甜点', price: 6, kind: 'single', active: true }
]

function mountView() {
  return mount(ProductsView, { global: { stubs: { ImportDialog: true, CategoryDialog: true } } })
}

describe('ProductsView 盘点', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listProducts.mockResolvedValue(PRODUCTS)
    api.listStock.mockResolvedValue([])
    api.stockAlerts.mockResolvedValue([])
    api.listStocktakes.mockResolvedValue([])
    api.createStocktake.mockResolvedValue({
      id: 7,
      status: 'open',
      items: [
        { productId: 1, name: '珍珠奶茶', systemQty: 10, actualQty: null, diff: 0 },
        { productId: 2, name: '蛋挞', systemQty: 4, actualQty: null, diff: 0 }
      ]
    })
    api.setStocktakeItems.mockImplementation(async (id, items) => ({
      id,
      status: 'open',
      items: items.map((it) => ({
        productId: it.productId,
        name: 'x',
        systemQty: it.productId === 1 ? 10 : 4,
        actualQty: it.actualQty,
        diff: it.actualQty - (it.productId === 1 ? 10 : 4)
      }))
    }))
    api.closeStocktake.mockResolvedValue({ id: 7, status: 'done', items: [] })
  })

  it('列出商品；没建库存行时显示「未跟踪」', async () => {
    const w = mountView()
    await flushPromises()
    const rows = w.findAll('.orders-table tbody tr')
    expect(rows.length).toBe(2)
    expect(w.text()).toContain('珍珠奶茶')
    expect(w.text()).toContain('未跟踪')
  })

  it('录入实盘后差异实时显示，关单前会先保存实盘', async () => {
    const w = mountView()
    await flushPromises()

    // D16 起「盘点」是个菜单：先点入口，再点「开始新盘点」
    await w.findAll('.orders-head button').find((b) => b.text().includes('盘点')).trigger('click')
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('开始新盘点')).trigger('click')
    await flushPromises()

    expect(w.text()).toContain('库存盘点 #7')
    const inputs = w.findAll('.modal input[type=number]')
    expect(inputs.length).toBe(2)

    // 账面 10，实盘 7 → 差异 -3
    await inputs[0].setValue(7)
    expect(w.find('.modal tbody tr').text()).toContain('-3')

    const closeBtn = w.findAll('.modal-actions button').find((b) => b.text().includes('关单'))
    await closeBtn.trigger('click')
    await flushPromises()

    expect(api.setStocktakeItems).toHaveBeenCalledWith(7, [{ productId: 1, actualQty: 7 }])
    expect(api.closeStocktake).toHaveBeenCalledWith(7, '店长')
  })

  it('已关单的盘点不让改：按钮禁用、输入只读', async () => {
    api.createStocktake.mockResolvedValue({
      id: 8,
      status: 'done',
      items: [{ productId: 1, name: '珍珠奶茶', systemQty: 10, actualQty: 8, diff: -2 }]
    })
    const w = mountView()
    await flushPromises()
    await w.findAll('.orders-head button').find((b) => b.text().includes('盘点')).trigger('click')
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('开始新盘点')).trigger('click')
    await flushPromises()

    const input = w.find('.modal input[type=number]')
    expect(input.attributes('disabled')).toBeDefined()
    const closeBtn = w.findAll('.modal-actions button').find((b) => b.text().includes('关单'))
    expect(closeBtn.attributes('disabled')).toBeDefined()
  })
})
