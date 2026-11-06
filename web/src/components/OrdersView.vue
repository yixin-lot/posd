<template>
  <section class="orders">
    <header class="orders-head">
      <h2>订单</h2>
      <div class="filters">
        <button :class="{ active: filter === 'all' }" @click="filter = 'all'">全部 {{ counts.all }}</button>
        <button :class="{ active: filter === 'held' }" @click="filter = 'held'">挂单 {{ counts.held }}</button>
        <button :class="{ active: filter === 'refunded' }" @click="filter = 'refunded'">已退款 {{ counts.refunded }}</button>
        <button :class="{ active: filter === 'void' }" @click="filter = 'void'">已作废 {{ counts.void }}</button>
        <button :class="{ active: filter === 'reprint' }" @click="filter = 'reprint'">
          待重打 {{ counts.reprint }}
        </button>
      </div>
      <span class="grow"></span>
      <button @click="load" :disabled="loading">{{ loading ? '刷新中…' : '刷新' }}</button>
    </header>

    <div class="filters-row">
      <label class="f-cell">日期
        <select v-model="dateFilter" @change="onFilterChange">
          <option value="">全部</option>
          <option value="today">今天</option>
          <option value="yesterday">昨天</option>
          <option value="7d">近 7 天</option>
          <option value="custom">自定义</option>
        </select>
      </label>
      <template v-if="dateFilter === 'custom'">
        <label class="f-cell">从
          <input type="date" v-model="customFrom" @change="onFilterChange" />
        </label>
        <label class="f-cell">到
          <input type="date" v-model="customTo" @change="onFilterChange" />
        </label>
      </template>
      <label class="f-cell">桌台
        <select v-model="tableId" @change="onFilterChange">
          <option value="">全部</option>
          <option v-for="t in tables" :key="t.id" :value="String(t.id)">
            {{ t.name }}{{ t.status === 'occupied' ? '（占用）' : '' }}
          </option>
        </select>
      </label>
    </div>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <p v-if="heldOverCount" class="orders-warn">
      有 {{ heldOverCount }} 张挂单超过 {{ holdExpireMin }} 分钟，未结算将自动取消（桌台单不受影响）。
    </p>

    <table class="orders-table">
      <thead>
        <tr>
          <th style="width: 160px">时间</th>
          <th style="width: 170px">单号</th>
          <th>商品</th>
          <th style="width: 90px" class="num">金额</th>
          <th style="width: 100px">打印</th>
          <th style="width: 130px"></th>
        </tr>
      </thead>
      <tbody>
        <template v-for="o in filtered" :key="o.orderNo">
          <tr
            :class="{ selected: selected === o.orderNo, 'held-over': isHeldOver(o) }"
            @click="selected = selected === o.orderNo ? '' : o.orderNo"
          >
            <td>
              <div>{{ o.createdAt }}</div>
              <div v-if="o.status === 'held'" class="held-duration">{{ heldDuration(o) }}</div>
            </td>
            <td class="mono">{{ o.orderNo }}</td>
            <td>{{ summary(o) }}</td>
            <td class="num">¥{{ o.total.toFixed(2) }}</td>
            <td>
              <span class="chip" :class="chipClass(o.printStatus)">{{ statusText(o.printStatus) }}</span>
              <span v-if="o.status === 'held'" class="chip chip-idle" style="margin-left: 6px">挂单</span>
              <span v-if="o.status === 'cancelled'" class="chip chip-idle" style="margin-left: 6px">已取消</span>
              <span v-if="o.status === 'refunded'" class="chip chip-bad" style="margin-left: 6px">已退款</span>
              <span v-if="o.status === 'void'" class="chip chip-idle" style="margin-left: 6px">已作废</span>
            </td>
            <td class="ops" @click.stop>
              <button @click="selected = o.orderNo">明细</button>
              <button v-if="o.status !== 'held' && o.status !== 'cancelled'" :disabled="o.printStatus === 1" @click="reprint(o)">重打</button>
              <button v-if="o.status === 'paid'" @click="startRefund(o)">退款</button>
              <button v-if="o.status === 'paid' && !o.refundAmount" class="danger" @click="startVoid(o)">作废</button>
            </td>
          </tr>
          <tr v-if="selected === o.orderNo" class="detail-row">
            <td colspan="6">
              <div class="detail">
                <div class="detail-items">
                  <div v-for="it in o.items" :key="it.productId" class="detail-item">
                    <span>{{ it.name }} × {{ it.qty }}</span>
                    <span class="num">¥{{ it.amount.toFixed(2) }}</span>
                  </div>
                </div>
                <div class="detail-sum">
                  <span>合计 ¥{{ o.total.toFixed(2) }}</span>
                  <span>现金 ¥{{ o.cash.toFixed(2) }}</span>
                  <span>找零 ¥{{ o.change.toFixed(2) }}</span>
                </div>
                <p v-if="o.status === 'held'" class="muted" style="margin: 10px 0 0">
                  挂单未收款：到收银台点上方「挂单」条，可继续结算这笔单。
                </p>
                <p v-else-if="o.status === 'cancelled'" class="muted" style="margin: 10px 0 0">
                  已取消：{{ o.voidReason || '挂单超时未结算，自动取消' }}。
                </p>
              </div>
            </td>
          </tr>
        </template>
        <tr v-if="!filtered.length">
          <td colspan="6" class="empty-cell">
            <EmptyState
              v-if="!loading && !list.length"
              title="还没有订单"
              desc="完成第一笔收银后，订单会出现在这里：回到收银台点商品、结算出票即可。"
              :actions="[]"
            />
            <p v-else class="empty-row">{{ loading ? '加载中…' : emptyText[filter] }}</p>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="refundTarget" class="mask" @click.self="refundTarget = null">
      <div class="modal product-modal">
        <h3>退款 · {{ refundTarget.orderNo }}</h3>
        <p class="muted" style="margin: 0">
          可整单退，也可按件退；现金原路退现金，会员余额支付的金额会退回会员余额，其余退回现金。剩余可退 ¥{{ (refundTarget.payable - refundTarget.refundAmount).toFixed(2) }}。
        </p>
        <div class="refund-lines">
          <div v-for="it in refundTarget.items" :key="it.productId" class="refund-line">
            <span>{{ it.name }}（¥{{ it.price.toFixed(2) }} × {{ it.qty }}，已退 {{ it.refundedQty || 0 }}）</span>
            <input
              type="number"
              min="0"
              :max="it.qty - (it.refundedQty || 0)"
              v-model.number="refundQty[it.productId]"
            />
          </div>
        </div>
        <div class="product-form">
          <label style="grid-column: 1 / -1">原因（可选）
            <input v-model="refundReason" maxlength="40" placeholder="例如：顾客取消" />
          </label>
        </div>
        <p style="margin: 0; font-weight: 700">本次退款：¥{{ refundPreview.toFixed(2) }}</p>
        <div class="modal-actions">
          <button @click="refundTarget = null">取消</button>
          <button class="primary" :disabled="refunding" @click="doRefund">
            {{ refunding ? '退款中…' : '确认退款' }}
          </button>
        </div>
        <p v-if="refundError" class="orders-error">{{ refundError }}</p>
      </div>
    </div>

    <div v-if="voidTarget" class="mask" @click.self="voidTarget = null">
      <div class="modal product-modal">
        <h3>作废 · {{ voidTarget.orderNo }}</h3>
        <p class="muted" style="margin: 0">
          整单作废：¥{{ voidTarget.payable.toFixed(2) }} 原路退回，<b>不计入销售额</b>。
          已退过款的订单请用「退款」，不要作废。
        </p>
        <div class="product-form">
          <label style="grid-column: 1 / -1">原因
            <input v-model="voidReason" maxlength="40" placeholder="例如：点错单 / 重复下单" />
          </label>
        </div>
        <div class="modal-actions">
          <button @click="voidTarget = null">取消</button>
          <button class="danger" :disabled="voiding" @click="doVoid">
            {{ voiding ? '作废中…' : '确认作废' }}
          </button>
        </div>
        <p v-if="voidError" class="orders-error">{{ voidError }}</p>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import EmptyState from './EmptyState.vue'

const list = ref([])
const loading = ref(false)
const error = ref('')
const filter = ref('all')
const selected = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
const refundTarget = ref(null)
const refundReason = ref('')
const refundQty = ref({})
const refunding = ref(false)
const refundError = ref('')
const voidTarget = ref(null)
const voidReason = ref('')
const voiding = ref(false)
const voidError = ref('')

const dateFilter = ref('')
const customFrom = ref('')
const customTo = ref('')
const tableId = ref('')
const tables = ref([])

// T2 挂单超时：由设置 hold_expire_minutes 决定（0=不自动取消）。
// 规则：只对外带挂单生效，**桌台挂单永不自动取消**（客人还在吃）。
// 时长优先用后端 heldAt（接口已暴露），缺失时回退 createdAt。
const holdExpireMin = ref(30)

const refundPreview = computed(() => {
  if (!refundTarget.value) return 0
  let sum = 0
  for (const it of refundTarget.value.items) {
    const q = Math.max(0, Math.min(Number(refundQty.value[it.productId] || 0), it.qty - (it.refundedQty || 0)))
    sum += q * it.price
  }
  return Math.round(sum * 100) / 100
})
let toastTimer = null

const counts = computed(() => ({
  all: list.value.length,
  held: list.value.filter((o) => o.status === 'held').length,
  refunded: list.value.filter((o) => o.status === 'refunded').length,
  void: list.value.filter((o) => o.status === 'void').length,
  reprint: list.value.filter((o) => o.printStatus === 3).length
}))

const filterMap = {
  all: (l) => l,
  held: (l) => l.filter((o) => o.status === 'held'),
  refunded: (l) => l.filter((o) => o.status === 'refunded'),
  void: (l) => l.filter((o) => o.status === 'void'),
  reprint: (l) => l.filter((o) => o.printStatus === 3)
}
const emptyText = {
  held: '没有挂单',
  refunded: '没有已退款的订单',
  void: '没有已作废的订单',
  reprint: '没有待重打的订单'
}
const filtered = computed(() => {
  let l = (filterMap[filter.value] || filterMap.all)(list.value)
  l = l.filter(inDateRange)
  l = l.filter(inTable)
  return l
})

const heldOverCount = computed(() => filtered.value.filter(isHeldOver).length)

// —— 日期筛选 ——
function dayKey(s) {
  return (s || '').slice(0, 10)
}
function todayKey() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
function inDateRange(o) {
  if (!dateFilter.value) return true
  const k = dayKey(o.createdAt)
  if (!k) return false
  const today = todayKey()
  if (dateFilter.value === 'today') return k === today
  if (dateFilter.value === 'yesterday') {
    const y = new Date()
    y.setDate(y.getDate() - 1)
    return k === dayKey(y.toLocaleDateString('en-CA'))
  }
  if (dateFilter.value === '7d') {
    const from = new Date()
    from.setDate(from.getDate() - 6)
    return k >= dayKey(from.toLocaleDateString('en-CA')) && k <= today
  }
  if (dateFilter.value === 'custom') {
    if (customFrom.value && k < customFrom.value) return false
    if (customTo.value && k > customTo.value) return false
    return true
  }
  return true
}

// —— 桌台筛选 ——
// 订单接口不返回桌台；桌台占用期持有 orderNo，按下拉所选桌台的 orderNo 匹配。
function inTable(o) {
  if (!tableId.value) return true
  const t = tables.value.find((x) => String(x.id) === tableId.value)
  return !!t && !!t.orderNo && t.orderNo === o.orderNo
}

// —— 挂单时长 ——
function heldMs(o) {
  if (o.status !== 'held') return 0
  const t = Date.parse((o.heldAt || o.createdAt).replace(' ', 'T')) // 本地时间字符串 → 本地毫秒
  if (Number.isNaN(t)) return 0
  return Math.max(0, Date.now() - t)
}
function heldDuration(o) {
  const ms = heldMs(o)
  if (ms <= 0) return '已挂不足 1 分钟'
  const h = Math.floor(ms / 3600000)
  const m = Math.floor((ms % 3600000) / 60000)
  if (h === 0) return `已挂 ${m} 分钟`
  return `已挂 ${h} 小时 ${m} 分`
}
function isHeldOver(o) {
  // 桌台单不参与超时提醒/自动取消
  if (o.status !== 'held' || o.tableName) return false
  if (holdExpireMin.value <= 0) return false
  return heldMs(o) > holdExpireMin.value * 60 * 1000
}

// —— URL query 持久化（history.replaceState，不引路由库）——
function syncQuery() {
  const q = new URLSearchParams()
  if (dateFilter.value) q.set('date', dateFilter.value)
  if (customFrom.value) q.set('from', customFrom.value)
  if (customTo.value) q.set('to', customTo.value)
  if (tableId.value) q.set('table', tableId.value)
  const s = q.toString()
  history.replaceState(null, '', s ? '?' + s : location.pathname)
}
function readQuery() {
  const q = new URLSearchParams(location.search)
  dateFilter.value = q.get('date') || ''
  customFrom.value = q.get('from') || ''
  customTo.value = q.get('to') || ''
  tableId.value = q.get('table') || ''
}
function onFilterChange() {
  if (dateFilter.value !== 'custom') {
    customFrom.value = ''
    customTo.value = ''
  }
  syncQuery()
}

function statusText(s) {
  return { 0: '未打印', 1: '出票中', 2: '已打印', 3: '待重打' }[s] || '未知'
}

function chipClass(s) {
  return { 0: 'chip-idle', 1: 'chip-busy', 2: 'chip-ok', 3: 'chip-bad' }[s] || ''
}

function summary(o) {
  if (!o.items || !o.items.length) return '—'
  const first = o.items[0].name
  return o.items.length === 1 ? first : first + ' 等 ' + o.items.length + ' 种'
}

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toastMsg.value = ''), 2400)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    list.value = await api.listOrders(50)
  } catch (e) {
    error.value = '加载订单失败：' + (e.message || '未知错误')
  } finally {
    loading.value = false
  }
}

async function loadTables() {
  try {
    tables.value = await api.listTables()
  } catch (e) {
    // 桌台列表拉不到不阻塞订单页（筛选下拉保持空）
    tables.value = []
  }
}

async function reprint(o) {
  try {
    await api.reprint(o.orderNo)
    toast('已重新提交出票，等待打印机')
    setTimeout(load, 800)
  } catch (e) {
    toast('重打失败：' + (e.message || '未知错误'), true)
  }
}

function startVoid(o) {
  voidTarget.value = o
  voidReason.value = ''
  voidError.value = ''
}

async function doVoid() {
  if (!voidTarget.value) return
  voiding.value = true
  voidError.value = ''
  try {
    await api.voidOrder(voidTarget.value.orderNo, voidReason.value)
    voidTarget.value = null
    toast('已作废')
    await load()
  } catch (e) {
    voidError.value = e.message || '作废失败'
  } finally {
    voiding.value = false
  }
}

function startRefund(o) {
  refundTarget.value = o
  refundReason.value = ''
  refundError.value = ''
  const qty = {}
  for (const it of o.items) {
    qty[it.productId] = it.qty - (it.refundedQty || 0)
  }
  refundQty.value = qty
}

async function doRefund() {
  if (!refundTarget.value) return
  refunding.value = true
  refundError.value = ''
  try {
    const items = Object.entries(refundQty.value)
      .map(([productId, qty]) => ({ productId, qty: Number(qty) || 0 }))
      .filter((it) => it.qty > 0)
    await api.refundOrder(refundTarget.value.orderNo, refundReason.value, items)
    refundTarget.value = null
    toast('已退款')
    await load()
  } catch (e) {
    refundError.value = e.message || '退款失败'
  } finally {
    refunding.value = false
  }
}

onMounted(() => {
  readQuery()
  load()
  loadTables()
})
</script>
