<template>
  <section class="orders shift">
    <header class="orders-head">
      <h2>{{ tr('shift.title') }}</h2>
      <span class="grow"></span>
      <span class="shift-open-pill" v-if="shift && shift.status === 'open'">{{ tr('shift.openPill', { time: openTime }) }}</span>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="!shift" class="shift-card">
      <h3>{{ tr('shift.open') }}</h3>
      <p class="muted">{{ tr('shift.openHint') }}</p>
      <div class="shift-form">
        <label>{{ tr('shift.openingCash') }} <input type="number" step="0.01" v-model.number="openingCash" /></label>
        <label>{{ tr('shift.cashier') }} <input v-model="staffName" /></label>
        <button class="primary" :disabled="busy" @click="open">{{ tr('shift.open') }}</button>
      </div>
    </div>

    <template v-else>
      <!-- v03 P6：左卡（1.1）2×2 汇总 + 合计营业/笔数 28px + 差异条；右列（0.9）班次信息 + 结班动作 -->
      <div class="shift-layout">
        <div class="shift-main">
          <div class="shift-main-title">{{ tr('shift.summaryTitle', { no: shift.shiftNo }) }}</div>
          <div class="shift-sum">
            <div class="sum-cell">
              <span>{{ tr('shift.cashExpected') }}</span>
              <b class="money">¥{{ expected.toFixed(2) }}</b>
            </div>
            <div class="sum-cell">
              <span>{{ tr('shift.cashActual') }}</span>
              <input class="sum-input" type="number" step="0.01" v-model.number="actualCash" />
            </div>
            <div class="sum-cell">
              <span>{{ tr('shift.scan') }}</span>
              <b class="money">¥{{ scanIn.toFixed(2) }}</b>
            </div>
            <div class="sum-cell">
              <span>{{ tr('shift.scanned') }}</span>
              <b class="money">¥{{ swipeIn.toFixed(2) }}</b>
            </div>
          </div>

          <div class="shift-divider"></div>

          <div class="shift-totals">
            <div>
              <span>{{ tr('shift.totalSales') }}</span>
              <b class="big brand">¥{{ shift.summary.total.toFixed(2) }}</b>
            </div>
            <div>
              <span>{{ tr('shift.orderCount') }}</span>
              <b class="big">{{ shift.summary.orderCount }}</b>
            </div>
          </div>

          <div class="shift-diff" :class="diffOk ? 'ok' : 'bad'">
            {{ tr('shift.diffPrefix') }} ¥{{ diffValue.toFixed(2) }}，{{ diffOk ? tr('shift.diffOk') : tr('shift.diffNeedNote') }}
          </div>

          <div class="shift-note-field">
            <label class="pay-field-label">{{ tr('shift.diffNote') }}</label>
            <input v-model="note" :placeholder="tr('shift.diffPh')" />
          </div>

          <table v-if="payments.length" class="orders-table tbl" style="margin-top: 14px">
            <thead>
              <tr><th>{{ tr('shift.method') }}</th><th class="num">{{ tr('shift.amount') }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="p in payments" :key="p.method">
                <td>{{ methodName(p.method) }}</td>
                <td class="num">¥{{ p.amount.toFixed(2) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="shift-side">
          <div class="shift-card">
            <h3>{{ tr('shift.info') }}</h3>
            <div class="shift-info">
              <div><span>{{ tr('shift.shiftNo') }}</span><b class="mono">{{ shift.shiftNo }}</b></div>
              <div><span>{{ tr('shift.openedAt') }}</span><b>{{ openTime }} · {{ shift.staffName || '—' }}</b></div>
              <div><span>{{ tr('shift.openingCash') }}</span><b>¥{{ num(shift.openingCash).toFixed(2) }}</b></div>
              <div><span>{{ tr('shift.handoverTo') }}</span><b>{{ shift.handoverTo || tr('shift.nextCashier') }}</b></div>
            </div>
          </div>

          <div v-if="shift.status === 'open' && role !== 'manager'" class="shift-card">
            <h3>{{ tr('shift.closeActions') }}</h3>
            <p class="muted">{{ tr('shift.managerOnly') }}</p>
          </div>

          <div v-else-if="shift.status === 'open'" class="shift-card">
            <h3>{{ tr('shift.closeActions') }}</h3>
            <p class="shift-card-sub">{{ tr('shift.closeHint') }}</p>
            <div class="shift-close">
              <button class="btn-line" @click="printSheet">{{ tr('shift.printSheet') }}</button>
              <button class="primary" :disabled="busy" @click="close">
                {{ busy ? tr('shift.working') : tr('shift.confirmClose') }}
              </button>
            </div>
          </div>

          <div v-else class="shift-card">
            <h3>{{ tr('shift.closed') }}</h3>
            <div class="shift-close">
              <label>{{ tr('shift.cashExpected') }} <b>¥{{ num(shift.expectedCash).toFixed(2) }}</b></label>
              <label>{{ tr('shift.cashActual') }} <b>¥{{ num(shift.actualCash).toFixed(2) }}</b></label>
              <label>{{ tr('shift.diff') }} <b :class="{ bad: num(shift.difference) !== 0 }">¥{{ num(shift.difference).toFixed(2) }}</b></label>
              <label v-if="shift.note">{{ tr('shift.note') }} <b>{{ shift.note }}</b></label>
              <button class="primary" @click="reset">{{ tr('shift.nextShift') }}</button>
            </div>
          </div>

        </div>
      </div>
    </template>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr, locale } from '../i18n.js'

const props = defineProps({
  role: { type: String, default: 'cashier' }
})

const shift = ref(null)
const openingCash = ref(0)
const staffName = ref(locale.value === 'zh-CN' ? '陈晓' : 'Manager')
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

// v03 P6 差异条：营业中按当前录入的实收现金实时算，已结班读班次落库值
const diffValue = computed(() => {
  if (!shift.value) return 0
  return shift.value.status === 'open' ? difference.value : num(shift.value.difference)
})
const diffOk = computed(() => Math.abs(diffValue.value) < 0.005)

// 本班收款方式明细：后端无收款时 payments 为 null，不能直接 .length
const payments = computed(() => {
  const s = shift.value && shift.value.summary
  if (!s || !Array.isArray(s.payments)) return []
  return s.payments
})

function num(v) {
  return typeof v === 'number' ? v : 0
}

function methodName(m) {
  return { cash: tr('shift.payCash'), offline_qr: tr('shift.payOffline'), credit: tr('shift.payCredit'), wechat: tr('shift.payWechat'), alipay: tr('shift.payAlipay'), member: tr('shift.payMember'), split: tr('shift.paySplit') }[m] || m
}

// v03 P6 顶部胶囊「●营业中 · 09:00 开班」
const openTime = computed(() => {
  const t = (shift.value && shift.value.openedAt) || ''
  const m = t.match(/(\d{2}:\d{2})/)
  return m ? m[1] : '--:--'
})
// 2×2 里的「扫码 / 被扫」按收款方式归类汇总
function paySum(keys) {
  return payments.value
    .filter((p) => keys.includes(p.method))
    .reduce((s, p) => s + (Number(p.amount) || 0), 0)
}
const scanIn = computed(() => paySum(['wechat', 'offline_qr']))
const swipeIn = computed(() => paySum(['alipay']))

// 交接单打印：本页即交接单版式，直接走系统打印
function printSheet() {
  window.print()
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
    error.value = tr('shift.loadFailed', { msg: e.message || '' })
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
    toast(tr('shift.opened', { no: shift.value.shiftNo }))
  } catch (e) {
    toast(tr('shift.openFailed', { msg: e.message || '' }), true)
  } finally {
    busy.value = false
  }
}

async function close() {
  busy.value = true
  try {
    shift.value = await api.closeShift(shift.value.id, { actualCash: Number(actualCash.value || 0), note: note.value })
    toast(tr('shift.closedToast'))
  } catch (e) {
    toast(tr('shift.closeFailed', { msg: e.message || '' }), true)
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
