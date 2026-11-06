<template>
  <div v-if="open" class="mask" @click.self="$emit('close')">
    <div class="modal member-modal">
      <h3>会员</h3>

      <div class="member-search">
        <input
          v-model="phone"
          maxlength="11"
          pattern="[0-9]{11}"
          title="请输入 11 位手机号"
          placeholder="输入 11 位手机号查会员"
          @keyup.enter="search"
        />
        <button class="primary" @click="search">查询</button>
      </div>
      <p class="phone-hint">手机号需为 11 位数字</p>

      <p v-if="error" class="orders-error">{{ error }}</p>

      <div v-if="member" class="member-card">
        <div class="member-row">
          <b>{{ member.name || '未命名会员' }}</b>
          <span class="muted">{{ member.phone }} · {{ member.level || '普通会员' }}</span>
        </div>
        <div class="member-balance">余额 ¥{{ Number(member.balance).toFixed(2) }}</div>
        <div class="muted" style="margin-bottom: 8px">积分 {{ member.points || 0 }}（100 积分 = 1 元）</div>

        <div class="member-topup">
          <input type="number" step="0.01" v-model.number="amount" placeholder="充值金额" />
          <button class="primary" :disabled="!amount || amount <= 0" @click="topup">充值</button>
        </div>

        <button class="member-use" @click="use">用这位会员结算</button>

        <div v-if="txns.length" class="member-txns">
          <div v-for="(t, i) in txns" :key="i" class="member-txn">
            <span>{{ t.kind === 'topup' ? '充值' : '消费' }} ¥{{ Number(t.amount).toFixed(2) }}</span>
            <span class="muted">余额 ¥{{ Number(t.balanceAfter).toFixed(2) }} · {{ t.createdAt }}</span>
          </div>
        </div>
      </div>

      <div v-else-if="searched && !member" class="member-empty">
        <p>该手机号还没开卡。</p>
        <div class="member-search">
          <input v-model="newName" maxlength="20" placeholder="会员称呼（可选）" />
          <button class="primary" @click="create">开卡</button>
        </div>
      </div>

      <div class="modal-actions">
        <button @click="$emit('close')">关闭</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api } from '../api/index.js'

const props = defineProps({ open: { type: Boolean, default: false }, current: { type: Object, default: null } })
const emit = defineEmits(['close', 'picked'])

const phone = ref(props.current ? props.current.phone : '')
const newName = ref('')
const member = ref(props.current)
const txns = ref([])
const searched = ref(false)
const amount = ref(0)
const error = ref('')

function validPhone(p) {
  return /^1\d{10}$/.test(p)
}

async function search() {
  error.value = ''
  member.value = null
  txns.value = []
  const p = phone.value.trim()
  if (!p) return
  if (!validPhone(p)) {
    error.value = '请输入 11 位手机号'
    return
  }
  try {
    member.value = await api.findMember(p)
    searched.value = true
    if (member.value) txns.value = await api.memberTxns(member.value.id)
  } catch (e) {
    searched.value = true
    if (e.status !== 404) error.value = '查询失败：' + (e.message || '未知错误')
  }
}

async function create() {
  error.value = ''
  if (!validPhone(phone.value.trim())) {
    error.value = '请输入 11 位手机号'
    return
  }
  try {
    member.value = await api.createMember({ phone: phone.value.trim(), name: newName.value.trim() })
    txns.value = []
  } catch (e) {
    error.value = '开卡失败：' + (e.message || '未知错误')
  }
}

async function topup() {
  error.value = ''
  try {
    const res = await api.topupMember(member.value.id, { amount: Number(amount.value), note: '现金充值' })
    member.value = res.member || res
    if (res.bonus > 0) toast('充值成功，另赠 ¥' + Number(res.bonus).toFixed(2))
    amount.value = 0
    txns.value = await api.memberTxns(member.value.id)
  } catch (e) {
    error.value = '充值失败：' + (e.message || '未知错误')
  }
}

function use() {
  emit('picked', member.value)
  emit('close')
}
</script>

<style scoped>
.member-modal { width: min(560px, 100%); }
.member-search { display: flex; gap: 8px; margin: 12px 0; }
.member-search input { flex: 1; }
.phone-hint { margin: -6px 0 0; font-size: 12px; color: var(--muted); }
.member-card { background: var(--panel-2); border: 1px solid var(--line); border-radius: 10px; padding: 12px; }
.member-row { display: flex; gap: 10px; align-items: baseline; }
.member-balance { font-size: 22px; font-weight: 700; color: var(--money); margin: 6px 0 10px; }
.member-topup { display: flex; gap: 8px; }
.member-topup input { flex: 1; }
.member-use { width: 100%; margin-top: 10px; background: var(--brand); border-color: var(--brand); color: #fff; padding: 9px; border-radius: 8px; }
.member-txns { margin-top: 12px; display: grid; gap: 6px; }
.member-txn { display: flex; justify-content: space-between; font-size: 12px; }
.member-empty p { margin: 8px 0; }
</style>
