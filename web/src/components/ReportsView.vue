<template>
  <section class="orders">
    <header class="orders-head reports-head">
      <h2>报表</h2>
      <div class="quick quick-mode">
        <button :class="{ on: mode === 'daily' }" @click="setMode('daily')">日结</button>
        <button :class="{ on: mode === 'monthly' }" @click="setMode('monthly')">月结</button>
      </div>
      <template v-if="mode === 'daily'">
        <input type="date" v-model="date" @change="pickDate" class="date-input" />
        <div class="quick quick-range">
          <button :class="{ on: quick === 'today' }" @click="quickToday">今天</button>
          <button :class="{ on: quick === 'yesterday' }" @click="quickYesterday">昨天</button>
          <button :class="{ on: quick === '7d' }" @click="quick7d">近 7 天</button>
        </div>
        <span class="grow"></span>
        <button @click="exportCSV">导出 CSV</button>
        <button @click="exportMonthly">导出本月</button>
        <button @click="load" :disabled="loading">{{ loading ? '统计中…' : '刷新' }}</button>
      </template>
      <template v-else>
        <input type="month" v-model="month" @change="loadMonth" class="date-input" />
        <span class="grow"></span>
        <button @click="exportMonthlyCSV">导出本月 CSV</button>
        <button @click="refreshMonth" :disabled="loading">{{ loading ? '统计中…' : '刷新' }}</button>
      </template>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <MonthlyView v-if="mode === 'monthly'" :key="month + '-' + refreshTick" :month="month" />
    <template v-else>
    <div class="stat-grid">
      <StatCard label="净销售" :value="'¥' + money(data.netSales)"
                :sub="'销售 ¥' + money(data.grossSales) + ' − 退款 ¥' + money(data.refundTotal)" />
      <StatCard label="订单数" :value="data.orderCount" :sub="'作废 ' + data.voidCount + ' 单（不计销售）'" />
      <StatCard label="客单价" :value="'¥' + money(data.avgTicket)" sub="净销售 ÷ 订单数" />
      <StatCard label="退款" :value="'¥' + money(data.refundTotal)" sub="已从净销售扣除" />
      <StatCard label="会员消费" :value="'¥' + money(data.memberAmount)"
                :sub="data.memberOrders + ' 单 · 占净销售 ' + share" />
      <StatCard label="会员折扣" :value="'¥' + money(data.memberDiscountTotal)" sub="等级让利 · 不计销售扣减" />
      <StatCard label="积分" :value="data.pointsEarned + ' / ' + data.pointsUsed" sub="发放 / 核销" />
      <StatCard label="作废" :value="'¥' + money(data.voidTotal)" :sub="data.voidCount + ' 单 · 不计销售'" />
      <StatCard label="现金净额" :value="'¥' + money(data.cashNet)" sub="实收 − 找零" />
      <StatCard label="毛利(估)" :value="'¥' + money(data.grossProfit)" sub="按商品当前进价估算 · 不做账" />
    </div>

    <div v-if="hourRows.length" class="shift-card">
      <h3>时段分布</h3>
      <div class="methods">
        <div v-for="h in hourRows" :key="h.hour" class="method-row">
          <span class="method-name">{{ h.hour }} 点</span>
          <span class="method-bar"><span class="method-fill" :style="{ width: h.ratio + '%' }"></span></span>
          <span class="method-amt">¥{{ money(h.amount) }}</span>
        </div>
      </div>
    </div>

    <div class="shift-card">
      <h3>收款构成</h3>
      <p v-if="!methodTotal" class="empty-note">这一天没有销售</p>
      <div v-else class="methods">
        <div v-for="m in methodRows" :key="m.method" class="method-row">
          <span class="method-name">{{ methodLabel(m.method) }}</span>
          <span class="method-bar">
            <span class="method-fill" :style="{ width: m.ratio + '%' }"></span>
          </span>
          <span class="method-val">¥{{ money(m.amount) }}</span>
          <span class="method-pct">{{ m.ratio.toFixed(1) }}%</span>
        </div>
      </div>
    </div>

    <div class="shift-card">
      <h3>商品销量 {{ quick === '7d' ? 'Top 10（近 7 天）' : 'Top 10' }}</h3>
      <table class="orders-table" style="margin-top: 8px">
        <thead>
          <tr>
            <th>商品</th>
            <th style="width: 100px" class="num">数量</th>
            <th style="width: 120px" class="num">金额</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in data.topProducts" :key="p.name">
            <td>{{ p.name }}</td>
            <td class="num">{{ p.qty }}</td>
            <td class="num">¥{{ money(p.amount) }}</td>
          </tr>
          <tr v-if="!data.topProducts.length">
            <td colspan="3" class="empty-row">这一天没有销售</td>
          </tr>
        </tbody>
      </table>
    </div>
    </template>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'
import StatCard from './StatCard.vue'
import MonthlyView from './MonthlyView.vue'

const mode = ref('daily')
const month = ref(new Date().toISOString().slice(0, 7))
const refreshTick = ref(0)
const loading = ref(false)
const error = ref('')
const quick = ref('today')
const date = ref(new Date().toISOString().slice(0, 10))
const data = reactive({
  grossSales: 0, refundTotal: 0, voidTotal: 0, netSales: 0, memberAmount: 0, memberOrders: 0,
  memberDiscountTotal: 0, pointsEarned: 0, pointsUsed: 0,
  orderCount: 0, voidCount: 0, avgTicket: 0, cashNet: 0, grossProfit: 0, hourly: [],
  byMethod: [], topProducts: []
})

const METHOD_LABELS = { cash: '现金', wechat: '微信', alipay: '支付宝' }

function methodLabel(m) {
  return METHOD_LABELS[m] || m
}

const hourRows = computed(() => {
  const hourly = data.hourly || []
  const max = Math.max(0, ...hourly.map((x) => Number(x) || 0))
  if (!max) return []
  const rows = []
  for (let h = 0; h < 24; h++) {
    const amount = Number(hourly[h] || 0)
    if (!amount) continue
    rows.push({ hour: h, amount, ratio: Math.round((amount / max) * 100) })
  }
  return rows
})

const methodTotal = computed(() => data.byMethod.reduce((s, m) => s + (Number(m.amount) || 0), 0))

const methodRows = computed(() =>
  data.byMethod
    .map((m) => ({ ...m, ratio: methodTotal.value ? ((Number(m.amount) || 0) / methodTotal.value) * 100 : 0 }))
    .sort((a, b) => b.amount - a.amount)
)

const share = computed(() => {
  const net = Number(data.netSales) || 0
  if (net <= 0) return '0%'
  return Math.round((Number(data.memberAmount) / net) * 100) + '%'
})

function money(v) {
  return (Number(v) || 0).toFixed(2)
}

function fmt(d) {
  return d.toISOString().slice(0, 10)
}

function pickDate() {
  quick.value = ''
  load()
}

function setMode(m) {
  mode.value = m
}

function loadMonth() {
  // 月份变化由 MonthlyView 的 :key 重建触发加载
}

async function refreshMonth() {
  // 重建 MonthlyView 触发重新加载
  refreshTick.value++
}

function exportMonthlyCSV() {
  const token = localStorage.getItem('pos.token') || ''
  fetch('/pos/v1/reports/monthly.csv?month=' + month.value, { headers: { Authorization: 'Bearer ' + token } })
    .then((r) => r.blob())
    .then((blob) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = 'monthly-' + month.value + '.csv'
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => { error.value = '导出失败' })
}

function quickToday() {
  quick.value = 'today'
  date.value = fmt(new Date())
  load()
}

function quickYesterday() {
  quick.value = 'yesterday'
  const d = new Date()
  d.setDate(d.getDate() - 1)
  date.value = fmt(d)
  load()
}

function quick7d() {
  quick.value = '7d'
  date.value = fmt(new Date())
  load()
}

function exportMonthly() {
  const token = localStorage.getItem('pos.token') || ''
  const month = date.value.slice(0, 7)
  fetch('/pos/v1/reports/monthly.csv?month=' + month, { headers: { Authorization: 'Bearer ' + token } })
    .then((r) => r.blob())
    .then((blob) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = 'monthly-' + month + '.csv'
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => { error.value = '导出失败' })
}

function exportCSV() {
  const token = localStorage.getItem('pos.token') || ''
  const url = '/pos/v1/reports/daily.csv?date=' + date.value.replaceAll('-', '')
  // 用 fetch 带 token，再触发下载（直接开新窗口带不了 Authorization）
  fetch(url, { headers: { Authorization: 'Bearer ' + token } })
    .then((r) => r.blob())
    .then((blob) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = 'daily-' + date.value + '.csv'
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => { error.value = '导出失败' })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    if (quick.value === '7d') {
      const days = []
      for (let i = 0; i < 7; i++) {
        const d = new Date()
        d.setDate(d.getDate() - i)
        days.push(fmt(d))
      }
      const list = await Promise.all(days.map((d) => api.dailyReport(d.replaceAll('-', ''))))
      aggregate(list)
    } else {
      const d = await api.dailyReport(date.value.replaceAll('-', ''))
      Object.assign(data, d)
    }
  } catch (e) {
    error.value = '统计失败：' + (e.message || '未知错误')
  } finally {
    loading.value = false
  }
}

// 近 7 天：聚合各日报表（净销售/订单/收款构成/商品销量）。
function aggregate(list) {
  const netSales = list.reduce((s, d) => s + (Number(d.netSales) || 0), 0)
  const grossSales = list.reduce((s, d) => s + (Number(d.grossSales) || 0), 0)
  const refundTotal = list.reduce((s, d) => s + (Number(d.refundTotal) || 0), 0)
  const orderCount = list.reduce((s, d) => s + (Number(d.orderCount) || 0), 0)
  const voidCount = list.reduce((s, d) => s + (Number(d.voidCount) || 0), 0)
  const cashNet = list.reduce((s, d) => s + (Number(d.cashNet) || 0), 0)

  const byMethodMap = {}
  for (const d of list) {
    for (const m of d.byMethod || []) {
      byMethodMap[m.method] = (byMethodMap[m.method] || 0) + (Number(m.amount) || 0)
    }
  }
  const topMap = {}
  for (const d of list) {
    for (const p of d.topProducts || []) {
      if (!topMap[p.name]) topMap[p.name] = { name: p.name, qty: 0, amount: 0 }
      topMap[p.name].qty += Number(p.qty) || 0
      topMap[p.name].amount += Number(p.amount) || 0
    }
  }

  Object.assign(data, {
    netSales, grossSales, refundTotal, orderCount, voidCount, cashNet,
    avgTicket: orderCount ? netSales / orderCount : 0,
    byMethod: Object.entries(byMethodMap).map(([method, amount]) => ({ method, amount })),
    topProducts: Object.values(topMap).sort((a, b) => b.amount - a.amount).slice(0, 10)
  })
}

onMounted(load)
</script>

<style scoped>
.quick {
  display: flex;
  gap: 6px;
  margin-left: 8px;
}

.quick button {
  padding: 5px 10px;
  font-size: 12px;
  border-radius: 6px;
  border: 1px solid var(--line);
  color: var(--muted);
  background: var(--panel-2);
  cursor: pointer;
}

.quick button.on {
  border-color: var(--brand);
  color: var(--brand);
}

.empty-note {
  margin: 14px 0;
  color: var(--muted);
  font-size: 13px;
}

.methods {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.method-row {
  display: grid;
  grid-template-columns: 52px 1fr 90px 56px;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.method-name {
  color: var(--text);
}

.method-bar {
  height: 10px;
  background: var(--line);
  border-radius: 999px;
  overflow: hidden;
}

.method-fill {
  display: block;
  height: 100%;
  background: var(--brand);
  border-radius: 999px;
  transition: width 0.3s;
}

.method-val {
  text-align: right;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.method-pct {
  text-align: right;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
</style>
