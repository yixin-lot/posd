<template>
  <section class="orders shift">
    <header class="orders-head">
      <h2>交接班</h2>
      <span class="grow"></span>
      <span class="shift-open-pill" v-if="shift && shift.status === 'open'">● 营业中 · {{ openTime }} 开班</span>
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
      <!-- v03 P6：左卡（1.1）2×2 汇总 + 合计营业/笔数 28px + 差异条；右列（0.9）班次信息 + 结班动作 -->
      <div class="shift-layout">
        <div class="shift-main">
          <div class="shift-main-title">结班汇总 · {{ shift.shiftNo }}</div>
          <div class="shift-sum">
            <div class="sum-cell">
              <span>现金（应收）</span>
              <b class="money">¥{{ expected.toFixed(2) }}</b>
            </div>
            <div class="sum-cell">
              <span>现金（实收）</span>
              <input class="sum-input" type="number" step="0.01" v-model.number="actualCash" />
            </div>
            <div class="sum-cell">
              <span>扫码</span>
              <b class="money">¥{{ scanIn.toFixed(2) }}</b>
            </div>
            <div class="sum-cell">
              <span>被扫</span>
              <b class="money">¥{{ swipeIn.toFixed(2) }}</b>
            </div>
          </div>

          <div class="shift-divider"></div>

          <div class="shift-totals">
            <div>
              <span>合计营业</span>
              <b class="big brand">¥{{ shift.summary.total.toFixed(2) }}</b>
            </div>
            <div>
              <span>订单笔数</span>
              <b class="big">{{ shift.summary.orderCount }}</b>
            </div>
          </div>

          <div class="shift-diff" :class="diffOk ? 'ok' : 'bad'">
            ● 现金差异 ¥{{ diffValue.toFixed(2) }}，{{ diffOk ? '账目一致' : '请填写差异说明后再结班' }}
          </div>

          <div class="shift-note-field">
            <label class="pay-field-label">差异说明（可选，差异非 0 时必填）</label>
            <input v-model="note" placeholder="例：晚间找零多 10 元，已当面点清" />
          </div>

          <table v-if="payments.length" class="orders-table tbl" style="margin-top: 14px">
            <thead>
              <tr><th>收款方式</th><th class="num">金额</th></tr>
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
            <h3>班次信息</h3>
            <div class="shift-info">
              <div><span>班次</span><b class="mono">{{ shift.shiftNo }}</b></div>
              <div><span>开班</span><b>{{ openTime }} · {{ shift.staffName || '—' }}</b></div>
              <div><span>期初现金</span><b>¥{{ num(shift.openingCash).toFixed(2) }}</b></div>
              <div><span>交接对象</span><b>{{ shift.handoverTo || '下一班收银员' }}</b></div>
            </div>
          </div>

          <div v-if="shift.status === 'open' && role !== 'manager'" class="shift-card">
            <h3>结班动作</h3>
            <p class="muted">结班需要店长权限，请让店长登录后操作。</p>
          </div>

          <div v-else-if="shift.status === 'open'" class="shift-card">
            <h3>结班动作</h3>
            <p class="shift-card-sub">结班后本班次锁定，现金盘点交给下一班</p>
            <div class="shift-close">
              <button class="btn-line" @click="printSheet">打印交接单</button>
              <button class="primary" :disabled="busy" @click="close">
                {{ busy ? '处理中…' : '确认结班' }}
              </button>
            </div>
          </div>

          <div v-else class="shift-card">
            <h3>已结班</h3>
            <div class="shift-close">
              <label>现金应收 <b>¥{{ num(shift.expectedCash).toFixed(2) }}</b></label>
              <label>现金实收 <b>¥{{ num(shift.actualCash).toFixed(2) }}</b></label>
              <label>差异 <b :class="{ bad: num(shift.difference) !== 0 }">¥{{ num(shift.difference).toFixed(2) }}</b></label>
              <label v-if="shift.note">备注 <b>{{ shift.note }}</b></label>
              <button class="primary" @click="reset">开下一班</button>
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
  return { cash: '现金', offline_qr: '线下码', credit: '挂账', wechat: '扫码', alipay: '被扫', member: '会员余额', split: '组合' }[m] || m
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
