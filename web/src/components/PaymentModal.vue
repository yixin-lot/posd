<template>
  <div class="mask" @click.self="$emit('close')">
    <div class="modal pay-modal">
      <div class="pay-head">
        <div>
          <h3>收银台结算</h3>
          <div class="pay-sub">订单 {{ orderLabel }} · {{ count }} 件商品</div>
        </div>
        <button class="modal-x" title="关闭" @click="$emit('close')">×</button>
      </div>

      <div class="pay-label">{{ tr('pay.due') }}</div>
      <div class="pay-amount">{{ money(total) }}</div>

      <div class="pay-methods">
        <button
          v-for="m in methods"
          :key="m.key"
          :class="{ active: method === m.key }"
          @click="pick(m.key)"
        >
          {{ tr('pay.' + m.key) }}
        </button>
      </div>

      <div v-if="method === 'cash'" class="pay-cols">
        <div class="pay-col">
          <label class="pay-field-label">{{ tr('pay.received') }}</label>
          <input
            ref="cashInput"
            class="cash-input"
            inputmode="decimal"
            autocomplete="off"
            :value="cashText"
            @input="onCashInput"
            @keyup.enter="confirm"
          />
          <div class="quicks">
            <button v-for="q in quicks" :key="q.v" @click="setQuick(q.v)">{{ q.label }}</button>
          </div>
        </div>
        <div class="pay-col change-col">
          <label class="pay-field-label">{{ tr('pay.change') }}</label>
          <div class="change" :class="{ bad: change < 0 }">{{ money(change) }}</div>
          <div class="change-hint">回车快速确认收款</div>
        </div>
      </div>

      <div v-else-if="method === 'split'" class="split-box">
        <div class="split-row" v-for="(row, i) in splitRows" :key="i">
          <select v-model="row.method">
            <option value="cash">现金</option>
            <option value="wechat">微信</option>
            <option value="alipay">支付宝</option>
            <option v-if="member" value="member">会员余额</option>
          </select>
          <input
            v-if="i === 0"
            class="split-amount"
            inputmode="decimal"
            :value="splitFirstText"
            @input="onSplitInput"
          />
          <input v-else class="split-amount" :value="splitRest.toFixed(2)" readonly />
        </div>
        <div class="split-sum" :class="{ ok: splitOk }">
          合计 {{ money(splitFirst) }} + {{ money(splitRest) }} = {{ money(total) }}
          <span v-if="!splitOk" class="split-warn">两笔合计需等于应收</span>
        </div>
      </div>

      <div v-else-if="method === 'member'" class="pay-note-box">
        <p class="hint" style="margin: 0">
          {{ member.name || member.phone }} 余额 {{ money(member.balance) }}，
          本单将扣 {{ money(total) }}，余额剩 {{ money(Number(member.balance) - total) }}。
        </p>
      </div>

      <div v-else-if="method === 'points'" class="pay-note-box">
        <p class="hint" style="margin: 0">
          {{ member.name || member.phone }} 有 {{ member.points }} 积分，
          本单抵扣 {{ Math.round(total * 100) }} 分（100 积分 = 1 元）。
        </p>
      </div>

      <div v-else class="pay-note-box">
        <p class="hint" style="margin: 0">
          请确认顾客已用{{ label }}付清 {{ money(total) }}，然后点确认（系统只记账，不自动对账）。
        </p>
      </div>

      <div class="modal-actions">
        <button class="cancel" @click="$emit('close')">取消</button>
        <button class="primary" :disabled="submitting || !canConfirm" @click="confirm">
          {{ submitting ? tr('pay.processing') : tr('pay.confirm') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { t as tr } from '../i18n.js'
import { money } from '../i18n.js'

const props = defineProps({
  total: { type: Number, required: true },
  submitting: { type: Boolean, default: false },
  member: { type: Object, default: null },
  count: { type: Number, default: 0 },
  orderLabel: { type: String, default: '待生成' },
  // 结算区 2×2 按钮预选的收款方式（cash / wechat / alipay / split）
  preset: { type: String, default: 'cash' }
})
const emit = defineEmits(['confirm', 'close'])

// v03 P3：四个方式胶囊为固定四档；有会员时在末尾追加「会员余额 / 积分抵扣」（会员支付是必需功能）
const methods = computed(() => {
  const list = [
    { key: 'cash' },
    { key: 'wechat' },
    { key: 'alipay' },
    { key: 'split' }
  ]
  if (props.member) {
    list.push({ key: 'member' })
    // 100 积分 = 1 元；够本单才显示
    if (Number(props.member.points || 0) >= Math.round(props.total * 100)) {
      list.push({ key: 'points', label: '积分抵扣' })
    }
  }
  return list
})

const method = ref(['cash', 'wechat', 'alipay', 'split'].includes(props.preset) ? props.preset : 'cash')
const cash = ref(props.total)
const cashText = ref(props.total.toFixed(2))
const cashInput = ref(null)

const label = computed(() => ({ cash: '现金', wechat: '扫码', alipay: '被扫', member: '会员余额', points: '积分抵扣', split: '组合' }[method.value] || '现金'))
const change = computed(() => Math.round((Number(cash.value || 0) - props.total) * 100) / 100)

// 快捷额按设计稿固定三档：50 / 100 / 其他
const quicks = [
  { v: 50, label: '50' },
  { v: 100, label: '100' },
  { v: 'custom', label: '其他' }
]

// 组合支付（P16）：两笔分账，第二笔自动补足，合计必须等于应收
const splitRows = ref([
  { method: 'cash', amount: props.total },
  { method: 'wechat', amount: 0 }
])
const splitFirst = computed(() => Math.round(Number(splitRows.value[0].amount || 0) * 100) / 100)
const splitRest = computed(() => Math.max(0, Math.round((props.total - splitFirst.value) * 100) / 100))
const splitFirstText = ref(props.total.toFixed(2))
const splitOk = computed(() => splitFirst.value >= 0 && splitFirst.value <= props.total + 0.001)

const canConfirm = computed(() => {
  if (method.value === 'cash') return change.value >= 0
  if (method.value === 'split') return splitOk.value
  if (method.value === 'member') return !!props.member && Number(props.member.balance || 0) >= props.total
  if (method.value === 'points') return !!props.member && Number(props.member.points || 0) >= Math.round(props.total * 100)
  return true
})

function onCashInput(e) {
  const raw = String(e.target.value || '').replace(/[^0-9.]/g, '')
  cashText.value = raw
  const n = Number(raw)
  cash.value = Number.isFinite(n) ? Math.round(n * 100) / 100 : 0
}

function setQuick(v) {
  if (v === 'custom') {
    cashText.value = ''
    cash.value = 0
    nextTick(() => cashInput.value && cashInput.value.focus())
    return
  }
  cashText.value = Number(v).toFixed(2)
  cash.value = Number(v)
}

function onSplitInput(e) {
  const raw = String(e.target.value || '').replace(/[^0-9.]/g, '')
  splitFirstText.value = raw
  const n = Number(raw)
  splitRows.value[0].amount = Number.isFinite(n) ? Math.round(n * 100) / 100 : 0
}

function pick(key) {
  method.value = key
  if (key === 'cash') {
    cashText.value = props.total.toFixed(2)
    cash.value = props.total
    nextTick(() => cashInput.value && cashInput.value.select())
  }
}

function confirm() {
  if (props.submitting || !canConfirm.value) return
  if (method.value === 'split') {
    emit('confirm', {
      method: 'split',
      payments: [
        { method: splitRows.value[0].method, amount: splitFirst.value },
        { method: splitRows.value[1].method, amount: splitRest.value }
      ],
      cash: splitRows.value[0].method === 'cash' ? splitFirst.value : 0
    })
    return
  }
  emit('confirm', {
    method: method.value,
    memberId: method.value === 'member' && props.member ? props.member.id : 0,
    // 扫码金额固定为应收；现金传实收，由后端算找零
    cash: method.value === 'cash' ? Math.round(Number(cash.value) * 100) / 100 : props.total
  })
}

onMounted(() => {
  if (cashInput.value) {
    cashInput.value.focus()
    cashInput.value.select()
  }
})
</script>
