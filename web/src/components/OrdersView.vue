<template>
  <section class="orders">
    <header class="orders-head">
      <h2>订单查询</h2>
      <span class="grow"></span>
      <button class="btn-line sm" disabled title="导出报表需在报表页操作">导出报表</button>
    </header>

    <div class="filters-row">
      <label class="f-pill">
        <select v-model="dateFilter" @change="onFilterChange">
          <option value="">全部日期</option>
          <option value="today">今日</option>
          <option value="yesterday">昨日</option>
          <option value="7d">近 7 天</option>
          <option value="custom">自定义</option>
        </select>
      </label>
      <template v-if="dateFilter === 'custom'">
        <label class="f-pill f-date">
          <input type="date" v-model="customFrom" @change="onFilterChange" />
        </label>
        <label class="f-pill f-date">
          <input type="date" v-model="customTo" @change="onFilterChange" />
        </label>
      </template>
      <label class="f-pill">
        <select v-model="payFilter">
          <option value="">支付方式 · 全部</option>
          <option value="cash">现金</option>
          <option value="wechat">扫码</option>
          <option value="alipay">被扫</option>
          <option value="member">会员余额</option>
          <option value="split">组合</option>
        </select>
      </label>
      <label class="f-pill">
        <select v-model="cashierFilter">
          <option value="">收银员 · 全部</option>
          <option v-for="c in cashiers" :key="c" :value="c">{{ c }}</option>
        </select>
      </label>
      <input class="f-search" v-model="kw" placeholder="搜索单号 / 商品 / 金额" />
      <span class="grow"></span>
      <span class="filters-meta">共 {{ filtered.length }} 笔 · 今日 ¥{{ todayTotal.toFixed(2) }}</span>
    </div>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <p v-if="heldOverCount" class="orders-warn">
      有 {{ heldOverCount }} 张挂单超过 {{ holdExpireMin }} 分钟，未结算将自动取消（桌台单不受影响）。
    </p>

    <table class="orders-table tbl">
      <colgroup>
        <col style="width: 11%" />
        <col style="width: 15%" />
        <col style="width: 28%" />
        <col style="width: 10%" />
        <col style="width: 9%" />
        <col style="width: 10%" />
        <col style="width: 9%" />
        <col style="width: 9%" />
      </colgroup>
      <thead>
        <tr>
          <th>时间</th>
          <th>单号</th>
          <th>商品</th>
          <th class="num">金额</th>
          <th>支付</th>
          <th>收银员</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="o in filtered" :key="o.orderNo">
          <tr
            :class="{ selected: selected === o.orderNo, 'held-over': isHeldOver(o) }"
            @click="selected = selected === o.orderNo ? '' : o.orderNo"
          >
            <td class="time-cell">
              <div :title="o.createdAt">{{ shortTime(o.createdAt) }}</div>
              <div v-if="o.status === 'held'" class="held-duration">{{ heldDuration(o) }}</div>
            </td>
            <td class="mono">{{ o.orderNo }}</td>
            <td>{{ summary(o) }}</td>
            <td class="num">¥{{ o.total.toFixed(2) }}</td>
            <td>{{ methodText(o) }}</td>
            <td>{{ cashierOf(o) }}</td>
            <td><span class="tag" :class="statusTag(o).cls">{{ statusTag(o).text }}</span></td>
            <td class="ops ops-links" @click.stop>
              <button
                v-if="o.status !== 'held' && o.status !== 'cancelled'"
                class="link-brand"
                :disabled="o.printStatus === 1"
                @click="reprint(o)"
              >重打</button>
              <button v-if="o.status === 'paid'" class="link-danger" @click="startRefund(o)">退款</button>
            </td>
          </tr>
          <tr v-if="selected === o.orderNo" class="detail-row">
            <td colspan="8">
              <div class="detail">
                <div class="detail-head">
                  明细 · 单号 {{ o.orderNo }} · {{ methodText(o) }} · {{ cashierOf(o) }} · 打印：{{ statusText(o.printStatus) }}
                </div>
                <div class="detail-items">
                  <div v-for="it in o.items" :key="it.productId" class="detail-item">
                    <span>{{ it.name }} ¥{{ it.price.toFixed(2) }} × {{ it.qty }} = ¥{{ it.amount.toFixed(2) }}</span>
                    <span class="num" v-if="it.refundedQty">已退 {{ it.refundedQty }}</span>
                  </div>
                </div>
                <div class="detail-sum">
                  <span>合计 ¥{{ o.total.toFixed(2) }}</span>
                  <span>实收 ¥{{ o.cash.toFixed(2) }}</span>
                  <span>找零 ¥{{ o.change.toFixed(2) }}</span>
                  <button
                    v-if="o.status === 'paid' && !o.refundAmount"
                    class="link-danger"
                    @click.stop="startVoid(o)"
                  >作废整单</button>
                </div>
                <p v-if="o.status === 'held'" class="muted" style="margin: 10px 0 0">
                  挂单未收款：到收银台点「取单 F5」，可继续结算这笔单。
                </p>
                <p v-else-if="o.status === 'cancelled'" class="muted" style="margin: 10px 0 0">
                  已取消：{{ o.voidReason || '挂单超时未结算，自动取消' }}。
                </p>
              </div>
            </td>
          </tr>
        </template>
        <tr v-if="!filtered.length">
          <td colspan="8" class="empty-cell">
            <EmptyState
              v-if="!loading && !list.length"
              title="还没有订单"
              desc="完成第一笔收银后，订单会出现在这里：回到收银台点商品、结算出票即可。"
              :actions="[]"
            />
            <p v-else class="empty-row">{{ loading ? '加载中…' : '没有匹配的订单' }}</p>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="orders-foot">
      <span>500 单内 1 秒出结果，默认仅今 · 共 {{ filtered.length }} 笔</span>
      <div class="pager">
        <button disabled>上一页</button>
        <button class="on" disabled>1</button>
        <button disabled>下一页</button>
      </div>
    </div>

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

// v03 P4 稿面只有三个筛选胶囊 + 搜索：日期 / 支付方式 / 收银员 / 搜索。
// 状态筛选已按稿件统一移除（挂单在收银台「取单」进入，退款/作废在行内操作）。
const payFilter = ref('')
// 状态筛选：v03 稿面没画，但挂单/退款/作废是日常核对场景，日期+支付+收银员替代不了
const statusFilter = ref('')
const cashierFilter = ref('')
const kw = ref('')

// v03 P4：时间列单行「11-05 07:36」，完整时间放 title
// v03 P4：时间列当日只显示 HH:MM，跨日才带日期（列宽足够单行）
function shortTime(s) {
  const m = String(s || '').match(/(\d{1,2})-(\d{1,2})[ T](\d{2}:\d{2})/)
  const now = new Date()
  const todayMd = `${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
  if (m) {
    const md = `${m[1].padStart(2, '0')}-${m[2].padStart(2, '0')}`
    return md === todayMd ? m[3] : `${md} ${m[3]}`
  }
  const t = String(s || '').match(/(\d{2}:\d{2})/)
  return t ? t[1] : s || '—'
}

// 订单接口不一定带支付/收银员字段，缺失时该条不参与这两个筛选（不误伤空表）
function normMethod(o) {
  if (o.payments && o.payments.length) {
    return o.payments.length > 1 ? 'split' : o.payments[0].method || ''
  }
  if (o.payMethod || o.method) return o.payMethod || o.method
  // 老订单只落了现金：payments 为空但 cash>0，按现金展示
  if (Number(o.cash) > 0) return 'cash'
  return ''
}
function methodText(o) {
  const m = normMethod(o)
  return { cash: '现金', wechat: '扫码', alipay: '被扫', member: '会员余额', split: '组合' }[m] || '—'
}
function cashierOf(o) {
  return o.cashier || o.staffName || o.operator || '—'
}
const cashiers = computed(() => {
  const s = new Set()
  for (const o of list.value) {
    const c = cashierOf(o)
    if (c && c !== '—') s.add(c)
  }
  return [...s]
})
function matchesKw(o) {
  const k = kw.value.trim().toLowerCase()
  if (!k) return true
  if ((o.orderNo || '').toLowerCase().includes(k)) return true
  if (String(o.total || '').includes(k)) return true
  return (o.items || []).some((it) => (it.name || '').toLowerCase().includes(k))
}
// 先按日期/桌台/关键词/支付/收银员筛，再按状态筛；计数基于前者（切状态不改变分母）
const baseFiltered = computed(() => {
  let l = list.value.filter(inDateRange).filter(inTable).filter(matchesKw)
  if (payFilter.value) l = l.filter((o) => !normMethod(o) || normMethod(o) === payFilter.value)
  if (cashierFilter.value) l = l.filter((o) => cashierOf(o) === cashierFilter.value)
  return l
})

const counts = computed(() => {
  const c = { all: baseFiltered.value.length, held: 0, refunded: 0, void: 0 }
  for (const o of baseFiltered.value) {
    if (o.status === 'held') c.held++
    else if (o.status === 'refunded') c.refunded++
    else if (o.status === 'void' || o.status === 'cancelled') c.void++
  }
  return c
})

const filtered = computed(() => {
  const l = baseFiltered.value
  if (statusFilter.value === 'held') return l.filter((o) => o.status === 'held')
  if (statusFilter.value === 'refunded') return l.filter((o) => o.status === 'refunded')
  if (statusFilter.value === 'void') return l.filter((o) => o.status === 'void' || o.status === 'cancelled')
  return l
})

const heldOverCount = computed(() => filtered.value.filter(isHeldOver).length)

// v03 P4 筛选行右侧统计：「共 N 笔 · 今日 ¥X」，不用另调接口
const sumTotal = computed(() =>
  filtered.value.reduce((s, o) => s + (Number(o.total) || 0), 0)
)
const todayTotal = computed(() =>
  list.value
    .filter((o) => o.status === 'paid' && dayKey(o.createdAt) === todayKey())
    .reduce((s, o) => s + (Number(o.total) || 0), 0)
)

// v03 P4 状态标签：已支付绿 / 待支付琥珀 / 已退款·已作废灰 / 待重打红
function statusTag(o) {
  if (o.status === 'held') return { text: '待支付', cls: 'tag-pending' }
  if (o.status === 'refunded') return { text: '已退款', cls: 'tag-stop' }
  if (o.status === 'void' || o.status === 'cancelled') return { text: '已作废', cls: 'tag-stop' }
  if (o.printStatus === 3) return { text: '待重打', cls: 'tag-bad' }
  return { text: '已支付', cls: 'tag-paid' }
}

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
