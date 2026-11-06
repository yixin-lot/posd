<template>
  <section class="orders">
    <header class="orders-head">
      <h2>审计</h2>
      <select v-model="filter" @change="applyFilter" class="date-input">
        <option value="">全部动作</option>
        <option v-for="a in ACTIONS" :key="a.value" :value="a.value">{{ a.label }}</option>
      </select>
      <span class="muted">共 {{ total }} 条 · 第 {{ page + 1 }} 页 · 设置 / 改价 / 作废等关键操作</span>
      <span class="grow"></span>
      <button @click="load" :disabled="loading">{{ loading ? '读取中…' : '刷新' }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div class="shift-card">
      <table class="orders-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>动作</th>
            <th>操作人</th>
            <th>详情</th>
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
            <td colspan="4" class="empty-row">没有审计记录</td>
          </tr>
        </tbody>
      </table>

    <div class="audit-pager">
      <button :disabled="page === 0" @click="prev">上一页</button>
      <span class="muted">{{ page + 1 }} / {{ Math.max(1, Math.ceil(total / pageSize)) }}</span>
      <button :disabled="(page + 1) * pageSize >= total" @click="nextPage">下一页</button>
    </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'

const loading = ref(false)
const error = ref('')
const filter = ref('')
const all = ref([])
const total = ref(0)
const page = ref(0)
const pageSize = 20

const ACTIONS = [
  { value: 'setting_change', label: '改设置' },
  { value: 'price_override', label: '改价' },
  { value: 'void', label: '作废' }
]
const LABELS = { setting_change: '改设置', price_override: '改价', void: '作废' }

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
    error.value = '读取失败：' + (e.message || '未知错误')
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
