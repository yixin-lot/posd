<template>
  <section class="orders">
    <header class="orders-head">
      <h2>{{ tr('audit.title') }}</h2>
      <select v-model="filter" @change="applyFilter" class="date-input">
        <option value="">{{ tr('audit.allActions') }}</option>
        <option v-for="a in ACTIONS" :key="a.value" :value="a.value">{{ a.label }}</option>
      </select>
      <span class="muted">{{ tr('audit.summary', { total, page: page + 1 }) }}</span>
      <span class="grow"></span>
      <button @click="load" :disabled="loading">{{ loading ? tr('audit.loading') : tr('audit.refresh') }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div class="shift-card">
      <table class="orders-table">
        <thead>
          <tr>
            <th>{{ tr('audit.time') }}</th>
            <th>{{ tr('audit.action') }}</th>
            <th>{{ tr('audit.operator') }}</th>
            <th>{{ tr('audit.detail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(a, i) in rows" :key="i">
            <td class="mono">{{ a.createdAt }}</td>
            <td><span class="audit-tag" :class="'t-' + a.action">{{ actionLabel(a.action) }}</span></td>
            <td>{{ a.operator }}</td>
            <td class="audit-detail">{{ detailText(a) }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="4" class="empty-row">{{ tr('audit.empty') }}</td>
          </tr>
        </tbody>
      </table>

    <div class="audit-pager">
      <button :disabled="page === 0" @click="prev">{{ tr('audit.prev') }}</button>
      <span class="muted">{{ page + 1 }} / {{ Math.max(1, Math.ceil(total / pageSize)) }}</span>
      <button :disabled="(page + 1) * pageSize >= total" @click="nextPage">{{ tr('audit.next') }}</button>
    </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr } from '../i18n.js'

const loading = ref(false)
const error = ref('')
const filter = ref('')
const all = ref([])
const total = ref(0)
const page = ref(0)
const pageSize = 20

const ACTIONS = [
  { value: 'setting_change', label: 'audit.actSetting' },
  { value: 'price_override', label: 'audit.actPrice' },
  { value: 'void', label: 'audit.actVoid' }
]
const LABELS = { setting_change: tr('audit.actSetting'), price_override: tr('audit.actPrice'), void: tr('audit.actVoid') }

function actionLabel(a) {
  return LABELS[a] || a || '—'
}

const rows = ref([])

function applyFilter() {
  rows.value = filter.value ? all.value.filter((a) => a.action === filter.value) : all.value
}

function detailText(a) {
  // 有单号时把单号放在详情前，便于追溯
  const orderPart = a.orderNo ? a.orderNo + ' ' : ''
  return orderPart + (a.detail || '')
}

function prev() {
  if (page.value > 0) { page.value--; load() }
}

function nextPage() {
  if ((page.value + 1) * pageSize < total.value) { page.value++; load() }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.auditLogs(pageSize, page * pageSize)
  all.value = Array.isArray(res) ? res : (res && res.items) || []
  total.value = Array.isArray(res) ? all.value.length : (res && res.total) || all.value.length
    applyFilter()
  } catch (e) {
    error.value = tr('audit.loadFailed', { msg: e.message || '' })
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.audit-tag {
  display: inline-block;
  padding: 2px 9px;
  border-radius: 999px;
  font-size: 12px;
  border: 1px solid var(--line);
  color: var(--muted);
  background: var(--panel-2);
}

.audit-tag.t-setting_change {
  color: var(--brand);
  border-color: var(--brand);
}

.audit-tag.t-price_override {
  color: var(--money);
  border-color: var(--money);
}

.audit-tag.t-void {
  color: var(--danger);
  border-color: var(--danger);
}

.audit-detail {
  color: var(--text);
  word-break: break-all;
}
</style>
