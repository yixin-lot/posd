<template>
  <header class="statusbar">
    <span class="shop">{{ shopName }}</span>
    <span class="sep"></span>

    <nav class="tabs">
      <button
        v-for="t in primaryTabs"
        :key="t.view"
        :class="{ active: view === t.view }"
        @click="$emit('update:view', t.view)"
      >{{ t.label }}</button>
    </nav>

    <div v-if="menuTabs.length" class="more-nav" :class="{ 'menu-open': moreOpen }">
      <!-- 用 div+button 替代 details/summary：原生 details 的 open 语义与 v-show 手动状态
           混搭在部分场景下点击无响应（Bug：更多点不了），统一为受控状态 -->
      <button type="button" class="more-btn" @click="moreOpen = !moreOpen">更多</button>
      <div class="more-menu" v-show="moreOpen" @click="moreOpen = false">
        <button
          v-for="t in menuTabs"
          :key="t.view"
          :class="{ active: view === t.view }"
          @click="$emit('update:view', t.view)"
        >{{ t.label }}</button>
      </div>
    </div>

    <span class="sep"></span>
    <span class="sb-user">{{ userLabel }}</span>
    <span class="sep"></span>
    <span class="sb-shift">{{ shiftText }}</span>
    <span class="grow"></span>

    <span class="sb-service">
      <i class="dot" :class="serviceClass"></i>
      {{ serviceText }}
    </span>
    <span class="sb-printer">
      <i class="dot" :class="printerClass"></i>
      {{ printerText }}
    </span>
    <!-- v03 状态条分层：今日单量琥珀 + 待重打红色 badge（保留 .sb-health 类以便移动端统一隐藏） -->
    <span v-if="health" class="sb-health sb-today"><span class="num">{{ health.ordersToday }}</span> 单</span>
    <span v-if="health && health.queue && health.queue.queued" class="sb-health sb-queue">
      待重打 <span class="badge">{{ health.queue.queued }}</span>
    </span>
    <button class="logout" @click="$emit('logout')">退出</button>
  </header>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps({
  view: { type: String, default: 'cashier' },
  health: { type: Object, default: null },
  error: { type: String, default: '' },
  user: { type: Object, default: null }
})
const role = computed(() => (props.user ? props.user.role : 'cashier'))
defineEmits(['update:view', 'logout'])

// UI 优化：页签收敛——高频 3 个常驻（收银/订单/桌台），其余进「更多」下拉，
// 避免 10 个页签在窄/矮视口下换行重叠（修复截图里的双排导航）。
const navTabs = [
  { label: '收银', view: 'cashier' },
  { label: '订单', view: 'orders' },
  { label: '桌台', view: 'tables' },
  { label: '商品', view: 'products', manager: true },
  { label: '班次', view: 'shift' },
  { label: '会员', view: 'members' },
  { label: '报表', view: 'reports', manager: true },
  { label: '审计', view: 'audit', manager: true },
  { label: '员工', view: 'staff', manager: true },
  { label: '设置', view: 'settings', manager: true }
]
// v-if 与 v-for 同元素在 Vue 3 会运行时炸（v-if 先评估拿不到 v-for 的 t），
// 这里统一过滤后再渲染。
const roleVisible = computed(() => navTabs.filter((t) => !t.manager || role.value === 'manager'))
const primaryTabs = computed(() => roleVisible.value.slice(0, 3))
const moreTabs = computed(() => roleVisible.value.slice(3))

// 移动端（tabs 隐藏）时「更多」要能进全部页签；桌面端只收收纳的次要页签。
const mq = window.matchMedia('(max-width: 430px)')
const isNarrow = ref(mq.matches)
function onMq(e) {
  isNarrow.value = e.matches
}
onMounted(() => mq.addEventListener('change', onMq))
onUnmounted(() => mq.removeEventListener('change', onMq))
const menuTabs = computed(() => (isNarrow.value ? roleVisible.value : moreTabs.value))

const moreOpen = ref(false)
// 切页签后自动收起「更多」
function closeMore() {
  moreOpen.value = false
}
watch(() => props.view, closeMore)

const serviceClass = computed(() => (props.error ? 'bad' : props.health ? 'ok' : 'warn'))
const serviceText = computed(() => (props.error ? '收银服务未连接' : props.health ? '收银服务在线' : '连接中'))

const printerClass = computed(() => {
  if (!props.health) return 'warn'
  return props.health.agent && props.health.agent.reachable ? 'ok' : 'bad'
})
const printerText = computed(() => {
  if (!props.health) return '打印状态未知'
  // 单机版出票在进程内，没有「外部出票助手」这个概念。
  if (props.health.mode === 'standalone') {
    return props.health.agent && props.health.agent.reachable ? '打印就绪' : '打印异常'
  }
  return props.health.agent && props.health.agent.reachable ? '出票助手在线' : '出票助手离线'
})

const shopName = computed(() => {
  const s = props.health && props.health.shop
  return (s && s.name) || '收银台'
})

const userLabel = computed(() => {
  if (!props.user) return '未登录'
  const role = props.user.role === 'manager' ? '店长' : '收银员'
  return props.user.name === role ? role : props.user.name + '（' + role + '）'
})

const shiftText = computed(() => {
  const s = props.health && props.health.shift
  return s ? '班次 ' + s.shiftNo : '未开班'
})
</script>
