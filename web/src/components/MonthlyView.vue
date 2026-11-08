<template>
  <div class="monthly">
    <p v-if="error" class="orders-error">{{ error }}</p>

    <div class="stat-grid">
      <StatCard :label="tr('reports.netSales')" :value="imoney(data.netSales)"
                :sub="tr('reports.grossSales') + ' ' + imoney(data.grossSales) + ' − ' + tr('reports.refunds') + ' ' + imoney(data.refundTotal)" />
      <StatCard :label="tr('reports.orders')" :value="data.orderCount" :sub="tr('monthly.ordersSub')" />
      <StatCard :label="tr('monthly.gross')" :value="imoney(data.grossSales)" :sub="tr('monthly.grossSub')" />
      <StatCard :label="tr('reports.refundCard')" :value="imoney(data.refundTotal)" :sub="tr('reports.refundSub')" />
      <StatCard :label="tr('reports.memberDiscount')" :value="imoney(data.memberDiscountTotal)" :sub="tr('reports.memberDiscountSub')" />
      <StatCard :label="tr('reports.voidCard')" :value="imoney(data.voidTotal)" :sub="tr('reports.voidSub', { n: data.voidCount })" />
      <StatCard :label="tr('reports.avgTicket')" :value="imoney(data.avgTicket)" :sub="tr('reports.avgSub')" />
    </div>

    <div class="shift-card">
      <h3>{{ tr('reports.methods') }}</h3>
      <p v-if="!methodTotal" class="empty-note">{{ tr('monthly.noSales') }}</p>
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
      <h3>{{ tr('monthly.top20') }}</h3>
      <table class="orders-table" style="margin-top: 8px">
        <thead>
          <tr>
            <th>{{ tr('reports.item') }}</th>
            <th style="width: 100px" class="num">{{ tr('reports.qty') }}</th>
            <th style="width: 120px" class="num">{{ tr('reports.amount') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in data.topProducts" :key="p.name">
            <td>{{ p.name }}</td>
            <td class="num">{{ p.qty }}</td>
            <td class="num">¥{{ money(p.amount) }}</td>
          </tr>
          <tr v-if="!data.topProducts.length">
            <td colspan="3" class="empty-row">{{ tr('monthly.noSales') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'
import StatCard from './StatCard.vue'
import { t as tr, money as imoney } from '../i18n.js'

const props = defineProps({ month: { type: String, required: true } })
const emit = defineEmits(['export'])

const loading = ref(false)
const error = ref('')
const data = reactive({
  orderCount: 0, grossSales: 0, refundTotal: 0, voidTotal: 0,
  netSales: 0, avgTicket: 0, byMethod: [], topProducts: []
})

const METHOD_LABELS = { cash: tr('orders.cash'), wechat: 'WeChat', alipay: 'Alipay' }

function methodLabel(m) {
  return METHOD_LABELS[m] || m
}

const methodTotal = computed(() => data.byMethod.reduce((s, m) => s + (Number(m.amount) || 0), 0))

const methodRows = computed(() =>
  data.byMethod
    .map((m) => ({ ...m, ratio: methodTotal.value ? ((Number(m.amount) || 0) / methodTotal.value) * 100 : 0 }))
    .sort((a, b) => b.amount - a.amount)
)

function money(v) {
  return (Number(v) || 0).toFixed(2)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    Object.assign(data, await api.monthlyReport(props.month))
  } catch (e) {
    error.value = tr('monthly.statFailed', { msg: e.message || '' })
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
