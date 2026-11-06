// 两个 adapter 的公共实现：只有 basePath 不同，其余逻辑都在这。
// 依赖 lib/http.js 的 token / 401 逻辑，不在 adapter 里重复实现。
import { request, getToken } from '../lib/http.js'

export function createPosApi(basePath) {
  const url = (path) => basePath + path

  return {
    // 系统
    health() {
      return request(url('/health'))
    },

    // 认证
    login(username, pin) {
      return request(url('/auth/login'), {
        method: 'POST',
        body: JSON.stringify({ username, pin })
      })
    },
    me() {
      return request(url('/auth/me'))
    },
    logout() {
      return request(url('/auth/logout'), { method: 'POST' })
    },

    // 商品
    listProducts(includeInactive) {
      return request(url('/products' + (includeInactive ? '?includeInactive=true' : '')))
    },
    createProduct(body) {
      return request(url('/products'), { method: 'POST', body: JSON.stringify(body) })
    },
    updateProduct(id, body) {
      return request(url('/products/' + id), { method: 'PUT', body: JSON.stringify(body) })
    },
    setProductActive(id, active) {
      return request(url('/products/' + id + '/active'), {
        method: 'POST',
        body: JSON.stringify({ active })
      })
    },
    deleteProduct(id) {
      return request(url('/products/' + id), { method: 'DELETE' })
    },
    importProducts(csvText) {
      return request(url('/products/import'), {
        method: 'POST',
        headers: { 'Content-Type': 'text/csv; charset=utf-8' },
        body: csvText
      })
    },
    async exportProducts() {
      const res = await fetch(url('/products/export'), {
        headers: { Authorization: 'Bearer ' + getToken() }
      })
      if (!res.ok) {
        throw new Error('导出失败：' + res.status)
      }
      return res.text()
    },

    // 员工（设置页，店长权限）
    listStaff() {
      return request(url('/staff'))
    },
    createStaff(body) {
      return request(url('/staff'), { method: 'POST', body: JSON.stringify(body) })
    },
    updateStaff(id, body) {
      return request(url('/staff/' + id), { method: 'PUT', body: JSON.stringify(body) })
    },
    setStaffActive(id, active) {
      return request(url('/staff/' + id + '/active'), {
        method: 'POST',
        body: JSON.stringify({ active })
      })
    },
    deleteStaff(id) {
      return request(url('/staff/' + id), { method: 'DELETE' })
    },

    // 订单
    createOrder(body) {
      return request(url('/orders'), { method: 'POST', body: JSON.stringify(body) })
    },
    getOrder(orderNo) {
      return request(url('/orders/' + encodeURIComponent(orderNo)))
    },
    listOrders(limit) {
      return request(url('/orders?limit=' + (limit || 50)))
    },
    payOrder(orderNo, body) {
      return request(url('/orders/' + encodeURIComponent(orderNo) + '/pay'), {
        method: 'POST',
        body: JSON.stringify(body || {})
      })
    },
    reprint(orderNo) {
      return request(url('/orders/' + encodeURIComponent(orderNo) + '/reprint'), { method: 'POST' })
    },

    // 班次
    latestShift() {
      return request(url('/shifts/latest'))
    },
    openShift(body) {
      return request(url('/shifts/open'), { method: 'POST', body: JSON.stringify(body) })
    },
    closeShift(id, body) {
      return request(url('/shifts/' + id + '/close'), { method: 'POST', body: JSON.stringify(body) })
    },

    // 类目
    listCategories() {
      return request(url('/categories'))
    },
    createCategory(body) {
      return request(url('/categories'), { method: 'POST', body: JSON.stringify(body) })
    },
    updateCategory(id, body) {
      return request(url('/categories/' + id), { method: 'PUT', body: JSON.stringify(body) })
    },
    deleteCategory(id) {
      return request(url('/categories/' + id), { method: 'DELETE' })
    },

    // 会员
    listMembers(keyword) {
      return request(url('/members/list?keyword=' + encodeURIComponent(keyword || '')))
    },
    findMember(phone) {
      return request(url('/members?phone=' + encodeURIComponent(phone)))
    },
    createMember(body) {
      return request(url('/members'), { method: 'POST', body: JSON.stringify(body) })
    },
    topupMember(id, body) {
      return request(url('/members/' + id + '/topup'), { method: 'POST', body: JSON.stringify(body) })
    },
    expiringMembers(days) {
      return request(url('/members/expiring?days=' + (days || 7)))
    },
    monthlyReport(month) {
      return request(url('/reports/monthly?month=' + month))
    },
    setMemberActive(id, active) {
      return request(url('/members/' + id + '/active'), { method: 'POST', body: JSON.stringify({ active }) })
    },
    deleteMember(id) {
      return request(url('/members/' + id), { method: 'DELETE' })
    },
    memberQuote(id, amount) {
      return request(url('/members/' + id + '/quote?amount=' + amount))
    },
    memberTxns(id) {
      return request(url('/members/' + id + '/txns'))
    },

    listTables() {
      return request(url('/tables'))
    },
    createTable(name) {
      return request(url('/tables'), { method: 'POST', body: JSON.stringify({ name }) })
    },
    openTable(id, body) {
      return request(url('/tables/' + id + '/open'), { method: 'POST', body: JSON.stringify(body) })
    },
    mergeTables(id, intoTableId) {
      return request(url('/tables/' + id + '/merge'), { method: 'POST', body: JSON.stringify({ intoTableId }) })
    },
    transferTable(id, toTableId) {
      return request(url('/tables/' + id + '/transfer'), { method: 'POST', body: JSON.stringify({ toTableId }) })
    },
    addTableItems(id, body) {
      return request(url('/tables/' + id + '/items'), { method: 'POST', body: JSON.stringify(body) })
    },
    checkoutTable(id, body) {
      return request(url('/tables/' + id + '/checkout'), { method: 'POST', body: JSON.stringify(body) })
    },
    releaseTable(id) {
      return request(url('/tables/' + id + '/release'), { method: 'POST' })
    },
    deleteTable(id) {
      return request(url('/tables/' + id), { method: 'DELETE' })
    },

    // 后厨档口
    printJobs(limit) {
      return request(url('/print/jobs?limit=' + (limit || 10)))
    },
    testPrinter(target) {
      return request(url('/printers/test'), { method: 'POST', body: JSON.stringify({ target }) })
    },
    listStocktakes(limit) {
      return request(url('/stocktakes?limit=' + (limit || 10)))
    },
    listPurchases(limit) {
      return request(url('/purchases?limit=' + (limit || 20)))
    },
    createPurchase(body) {
      return request(url('/purchases'), { method: 'POST', body: JSON.stringify(body || {}) })
    },
    getPurchase(id) {
      return request(url('/purchases/' + id))
    },
    createStocktake(body) {
      return request(url('/stocktakes'), { method: 'POST', body: JSON.stringify(body || {}) })
    },
    getStocktake(id) {
      return request(url('/stocktakes/' + id))
    },
    setStocktakeItems(id, items) {
      return request(url('/stocktakes/' + id + '/items'), { method: 'PUT', body: JSON.stringify({ items }) })
    },
    closeStocktake(id, operator) {
      return request(url('/stocktakes/' + id + '/close'), { method: 'POST', body: JSON.stringify({ operator: operator || '' }) })
    },
    printLabels(body) {
      return request(url('/print/labels'), { method: 'POST', body: JSON.stringify(body) })
    },
    uploadProductImage(id, file) {
      const fd = new FormData()
      fd.append('file', file)
      return request(url('/products/' + id + '/image'), { method: 'POST', body: fd })
    },
    uploadLogo(file) {
      const fd = new FormData()
      fd.append('file', file)
      return request(url('/settings/logo'), { method: 'POST', body: fd })
    },
    listStock() {
      return request(url('/stock'))
    },
    stockAlerts() {
      return request(url('/stock/alerts'))
    },
    setStock(productId, body) {
      return request(url('/stock/' + productId), { method: 'PUT', body: JSON.stringify(body) })
    },
    adjustStock(productId, body) {
      return request(url('/stock/' + productId + '/adjust'), { method: 'POST', body: JSON.stringify(body) })
    },
    listStations() {
      return request(url('/kitchen/stations'))
    },
    createStation(body) {
      return request(url('/kitchen/stations'), { method: 'POST', body: JSON.stringify(body) })
    },
    deleteStation(id) {
      return request(url('/kitchen/stations/' + id), { method: 'DELETE' })
    },
    // 打印队列最近 10 条（设置页只读）
    listPrintJobs() {
      return request(url('/print/jobs'))
    },

    // 报表
    dailyReport(date) {
      return request(url('/reports/daily?date=' + date))
    },

    // 审计（店长）
    auditLogs(limit, offset) {
      return request(url('/audit?limit=' + (limit || 50) + '&offset=' + (offset || 0)))
    },

    // 门店设置（店名/电话/脚注）
    getSettings() {
      return request(url('/settings'))
    },
    saveSettings(body) {
      return request(url('/settings'), { method: 'PUT', body: JSON.stringify(body) })
    },
    voidOrder(orderNo, reason) {
      return request(url('/orders/' + encodeURIComponent(orderNo) + '/void'), {
        method: 'POST',
        body: JSON.stringify({ reason: reason || '' })
      })
    },
    refundOrder(orderNo, reason, items) {
      return request(url('/orders/' + encodeURIComponent(orderNo) + '/refund'), {
        method: 'POST',
        body: JSON.stringify({ reason: reason || '', items: items || [] })
      })
    }
  }
}
