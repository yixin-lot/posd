<template>
  <div class="mask" @click.self="$emit('close')">
    <div class="modal">
      <h3>收银台结算</h3>
      <div class="pay-amount">¥{{ total.toFixed(2) }}</div>

      <div class="pay-methods">
        <button
          v-for="m in methods"
          :key="m.key"
          :class="{ active: method === m.key }"
          @click="pick(m.key)"
        >
          {{ m.label }}
        </button>
      </div>

      <template v-if="method === 'cash'">
        <input
          ref="cashInput"
          type="number"
          step="0.01"
          v-model.number="cash"
          placeholder="实收金额"
          style="font-size: 22px; text-align: center"
          @keyup.enter="confirm"
        />

        <div class="quicks">
          <button v-for="q in quicks" :key="q" @click="cash = q">{{ q }}</button>
        </div>

        <div class="change">
          <span>找零</span>
          <b>¥{{ change.toFixed(2) }}</b>
        </div>
      </template>
      <p v-else-if="method === 'points'" class="hint" style="margin: 0">
        {{ member.name || member.phone }} 有 {{ member.points }} 积分，
        本单抵扣 {{ Math.round(total * 100) }} 分（100 积分 = 1 元）。
      </p>
      <p v-else-if="method === 'member'" class="hint" style="margin: 0">
        {{ member.name || member.phone }} 余额 ¥{{ Number(member.balance).toFixed(2) }}，
        本单将扣 ¥{{ total.toFixed(2) }}，余额剩 ¥{{ (Number(member.balance) - total).toFixed(2) }}。
      </p>
      <p v-else class="hint" style="margin: 0">
        请确认顾客已用{{ label }}付清 ¥{{ total.toFixed(2) }}，然后点确认（系统只记账，不自动对账）。
      </p>

      <div class="modal-actions">
        <button @click="$emit('close')">取消 <span class="kbd">Esc</span></button>
        <button class="primary" :disabled="submitting || (method === 'cash' && change < 0)" @click="confirm">
          {{ submitting ? '出票中…' : '确认收款 · 自动出票' }} <span class="kbd">Enter</span>
        </button>
      </div>
      <p class="hint" style="margin: 0">
        现金需输入实收并自动找零；扫码为记账式，只记录方式与金额。
      </p>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'

const props = defineProps({
  total: { type: Number, required: true },
  submitting: { type: Boolean, default: false },
  member: { type: Object, default: null }
})
const emit = defineEmits(['confirm', 'close'])

const methods = computed(() => {
  const list = [
    { key: 'cash', label: '现金' },
    { key: 'wechat', label: '微信' },
    { key: 'alipay', label: '支付宝' }
  ]
  if (props.member) {
    list.push({ key: 'member', label: '会员余额' })
    // 100 积分 = 1 元；够本单才显示
    if (Number(props.member.points || 0) >= Math.round(props.total * 100)) {
      list.push({ key: 'points', label: '积分抵扣' })
    }
  }
  return list
})

const method = ref('cash')
const cash = ref(props.total)
const cashInput = ref(null)

const label = computed(() => (method.value === 'cash' ? '现金' : method.value === 'wechat' ? '微信' : '支付宝'))
const change = computed(() => Math.round((Number(cash.value || 0) - props.total) * 100) / 100)

const quicks = computed(() => {
  const base = [props.total, 20, 50, 100]
  return Array.from(new Set(base.map((n) => Math.round(n * 100) / 100))).filter((n) => n >= props.total)
})

function pick(key) {
  method.value = key
  if (key === 'cash') {
    cash.value = props.total
    nextTick(() => cashInput.value && cashInput.value.select())
  }
}

function confirm() {
  if (props.submitting) return
  if (method.value === 'cash' && change.value < 0) return
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
