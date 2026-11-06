<template>
  <section class="orders shift">
    <header class="orders-head">
      <h2>交接班</h2>
      <span class="grow"></span>
      <button @click="load" :disabled="busy">{{ busy ? '处理中…' : '刷新' }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="!shift" class="shift-card">
      <h3>开班</h3>
      <p class="muted">开班后本班订单都归到这个班次，结班时用来对账。</p>
      <div class="shift-form">
        <label>期初现金 <input type="number" step="0.01" v-model.number="openingCash" /></label>
        <label>收银员 <input v-model="staffName" /></label>
        <button class="primary" :disabled="busy" @click="open">开班</button>
      </div>
    </div>

    <template v-else>
      <div class="shift-cards">
        <div class="shift-card">
          <div class="label">班次</div>
          <div class="value mono">{{ shift.shiftNo }}</div>
          <div class="muted">{{ shift.status === 'open' ? '进行中' : '已结班' }} · {{ shift.staffName || '未署名' }}</div>
        </div>
        <div class="shift-card">
          <div class="label">订单笔数</div>
          <div class="value">{{ shift.summary.orderCount }}</div>
          <div class="muted">{{ shift.openedAt }}</div>
        </div>
        <div class="shift-card">
          <div class="label">应收合计</div>
          <div class="value money">¥{{ shift.summary.total.toFixed(2) }}</div>
          <div class="muted">含各类收款</div>
        </div>
        <div class="shift-card">
          <div class="label">现金净额</div>
          <div class="value money">¥{{ shift.summary.cashNet.toFixed(2) }}</div>
          <div class="muted">已扣找零</div>
        </div>
        <div class="shift-card" v-if="(shift.summary.crossRefundCount || 0) > 0">
          <div class="label">跨班次退款</div>
          <div class="value">¥{{ Number(shift.summary.crossRefundTotal || 0).toFixed(2) }}</div>
          <div class="muted">{{ shift.summary.crossRefundCount }} 笔 · 原单不在本班次</div>
        </div>
        <div class="shift-card">
          <div class="label">积分发放 / 核销</div>
          <div class="value">{{ shift.summary.pointsEarned || 0 }} / {{ shift.summary.pointsUsed || 0 }}</div>
          <div class="muted">本班次</div>
        </div>
        <div class="shift-card">
          <div class="label">堂食 / 外带</div>
          <div class="value">{{ shift.summary.dineInCount || 0 }} / {{ shift.summary.takeawayCount || 0 }}</div>
          <div class="muted">按单数统计</div>
        </div>
        <div class="shift-card">
          <div class="label">退款合计</div>
          <div class="value money">¥{{ shift.summary.refundTotal.toFixed(2) }}</div>
          <div class="muted">现金退款要拿出钱箱</div>
        </div>
        <div class="shift-card">
          <div class="label">会员折扣</div>
          <div class="value money">¥{{ Number(shift.summary.memberDiscountTotal || 0).toFixed(2) }}</div>
          <div class="muted">等级让利 · 不计销售扣减</div>
        </div>
      </div>

      <p class="muted" style="margin: 0 0 12px">作废单与挂单不计入本班次销售。</p>

      <table v-if="shift.summary.payments.length" class="orders-table">
        <thead>
          <tr><th>收款方式</th><th class="num">金额</th></tr>
        </thead>
        <tbody>
          <tr v-for="p in shift.summary.payments" :key="p.method">
            <td>{{ methodName(p.method) }}</td>
            <td class="num">¥{{ p.amount.toFixed(2) }}</td>
          </tr>
        </tbody>
      </table>

      <div v-if="shift.status === 'open' && role !== 'manager'" class="shift-card">
        <h3>结班</h3>
        <p class="muted">结班需要店长权限，请让店长登录后操作。</p>
      </div>

      <div v-else-if="shift.status === 'open'" class="shift-card">
        <h3>结班</h3>
        <div class="shift-form">
          <label>现金应收 <b>¥{{ expected.toFixed(2) }}</b>
            <span class="muted" style="font-weight: 400; font-size: 12px">现金应收 = 期初 + 收款 − 退款</span>
          </label>
          <label>实收现金 <input type="number" step="0.01" v-model.number="actualCash" /></label>
          <label>差异 <b :class="{ bad: difference !== 0 }">¥{{ difference.toFixed(2) }}</b></label>
          <label>备注 <input v-model="note" placeholder="差异说明（可选）" /></label>
          <button class="primary" :disabled="busy" @click="close">结班并打印交接单</button>
        </div>
      </div>

      <div v-else class="shift-card">
        <h3>已结班</h3>
        <div class="shift-form">
          <label>现金应收 <b>¥{{ num(shift.expectedCash).toFixed(2) }}</b></label>
          <label>现金实收 <b>¥{{ num(shift.actualCash).toFixed(2) }}</b></label>
          <label>差异 <b :class="{ bad: num(shift.difference) !== 0 }">¥{{ num(shift.difference).toFixed(2) }}</b></label>
          <label v-if="shift.note">备注 <b>{{ shift.note }}</b></label>
          <button class="primary" @click="reset">开下一班</button>
        </div>
      </div>
    </template>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/index.js'

const props = defineProps({
  role: { type: String, default: 'cashier' }
})

const shift = ref(null)
const openingCash = ref(0)
const staffName = ref('陈晓')
const actualCash = ref(0)
const note = ref('')
const busy = ref(false)
const error = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
let toastTimer = null

const expected = computed(() =>
  shift.value ? shift.value.openingCash + shift.value.summary.cashNet - shift.value.summary.refundTotal : 0
)
const difference = computed(() => Math.round((Number(actualCash.value || 0) - expected.value) * 100) / 100)

function num(v) {
  return typeof v === 'number' ? v : 0
}

function methodName(m) {
  return { cash: '现金', offline_qr: '线下码', credit: '挂账' }[m] || m
}

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toastMsg.value = ''), 2600)
}

async function load() {
  busy.value = true
  error.value = ''
  try {
    shift.value = await api.latestShift()
    if (shift.value && shift.value.status === 'open') {
      actualCash.value = Math.round(expected.value * 100) / 100
    }
  } catch (e) {
    error.value = '加载班次失败：' + (e.message || '未知错误')
  } finally {
    busy.value = false
  }
}

async function open() {
  busy.value = true
  error.value = ''
  try {
    shift.value = await api.openShift({ openingCash: Number(openingCash.value || 0), staffName: staffName.value })
    actualCash.value = Math.round(expected.value * 100) / 100
    toast('已开班 ' + shift.value.shiftNo)
  } catch (e) {
    toast('开班失败：' + (e.message || '未知错误'), true)
  } finally {
    busy.value = false
  }
}

async function close() {
  busy.value = true
  try {
    shift.value = await api.closeShift(shift.value.id, { actualCash: Number(actualCash.value || 0), note: note.value })
    toast('已结班，交接单已提交打印')
  } catch (e) {
    toast('结班失败：' + (e.message || '未知错误'), true)
  } finally {
    busy.value = false
  }
}

function reset() {
  shift.value = null
  openingCash.value = 0
  actualCash.value = 0
  note.value = ''
}

onMounted(load)
</script>
