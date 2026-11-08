<template>
  <div v-if="demo" class="demo-banner">
    <div class="demo-bar">
      <span class="demo-badge">{{ tr('demo.badge') }}</span>
      <span class="demo-text">{{ tr('demo.text') }}</span>
      <span class="demo-spacer"></span>
      <button class="demo-btn" @click="toggle">{{ tr('demo.view') }}</button>
      <button class="demo-btn primary" @click="startSetup">{{ tr('demo.setup') }}</button>
    </div>

    <div v-if="open" class="demo-panel">
      <p class="demo-title">{{ tr('demo.panelTitle') }}</p>
      <div class="demo-orders">
        <div v-for="o in demoOrders" :key="o.no" class="demo-order">
          <div class="demo-row">
            <span class="demo-no">{{ o.no }}</span>
            <span class="demo-time">{{ o.time }}</span>
          </div>
          <div class="demo-row">
            <span>{{ o.itemsText }}</span>
            <span class="demo-money">¥{{ o.total.toFixed(2) }}</span>
          </div>
          <div class="demo-row">
            <span class="demo-muted">{{ o.pay }}</span>
            <span class="demo-chip">{{ o.status }}</span>
          </div>
        </div>
      </div>
      <p class="demo-note">{{ tr('demo.note') }}</p>
      <button class="demo-btn danger" @click="clearDemo">{{ tr('demo.clear') }}</button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { request } from '../lib/http.js'
import { t as tr, locale } from '../i18n.js'

// setup_done=false（未完成首次向导）且本机未手动清空示例（localStorage 无标记）即演示模式。
// 「开始配置」打开首次向导（pos:setup 事件）；向导完成（setup_done=true）后角标消失且不再出现。
// 示例订单不落库（前端 mock），避免污染真实营业数据。
const KEY = 'pos_demo_seen'
const dismissed = ref(!!localStorage.getItem(KEY))
const setupDone = ref(false)
const demo = computed(() => !setupDone.value && !dismissed.value)
const open = ref(false)

async function refreshSetup() {
  try {
    const s = await request('/pos/v1/settings')
    setupDone.value = (s && s.setup_done) === 'true'
  } catch (e) {
    // 拉不到 settings 时维持当前演示态
  }
}

function onSetupDone() {
  refreshSetup()
}

onMounted(() => {
  refreshSetup()
  window.addEventListener('pos:setup-done', onSetupDone)
})

onUnmounted(() => {
  window.removeEventListener('pos:setup-done', onSetupDone)
})

const demoOrders = computed(() => locale.value === 'zh-CN' ? [
  {
    no: 'DEMO-0001',
    time: '今天 09:12',
    itemsText: '珍珠奶茶 ×2 · 椰果奶茶 ×1',
    total: 38,
    pay: '现金 50 找零 12',
    status: '已出票'
  },
  {
    no: 'DEMO-0002',
    time: '今天 09:30',
    itemsText: '杨枝甘露 ×1 · 海盐芝士 ×1',
    total: 36,
    pay: '扫码支付',
    status: '已出票'
  },
  {
    no: 'DEMO-0003',
    time: '今天 10:02',
    itemsText: '生椰拿铁 ×2 · 蛋挞 ×2',
    total: 46,
    pay: '现金 50 找零 4',
    status: '已出票'
  }
] : [
  { no: 'D-001', time: 'Today 09:12', itemsText: 'Pearl Milk Tea ×2 · Coconut Milk Tea ×1', total: '¥38.00', pay: 'Cash 50, change 12', status: 'Printed' },
  { no: 'D-002', time: 'Today 09:30', itemsText: 'Mango Pomelo ×1 · Sea Salt Cheese ×1', total: '¥36.00', pay: 'Scan to pay', status: 'Printed' },
  { no: 'D-003', time: 'Today 10:02', itemsText: 'Coconut Latte ×2 · Egg Tart ×2', total: '¥42.00', pay: 'Cash 50, change 4', status: 'Printed' }
])

function toggle() {
  open.value = !open.value
}

function clearDemo() {
  if (!window.confirm(tr('demo.confirm'))) return
  localStorage.setItem(KEY, '1')
  dismissed.value = true
}

function startSetup() {
  window.dispatchEvent(new CustomEvent('pos:setup'))
}
</script>

<style scoped>
.demo-banner {
  border-bottom: 1px solid var(--line);
  background: var(--panel);
}

.demo-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 18px;
  font-size: 13px;
}

.demo-badge {
  background: var(--money-soft);
  color: var(--money);
  border: 1px solid var(--money);
  border-radius: 4px;
  padding: 2px 8px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

.demo-text {
  color: var(--muted);
}

.demo-spacer {
  flex: 1;
}

.demo-btn {
  padding: 4px 12px;
  font-size: 12px;
  border-radius: 6px;
}

.demo-btn.primary {
  border-color: var(--brand);
  color: var(--brand);
}

.demo-btn.danger {
  border-color: var(--danger);
  color: var(--danger);
  margin-top: 4px;
}

.demo-panel {
  padding: 14px 18px;
  border-top: 1px solid var(--line);
  background: var(--panel-2);
}

.demo-title {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--text);
}

.demo-orders {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.demo-order {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 10px;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.demo-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.demo-no {
  font-weight: 700;
  color: var(--text);
}

.demo-time {
  color: var(--muted);
}

.demo-money {
  color: var(--money);
  font-weight: 700;
}

.demo-muted {
  color: var(--muted);
}

.demo-chip {
  border: 1px solid var(--ok);
  color: var(--ok);
  border-radius: 4px;
  padding: 0 6px;
  font-size: 11px;
}

.demo-note {
  margin: 10px 0 4px;
  color: var(--muted);
  font-size: 12px;
}
.demo-btn.primary {
  color: #fff;
}
</style>
