// ============================================================
// POS 收银契约（类型定义，不写实现）
// 依据：docs/api/pos-cashier-api.yaml（v1.0.0）
// UI 只依赖这里的方法签名与返回结构，字段名一个都不能改。
// ============================================================

/**
 * 操作员
 * @typedef {object} Staff
 * @property {number} id 操作员 ID
 * @property {string} name 姓名
 * @property {'manager'|'cashier'} role 角色
 */

/**
 * 商品（在售）
 * @typedef {object} Product
 * @property {number} id 商品 ID
 * @property {string} name 名称
 * @property {string|null} barcode 条码
 * @property {string} category 类目
 * @property {number} price 售价（decimal(10,2)）
 * @property {boolean} hot 热销标记
 * @property {boolean} active 在售标记
 */

/**
 * 下单明细项
 * @typedef {object} OrderItemInput
 * @property {string} productId 商品 ID（字符串）
 * @property {number} qty 数量（>=1）
 */

/**
 * 下单请求体
 * @typedef {object} OrderCreate
 * @property {string} [idempotencyKey] 幂等键
 * @property {number} cash 实收现金
 * @property {number} [discount] 优惠（默认 0）
 * @property {OrderItemInput[]} items 明细（至少 1 项）
 */

/**
 * 订单明细项
 * @typedef {object} OrderItem
 * @property {string} productId
 * @property {string} name
 * @property {number} price
 * @property {number} qty
 * @property {number} amount
 */

/**
 * 订单
 * @typedef {object} Order
 * @property {string} orderNo 单号
 * @property {string} bizDate 营业日期
 * @property {'paid'|'refunded'|'void'} status
 * @property {number} total 合计
 * @property {number} discount 优惠
 * @property {number} payable 应收
 * @property {number} cash 实收
 * @property {number} change 找零
 * @property {0|1|2|3} printStatus 0 未打印 / 1 已入队 / 2 已打印 / 3 待重打
 * @property {string} createdAt
 * @property {OrderItem[]} items
 */

/**
 * 班次
 * @typedef {object} Shift
 * @property {number} id
 * @property {string} shiftNo
 * @property {'open'|'closed'} status
 * @property {string} staffName
 * @property {number} openingCash 期初现金
 * @property {number|null} expectedCash 现金应收
 * @property {number|null} actualCash 现金实收
 * @property {number|null} difference 差异
 * @property {string} openedAt
 * @property {string|null} closedAt
 * @property {ShiftSummary} summary
 */

/**
 * 班次汇总
 * @typedef {object} ShiftSummary
 * @property {number} orderCount 订单笔数
 * @property {number} total 应收合计
 * @property {number} cashNet 现金净额
 * @property {ShiftPayment[]} payments 收款方式汇总
 */

/**
 * 收款方式汇总
 * @typedef {object} ShiftPayment
 * @property {string} method 收款方式
 * @property {number} amount 金额
 */

/**
 * 健康检查
 * @typedef {object} Health
 * @property {string} status
 * @property {'standalone'|'cloud'} mode
 * @property {{queued: number}} queue 待打印队列
 * @property {number} ordersToday 今日订单数
 * @property {{reachable: boolean}} agent 出票助手可达
 */

/**
 * 登录结果
 * @typedef {object} LoginResult
 * @property {string} token
 * @property {Staff} staff
 */

/**
 * 商品写入请求体（商品管理页）
 * @typedef {object} ProductWrite
 * @property {string} name 名称
 * @property {string} [barcode] 条码
 * @property {string} [category] 类目
 * @property {number} price 售价
 * @property {number} [cost] 进价
 * @property {number} [sort] 排序
 * @property {boolean} [hot] 热销推荐
 */

/**
 * 收银台 API 契约（UI 唯一入口）
 * @typedef {object} PosApi
 * @property {() => Promise<Health>} health 健康检查（免鉴权）
 * @property {(username: string, pin: string) => Promise<LoginResult>} login 登录
 * @property {() => Promise<Staff>} me 当前操作员
 * @property {() => Promise<void>} logout 登出
 * @property {(includeInactive?: boolean) => Promise<Product[]>} listProducts 商品列表
 * @property {(body: ProductWrite) => Promise<Product>} createProduct 新建商品
 * @property {(id: number, body: ProductWrite) => Promise<Product>} updateProduct 更新商品
 * @property {(id: number, active: boolean) => Promise<Product>} setProductActive 上架/下架
 * @property {(id: number) => Promise<void>} deleteProduct 删除商品
 * @property {(body: OrderCreate) => Promise<Order>} createOrder 下单（金额由服务端重算）
 * @property {(orderNo: string) => Promise<Order>} getOrder 订单详情（含打印状态）
 * @property {(limit?: number) => Promise<Order[]>} listOrders 订单列表
 * @property {(orderNo: string) => Promise<void>} reprint 重打小票
 * @property {() => Promise<Shift|null>} latestShift 最近班次（从未开班 → null）
 * @property {(body: {openingCash: number, staffName?: string}) => Promise<Shift>} openShift 开班
 * @property {(id: number, body: {actualCash?: number, note?: string}) => Promise<Shift>} closeShift 结班（店长）
 */

export {}
