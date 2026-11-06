<template>
  <header class="statusbar" :class="{ offline: isOffline, manage: view !== 'cashier' }">
    <!-- v03 状态条左段：门店 | 收银员 | 班次（离线时整体转琥珀 + 左侧竖条） -->
    <div class="sb-left">
      <b class="shop">{{ shopName }}</b>
      <span class="sep"></span>
      <span class="sb-user">{{ userLabel }}</span>
      <span class="sep"></span>
      <span class="sb-shift">{{ shiftText }}</span>

      <!-- v03 P8 离线态：琥珀状态横幅 -->
      <span v-if="isOffline" class="sb-offline-pill">离线模式 · 数据稍后自动同步</span>
    </div>

    <div class="sb-right">
      <!-- 今日已收只属于收银台（老板/收银员一眼看数），管理页不显示 -->
      <span v-if="view === 'cashier' && health" class="sb-today">
        今日已收 <b class="sb-today-amt">¥{{ salesText }}</b>
      </span>

      <span class="sb-service">
        <i class="dot" :class="serviceClass"></i>
        {{ serviceText }}
      </span>
      <span class="sb-printer">
        <i class="dot" :class="printerClass"></i>
        {{ printerText }}
      </span>

      <span v-if="queued" class="sb-queue">待重打 <span class="badge">{{ queued }}</span></span>

      <span v-if="view === 'cashier'" class="sb-sync"><i class="dot sync"></i>同步 {{ syncText }}</span>

      <!-- 右上「菜单」是收银台的快捷跳转：低频操作全收进抽屉，管理页另有左侧 nav64 -->
      <div v-if="view === 'cashier'" class="more-nav" :class="{ 'menu-open': moreOpen }">
        <button type="button" class="more-btn" @click="moreOpen = !moreOpen">☰ 菜单</button>
      </div>

      <!-- v03 规则三：退出登录是全局动作，常驻顶栏右侧，10 个页面都可见，不进任何页面菜单 -->
      <button type="button" class="sb-logout" title="退出登录" @click="$emit('logout')">
        <span class="lo-full">退出登录</span><span class="lo-short">退出</span>
      </button>
    </div>

    <!-- v03 menu-drawer：全屏遮罩 + 右侧 360px 全高深色面板。
         挂到 body，避免被状态条的 overflow 裁掉、也不受顶层 z-index 影响。 -->
    <Teleport to="body">
      <div v-if="moreOpen" class="menu-drawer" @click.self="moreOpen = false">
        <aside class="md-panel">
          <div class="md-head">
            <h4>菜单</h4>
            <button type="button" class="md-close" @click="moreOpen = false">关闭</button>
          </div>
          <p class="md-sub">收钱、卖货都在主屏完成；查单、管商品、交接班才进这里。</p>
          <div class="md-list">
            <button
              v-for="t in drawerItems"
              :key="t.view"
              type="button"
              class="mi"
              @click="go(t.view)"
            >
              <span class="mi-icon" :class="t.tone">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path v-for="(d, i) in t.icon" :key="i" :d="d" /></svg>
              </span>
              <span class="mi-text">
                <b>{{ t.menu }}</b>
                <i>{{ t.desc }}</i>
              </span>
            </button>
          </div>
          <div class="md-foot">
            <span class="md-user">{{ userLabel }}</span>
            <span class="md-hint">退出登录在顶栏右上，任何页面都可直接退出</span>
          </div>
        </aside>
      </div>
    </Teleport>

    <!-- 旧页签保留为隐藏的导航源：脚本与快捷键仍按这一套口径切页，
         视觉导航由管理页左侧 nav64 + 收银台 ☰ 菜单承担（styles.css 里 .tabs{display:none}）。 -->
    <nav class="tabs" aria-hidden="true">
      <button
        v-for="t in primaryTabs"
        :key="t.view"
        :class="{ active: view === t.view }"
        @click="$emit('update:view', t.view)"
      >{{ t.short }}</button>
    </nav>
  </header>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { navItems } from '../nav.js'

const props = defineProps({
  view: { type: String, default: 'cashier' },
  health: { type: Object, default: null },
  error: { type: String, default: '' },
  user: { type: Object, default: null }
})
const role = computed(() => (props.user ? props.user.role : 'cashier'))
const emit = defineEmits(['update:view', 'logout'])

// 导航清单的唯一来源是 nav.js：管理页左侧 nav64 与 ☰ 抽屉都从这里取，按角色过滤。
// 页面里不再各写一份清单（两处硬编码必然漂移）。
// menu / desc / tone / icon 按 v03 menu-drawer 的卡片写法：44px 图标块 + 16px 标题 + 12px 副行。
const roleVisible = computed(() => navItems(role.value))
const primaryTabs = computed(() => roleVisible.value.slice(0, 3))
const drawerItems = computed(() => roleVisible.value)

const moreOpen = ref(false)
function go(v) {
  moreOpen.value = false
  emit('update:view', v)
}
watch(() => props.view, () => {
  moreOpen.value = false
})

const queued = computed(() => (props.health && props.health.queue ? props.health.queue.queued : 0))

const salesText = computed(() => {
  const v = props.health && props.health.salesToday
  if (!v) return '0.00'
  // 后端金额单位为分（与订单表一致），状态条按元显示
  return (Number(v) / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
})

const syncText = computed(() => (props.health && props.health.syncedAt) || '--:--')

// v03 P8：离线 = 连不上收银服务。左侧琥珀竖条 + 店名转琥珀 + 状态横幅 + ○离线
const isOffline = computed(() => !props.health && !!props.error)

const serviceClass = computed(() => (isOffline.value ? 'off' : props.error ? 'bad' : props.health ? 'ok' : 'warn'))
const serviceText = computed(() => (isOffline.value ? '离线' : props.error ? '收银服务未连接' : props.health ? '在线' : '连接中'))

// 打印输出随模式切文案：本地直连 / 云打印，异常时红点 + 「云打印 · 异常」
const cloudMode = computed(() => !!(props.health && props.health.printMode === 'cloud'))
const printerClass = computed(() => {
  if (!props.health) return 'warn'
  return props.health.agent && props.health.agent.reachable ? 'ok' : 'bad'
})
const printerText = computed(() => {
  if (!props.health) return '打印状态未知'
  const ok = props.health.agent && props.health.agent.reachable
  if (cloudMode.value) return ok ? '打印 · 云打印就绪' : '打印 · 云打印异常'
  if (props.health.mode === 'standalone') return ok ? '打印 · 本地就绪' : '打印 · 本地异常'
  return ok ? '打印 · 出票助手在线' : '打印 · 出票助手离线'
})

const shopName = computed(() => {
  const s = props.health && props.health.shop
  return (s && s.name) || '收银台'
})

const userLabel = computed(() => {
  if (!props.user) return '未登录'
  const r = props.user.role === 'manager' ? '店长' : '收银员'
  return props.user.name === r ? r : props.user.name + ' · ' + r
})

const shiftText = computed(() => {
  const s = props.health && props.health.shift
  return s ? '班次 ' + s.shiftNo : '未开班'
})
</script>
