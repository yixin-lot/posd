<template>
  <div class="monthly">
    <p v-if="error" class="orders-error">{{ error }}</p>

    <div class="stat-grid">
      <StatCard label="净销售" :value="'¥' + money(data.netSales)"
                :sub="'销售 ¥' + money(data.grossSales) + ' − 退款 ¥' + money(data.refundTotal)" />
      <StatCard label="订单数" :value="data.orderCount" sub="当月成交单数" />
      <StatCard label="销售合计" :value="'¥' + money(data.grossSales)" sub="不含退款" />
      <StatCard label="退款" :value="'¥' + money(data.refundTotal)" sub="已从净销售扣除" />
      <StatCard label="会员折扣" :value="'¥' + money(data.memberDiscountTotal)" sub="等级让利" />
      <StatCard label="作废" :value="'¥' + money(data.voidTotal)" sub="不计销售" />
      <StatCard label="客单价" :value="'¥' + money(data.avgTicket)" sub="净销售 ÷ 订单数" />
    </div>

    <div class="shift-card">
      <h3>收款构成</h3>
      <p v-if="!methodTotal" class="empty-note">这个月没有销售</p>
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
      <h3>商品销量 Top 20</h3>
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
            <td colspan="3" class="empty-row">这个月没有销售</td>
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

const props = defineProps({ month: { type: String, required: true } })
const emit = defineEmits(['export'])

const loading = ref(false)
const error = ref('')
const data = reactive({
  orderCount: 0, grossSales: 0, refundTotal: 0, voidTotal: 0,
  netSales: 0, avgTicket: 0, byMethod: [], topProducts: []
})

const METHOD_LABELS = { cash: '现金', wechat: '微信', alipay: '支付宝' }

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
    error.value = '统计失败：' + (e.message || '未知错误')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
