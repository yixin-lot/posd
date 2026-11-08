<template>
  <section class="orders">
    <header class="orders-head">
      <h2>{{ tr('mem.title') }}</h2>
      <input class="products-search" v-model="keyword" :placeholder="tr('mem.searchPh')" @input="load" />
      <span class="grow"></span>
      <button @click="load">{{ tr('mem.refresh') }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="expiring.length && !expDismissed" class="member-expiring">
      <div class="exp-head">
        <button class="exp-toggle" @click="expOpen = !expOpen">
          {{ expOpen ? tr('mem.collapse') : tr('mem.expand') }} {{ tr('mem.expiring', { n: expiring.length }) }}
        </button>
        <button class="exp-close" :title="tr('mem.dismiss')" @click="dismissExpiring">×</button>
      </div>
      <div v-if="expOpen" class="exp-list">
        <div v-for="m in expiring" :key="m.id" class="exp-row">
          <span class="exp-name">{{ m.name || '—' }}</span>
          <span class="mono">{{ m.phone }}</span>
          <span>{{ tr('mem.points', { n: m.points }) }}</span>
          <span class="exp-date">{{ m.pointsExpireAt || '—' }}</span>
          <button class="exp-print" disabled :title="tr('mem.printHint')">{{ tr('mem.print') }}</button>
        </div>
      </div>
    </div>

    <div class="cols" style="gap: 14px; align-items: flex-start">
      <div class="panel" style="flex: 1.2">
        <table class="orders-table">
          <thead>
            <tr><th>{{ tr('mem.phone') }}</th><th>{{ tr('mem.name') }}</th><th class="num">{{ tr('mem.balance') }}</th><th class="num">{{ tr('mem.pointsLabel') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="m in list" :key="m.id" :class="{ selected: current && current.id === m.id, off: m.active === false }" @click="pick(m)">
              <td class="mono">{{ m.phone }}</td>
              <td>
                {{ m.name || '—' }} <span class="muted">{{ m.level || '' }}</span>
                <span v-if="m.active === false" class="member-off">{{ tr('mem.inactive') }}</span>
              </td>
              <td class="num">¥{{ Number(m.balance).toFixed(2) }}</td>
              <td class="num">{{ m.points }}</td>
            </tr>
            <tr v-if="!list.length"><td colspan="4" class="empty-row">{{ tr('mem.empty') }}</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel" style="flex: 1" v-if="current">
        <div class="panel-head">
          <h2>{{ current.name || current.phone }}</h2>
          <span v-if="current.active === false" class="member-off">{{ tr('mem.inactive') }}</span>
        </div>
        <div style="padding: 14px">
          <div class="member-balance">{{ tr('mem.balanceLine', { v: Number(current.balance).toFixed(2) }) }}</div>
          <div class="muted" style="margin-bottom: 10px">{{ tr('mem.pointsLine', { n: current.points, rate: redeemRate }) }}</div>

          <div class="member-topup">
            <input type="number" step="0.01" v-model.number="amount" :placeholder="tr('mem.amountPh')" />
            <button class="primary" :disabled="!amount || amount <= 0" @click="topup">{{ tr('mem.topup') }}</button>
          </div>

          <div style="display:flex; gap:8px; margin-top:10px">
            <button @click="toggleActive">{{ current.active ? tr('mem.disable') : tr('mem.enable') }}</button>
            <button class="danger" @click="removeMember">{{ tr('mem.delete') }}</button>
          </div>

          <h3 style="margin-top: 16px">{{ tr('mem.recent') }}</h3>
          <div class="member-txns">
            <div v-for="(t, i) in txns" :key="i" class="member-txn">
              <span>{{ kindText(t.kind) }} {{ t.kind.startsWith('points') ? tr('mem.points', { n: t.amount }) : '¥' + Number(t.amount).toFixed(2) }}</span>
              <span class="muted">{{ t.createdAt }}</span>
            </div>
            <div v-if="!txns.length" class="muted">{{ tr('mem.noTxns') }}</div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr } from '../i18n.js'

const list = ref([])
const keyword = ref('')
const current = ref(null)
const txns = ref([])
const amount = ref(0)
const error = ref('')
const expiring = ref([])
const expOpen = ref(false)
const expDismissed = ref(false)
const redeemRate = ref(100)

const EXP_SESSION_KEY = 'pos_expiring_dismissed'

function dismissExpiring() {
  expDismissed.value = true
  try {
    sessionStorage.setItem(EXP_SESSION_KEY, '1')
  } catch (e) { /* 隐私模式等场景忽略 */ }
}

const KIND = {
  topup: tr('mem.kindTopup'), consume: tr('mem.kindConsume'), refund: tr('mem.kindRefund'),
  points_earn: tr('mem.kindEarn'), points_use: tr('mem.kindUse'), points_refund: tr('mem.kindRefundPoints')
}
function kindText(k) { return KIND[k] || k }

async function load() {
  error.value = ''
  try {
    list.value = await api.listMembers(keyword.value)
  } catch (e) {
    error.value = tr('mem.loadFailed', { msg: e.message || '' })
  }
}

async function pick(m) {
  current.value = m
  try {
    txns.value = await api.memberTxns(m.id)
  } catch (e) {
    txns.value = []
  }
}

async function toggleActive() {
  try {
    current.value = await api.setMemberActive(current.value.id, !current.value.active)
    await load()
  } catch (e) {
    error.value = tr('mem.opFailed', { msg: e.message || '' })
  }
}

async function removeMember() {
  if (!window.confirm(tr('mem.deleteConfirm'))) return
  try {
    await api.deleteMember(current.value.id)
    current.value = null
    txns.value = []
    await load()
  } catch (e) {
    // 后端删除有流水的会员返回 409（message 复用了开卡文案），这里按状态给明确提示
    error.value = e.status === 409 ? tr('mem.deleteHasTxns') : tr('mem.deleteFailed', { msg: e.message || '' })
  }
}

async function topup() {
  try {
    const res = await api.topupMember(current.value.id, { amount: Number(amount.value), note: tr('mem.topupNote') })
    const m = res.member || res
    if (res.bonus > 0) error.value = tr('mem.topupBonus', { v: Number(res.bonus).toFixed(2) })
    current.value = m
    amount.value = 0
    txns.value = await api.memberTxns(m.id)
    await load()
  } catch (e) {
    error.value = tr('mem.topupFailed', { msg: e.message || '' })
  }
}

onMounted(async () => {
  await load()
  try {
    expDismissed.value = sessionStorage.getItem(EXP_SESSION_KEY) === '1'
  } catch (e) { /* 忽略 */ }
  if (!expDismissed.value) {
    try {
      expiring.value = await api.expiringMembers(7)
    } catch (e) { /* 不影响列表 */ }
  }
  try {
    const s = await api.getSettings()
    redeemRate.value = Number(s.points_per_yuan || 100)
  } catch (e) { /* 用默认值 */ }
})
</script>

<style scoped>
.member-balance { font-size: 22px; font-weight: 700; color: var(--money); }
.member-topup { display: flex; gap: 8px; }
.member-topup input { flex: 1; }
.member-txns { display: grid; gap: 6px; margin-top: 8px; }
.member-txn { display: flex; justify-content: space-between; font-size: 12px; }

/* {{ tr('mem.inactive') }}会员：整行灰色 + 标签 */
tr.off td {
  color: var(--muted);
  opacity: 0.6;
}

.member-off {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--muted);
  border: 1px solid var(--muted);
  vertical-align: 1px;
}

.panel-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
