<template>
  <div v-if="open" class="mask" @click.self="$emit('close')">
    <div class="modal member-modal">
      <h3>{{ tr('mem2.title') }}</h3>

      <div class="member-search">
        <input
          v-model="phone"
          maxlength="11"
          pattern="[0-9]{11}"
          :title="tr('mem2.phoneTitle')"
          :placeholder="tr('mem2.phonePh')"
          @keyup.enter="search"
        />
        <button class="primary" @click="search">{{ tr('mem2.search') }}</button>
      </div>
      <p class="phone-hint">{{ tr('mem2.phoneHint') }}</p>

      <p v-if="error" class="orders-error">{{ error }}</p>

      <div v-if="member" class="member-card">
        <div class="member-row">
          <b>{{ member.name || tr('mem2.unnamed') }}</b>
          <span class="muted">{{ member.phone }} · {{ member.level || tr('mem2.level') }}</span>
        </div>
        <div class="member-balance">{{ tr('mem.balanceLine', { v: Number(member.balance).toFixed(2) }) }}</div>
        <div class="muted" style="margin-bottom: 8px">{{ tr('mem2.pointsLine', { n: member.points || 0 }) }}</div>

        <div class="member-topup">
          <input type="number" step="0.01" v-model.number="amount" :placeholder="tr('mem.amountPh')" />
          <button class="primary" :disabled="!amount || amount <= 0" @click="topup">{{ tr('mem.topup') }}</button>
        </div>

        <button class="member-use" @click="use">{{ tr('mem2.use') }}</button>

        <div v-if="txns.length" class="member-txns">
          <div v-for="(t, i) in txns" :key="i" class="member-txn">
            <span>{{ t.kind === 'topup' ? tr('mem.kindTopup') : tr('mem.kindConsume') }} ¥{{ Number(t.amount).toFixed(2) }}</span>
            <span class="muted">{{ tr('mem2.txnBalance', { v: Number(t.balanceAfter).toFixed(2), at: t.createdAt }) }}</span>
          </div>
        </div>
      </div>

      <div v-else-if="searched && !member" class="member-empty">
        <p>{{ tr('mem2.notFound') }}</p>
        <div class="member-search">
          <input v-model="newName" maxlength="20" :placeholder="tr('mem2.namePh')" />
          <button class="primary" @click="create">{{ tr('mem2.create') }}</button>
        </div>
      </div>

      <div class="modal-actions">
        <button @click="$emit('close')">{{ tr('mem2.close') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr } from '../i18n.js'

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
    error.value = tr('mem2.phoneError')
    return
  }
  try {
    member.value = await api.findMember(p)
    searched.value = true
    if (member.value) txns.value = await api.memberTxns(member.value.id)
  } catch (e) {
    searched.value = true
    if (e.status !== 404) error.value = tr('mem2.searchFailed', { msg: e.message || '' })
  }
}

async function create() {
  error.value = ''
  if (!validPhone(phone.value.trim())) {
    error.value = tr('mem2.phoneError')
    return
  }
  try {
    member.value = await api.createMember({ phone: phone.value.trim(), name: newName.value.trim() })
    txns.value = []
  } catch (e) {
    error.value = tr('mem2.createFailed', { msg: e.message || '' })
  }
}

async function topup() {
  error.value = ''
  try {
    const res = await api.topupMember(member.value.id, { amount: Number(amount.value), note: tr('mem2.topupNote') })
    member.value = res.member || res
    if (res.bonus > 0) toast(tr('mem.topupBonus', { v: Number(res.bonus).toFixed(2) }))
    amount.value = 0
    txns.value = await api.memberTxns(member.value.id)
  } catch (e) {
    error.value = tr('mem.topupFailed', { msg: e.message || '' })
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
