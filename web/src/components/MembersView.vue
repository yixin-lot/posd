<template>
  <section class="orders">
    <header class="orders-head">
      <h2>会员</h2>
      <input class="products-search" v-model="keyword" placeholder="手机号或姓名" @input="load" />
      <span class="grow"></span>
      <button @click="load">刷新</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="expiring.length && !expDismissed" class="member-expiring">
      <div class="exp-head">
        <button class="exp-toggle" @click="expOpen = !expOpen">
          {{ expOpen ? '收起' : '展开' }} 积分到期提醒（{{ expiring.length }} 位会员 7 天内到期）
        </button>
        <button class="exp-close" title="本次会话内不再提醒" @click="dismissExpiring">×</button>
      </div>
      <div v-if="expOpen" class="exp-list">
        <div v-for="m in expiring" :key="m.id" class="exp-row">
          <span class="exp-name">{{ m.name || '—' }}</span>
          <span class="mono">{{ m.phone }}</span>
          <span>{{ m.points }} 分</span>
          <span class="exp-date">{{ m.pointsExpireAt || '—' }}</span>
          <button class="exp-print" disabled title="下次结账自动打印">打印提醒</button>
        </div>
      </div>
    </div>

    <div class="cols" style="gap: 14px; align-items: flex-start">
      <div class="panel" style="flex: 1.2">
        <table class="orders-table">
          <thead>
            <tr><th>手机号</th><th>姓名</th><th class="num">余额</th><th class="num">积分</th></tr>
          </thead>
          <tbody>
            <tr v-for="m in list" :key="m.id" :class="{ selected: current && current.id === m.id, off: m.active === false }" @click="pick(m)">
              <td class="mono">{{ m.phone }}</td>
              <td>
                {{ m.name || '—' }} <span class="muted">{{ m.level || '' }}</span>
                <span v-if="m.active === false" class="member-off">已停用</span>
              </td>
              <td class="num">¥{{ Number(m.balance).toFixed(2) }}</td>
              <td class="num">{{ m.points }}</td>
            </tr>
            <tr v-if="!list.length"><td colspan="4" class="empty-row">没有会员</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel" style="flex: 1" v-if="current">
        <div class="panel-head">
          <h2>{{ current.name || current.phone }}</h2>
          <span v-if="current.active === false" class="member-off">已停用</span>
        </div>
        <div style="padding: 14px">
          <div class="member-balance">余额 ¥{{ Number(current.balance).toFixed(2) }}</div>
          <div class="muted" style="margin-bottom: 10px">积分 {{ current.points }}（{{ redeemRate }} 分抵 1 元）</div>

          <div class="member-topup">
            <input type="number" step="0.01" v-model.number="amount" placeholder="充值金额" />
            <button class="primary" :disabled="!amount || amount <= 0" @click="topup">充值</button>
          </div>

          <div style="display:flex; gap:8px; margin-top:10px">
            <button @click="toggleActive">{{ current.active ? '停用会员' : '启用会员' }}</button>
            <button class="danger" @click="removeMember">删除</button>
          </div>

          <h3 style="margin-top: 16px">最近流水</h3>
          <div class="member-txns">
            <div v-for="(t, i) in txns" :key="i" class="member-txn">
              <span>{{ kindText(t.kind) }} {{ t.kind.startsWith('points') ? t.amount + ' 分' : '¥' + Number(t.amount).toFixed(2) }}</span>
              <span class="muted">{{ t.createdAt }}</span>
            </div>
            <div v-if="!txns.length" class="muted">暂无流水</div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'

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
  topup: '充值', consume: '消费', refund: '退款',
  points_earn: '积分发放', points_use: '积分核销', points_refund: '积分退回'
}
function kindText(k) { return KIND[k] || k }

async function load() {
  error.value = ''
  try {
    list.value = await api.listMembers(keyword.value)
  } catch (e) {
    error.value = '读取失败：' + (e.message || '未知错误')
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
    error.value = '操作失败：' + (e.message || '未知错误')
  }
}

async function removeMember() {
  if (!window.confirm('删除后不可恢复；有流水的会员会被拒绝。确认删除？')) return
  try {
    await api.deleteMember(current.value.id)
    current.value = null
    txns.value = []
    await load()
  } catch (e) {
    // 后端删除有流水的会员返回 409（message 复用了开卡文案），这里按状态给明确提示
    error.value = e.status === 409 ? '删除失败：有流水的会员只能停用' : '删除失败：' + (e.message || '未知错误')
  }
}

async function topup() {
  try {
    const res = await api.topupMember(current.value.id, { amount: Number(amount.value), note: '会员中心充值' })
    const m = res.member || res
    if (res.bonus > 0) error.value = '充值成功，另赠 ¥' + Number(res.bonus).toFixed(2)
    current.value = m
    amount.value = 0
    txns.value = await api.memberTxns(m.id)
    await load()
  } catch (e) {
    error.value = '充值失败：' + (e.message || '未知错误')
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

/* 已停用会员：整行灰色 + 标签 */
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
