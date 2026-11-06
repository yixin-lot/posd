<template>
  <LoginView v-if="!user" @success="onLogin" />

  <div v-else class="app" :class="view === 'cashier' ? 'cashier-mode' : 'manage-mode'">
    <!-- v03：收银台是专注模式，全屏无侧栏；管理页才有 64px 图标侧栏 -->
    <NavRail v-if="view !== 'cashier'" v-model:view="view" :role="user.role" />
    <div class="shell-main">
    <StatusBar v-model:view="view" :health="health" :error="agentError" :user="user" @logout="onLogout" />
    <DemoBanner />
    <OnboardingWizard :role="user.role" />
    <TemplatePicker :role="user.role" v-model:open="tplOpen" />

    <div v-if="view === 'cashier' && stockAlerts.length" class="held-bar stock-bar">
      <span class="held-title">库存偏低 {{ stockAlerts.length }} 项</span>
      <button class="held-item" @click="view = 'products'">
        {{ stockAlerts.slice(0, 4).map((s) => s.name + ' ' + s.qty).join('、') }}
        <span class="held-go">去补货</span>
      </button>
    </div>

    <div v-if="view === 'cashier' && heldOrders.length" class="held-bar">
      <span class="held-title">挂单 {{ heldOrders.length }} 笔</span>
      <button v-for="h in heldOrders" :key="h.orderNo" class="held-item" @click="startPayHeld(h)">
        {{ h.orderNo }} · ¥{{ Number(h.payable).toFixed(2) }}
        <span class="held-go">收款</span>
      </button>
    </div>

    <main v-if="view === 'cashier'" class="cols">
      <ProductGrid
        ref="grid"
        :products="filtered"
        :categories="categories"
        v-model:active="activeCat"
        v-model:keyword="keyword"
        :all="products"
        @add="add"
      />

      <CartPanel
        :items="cart"
        :held-count="heldOrders.length"
        :class="{ open: cartOpen }"
        @inc="inc"
        @dec="dec"
        @remove="remove"
        @clear="clear"
        @reprice="reprice"
        @hold="holdOrder"
        @pickup="heldOrders.length && startPayHeld(heldOrders[0])"
        @discount="memberOpen = true"
      />

      <aside class="panel">
        <div class="checkout">
          <div>
            <div class="total-label">应收合计</div>
            <div class="total">¥{{ payTotal.toFixed(2) }}</div>
            <button class="pay-note" @click="memberOpen = true">
              优惠 -¥{{ discountOf.toFixed(2) }} · {{ cart.length }} 件
              <span v-if="member" class="member-chip">{{ member.name || member.phone }}</span>
            </button>
          </div>

          <div class="spacer"></div>

          <!-- v03 P2：收款方式 2×2 四按钮 52px -->
          <div class="method-label">收款方式</div>
          <div class="pay-methods-grid">
            <button :disabled="!cart.length || submitting" @click="openPay('cash')">现金</button>
            <button :disabled="!cart.length || submitting" @click="openPay('wechat')">扫码</button>
            <button :disabled="!cart.length || submitting" @click="openPay('alipay')">被扫</button>
            <button :disabled="!cart.length || submitting" @click="openPay('split')">组合</button>
          </div>

          <button class="pay" :disabled="!cart.length || submitting" @click="checkout">
            结算 <span class="kbd">Enter</span>
          </button>

          <button class="cart-toggle" :class="{ open: cartOpen }" @click="cartOpen = !cartOpen">
            购物车<span v-if="cart.length" class="cart-count">{{ cart.length }}</span>
          </button>
        </div>
      </aside>
    </main>

    <!-- v03 P2：独立快捷键条 38px，贴在内容区底部 -->
    <div v-if="view === 'cashier'" class="hotkeys-bar">
      <span><b>F1</b> 搜索</span>
      <span><b>F2</b> 改价</span>
      <span><b>F4</b> 挂单</span>
      <span><b>F5</b> 取单</span>
      <span><b>Enter</b> 结算</span>
      <span><b>Ctrl+W</b> 清空</span>
      <span><b>+/-</b> 数量</span>
    </div>

    <div v-else-if="view === 'orders'" class="page light"><OrdersView /></div>
    <div v-else-if="view === 'tables'" class="page light"><TablesView :role="user.role" /></div>
    <div v-else-if="view === 'products'" class="page light"><ProductsView @changed="loadProducts" @templates="tplOpen = true" /></div>
    <div v-else-if="view === 'settings'" class="page light"><SettingsView :role="user.role" /></div>
    <div v-else-if="view === 'staff'" class="page light"><StaffView /></div>
    <div v-else-if="view === 'members'" class="page light"><MembersView /></div>
    <div v-else-if="view === 'reports'" class="page light"><ReportsView /></div>
    <div v-else-if="view === 'audit'" class="page light"><AuditView /></div>
    <div v-else class="page light"><ShiftView :role="user.role" /></div>

    <MemberDialog :open="memberOpen" :current="member" @close="memberOpen = false" @picked="onMemberPicked" />

    <PaymentModal
      v-if="paying"
      :total="payTotal"
      :count="heldTarget ? 0 : cart.length"
      :order-label="heldTarget ? heldTarget.orderNo : '待生成'"
      :member="member"
      :submitting="submitting"
      :preset="payPreset"
      @confirm="pay"
      @close="paying = false; heldTarget = null"
    />

    <div v-if="choiceTarget" class="mask" @click.self="choiceTarget = null">
      <div class="modal">
        <h3>选择套餐内容 · {{ choiceTarget.name }}</h3>
        <div v-for="(opts, gid) in groupsOf(choiceTarget)" :key="gid" style="margin-bottom: 12px">
          <p class="muted" style="margin: 0 0 6px">第 {{ gid }} 组（选一个）</p>
          <div style="display:flex; gap:8px; flex-wrap: wrap">
            <button
              v-for="o in opts"
              :key="o.componentId"
              :class="{ primary: choicePick[gid] === Number(o.componentId) }"
              @click="choicePick[gid] = Number(o.componentId)"
            >{{ o.name }}</button>
          </div>
        </div>
        <div class="modal-actions">
          <button @click="choiceTarget = null">取消</button>
          <button class="primary" @click="confirmChoice">加入购物车</button>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import StatusBar from './components/StatusBar.vue'
import NavRail from './components/NavRail.vue'
import DemoBanner from './components/DemoBanner.vue'
import OnboardingWizard from './components/OnboardingWizard.vue'
import TemplatePicker from './components/TemplatePicker.vue'
import ProductGrid from './components/ProductGrid.vue'
import CartPanel from './components/CartPanel.vue'
import LoginView from './components/LoginView.vue'
import OrdersView from './components/OrdersView.vue'
import TablesView from './components/TablesView.vue'
import ProductsView from './components/ProductsView.vue'
import ShiftView from './components/ShiftView.vue'
import SettingsView from './components/SettingsView.vue'
import StaffView from './components/StaffView.vue'
import ReportsView from './components/ReportsView.vue'
import AuditView from './components/AuditView.vue'
import MembersView from './components/MembersView.vue'
import PaymentModal from './components/PaymentModal.vue'
import MemberDialog from './components/MemberDialog.vue'
import { api } from './api/index.js'
import { cartTotal, roundPayable } from './lib/money.js'
import { clearToken, getToken } from './lib/http.js'

const user = ref(null)
const view = ref('cashier')
const tplOpen = ref(false)
const products = ref([])
// 默认停在「全部」而不是「热销」：新建商品默认不热销，若默认看热销，
// 店员会以为"商品没保存"。热销只作为快捷筛选，不能当默认视图。
const categories = ref(['全部', '热销'])
const health = ref(null)
const agentError = ref('')
const keyword = ref('')
const activeCat = ref('全部')
const cart = ref([])
const paying = ref(false)
const heldOrders = ref([])
const stockAlerts = ref([])
const heldTarget = ref(null)
const roundMode = ref('none')
const memberOpen = ref(false)
const member = ref(null)
const memberQuote = ref(null)
const submitting = ref(false)
const cartOpen = ref(false)
const toastMsg = ref('')
const toastBad = ref(false)
const grid = ref(null)

let healthTimer = null
let toastTimer = null

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return products.value.filter((p) => {
    if (kw) {
      // 搜索是全局的：不受当前类目限制，否则「桂花乌龙」在奶茶类目下搜不到。
      return (
        p.name.toLowerCase().includes(kw) ||
        (p.category || '').toLowerCase().includes(kw) ||
        (p.barcode || '').includes(kw) ||
        String(p.id) === kw
      )
    }
    if (activeCat.value === '全部') return true
    return activeCat.value === '热销' ? p.hot : p.category === activeCat.value
  })
})

const total = computed(() => cartTotal(cart.value))

// 与服务端同一口径：抹零向下取整（分）。前端算出来是为了"付款金额=应收"，
// 避免服务端因为金额不一致直接拒单。
const payable = computed(() => roundPayable(total.value, roundMode.value))
// 会员折扣金额：报价后的应收比原价少多少，给结算区「优惠 -¥X」用
const discountOf = computed(() => {
  if (!memberQuote.value) return 0
  const off = payable.value - Number(memberQuote.value.payable || 0)
  return off > 0 ? off : 0
})
// 收挂单时弹窗金额取挂单应收，不是购物车合计。
const payTotal = computed(() => {
  if (heldTarget.value) return Number(heldTarget.value.payable)
  if (memberQuote.value) return Number(memberQuote.value.payable)
  return payable.value
})

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toastMsg.value = ''
  }, 2600)
}

const choiceTarget = ref(null)
const choicePick = ref({})

function groupsOf(p) {
  const groups = {}
  for (const b of p.bundle || []) {
    if (b.groupId > 0) {
      ;(groups[b.groupId] ||= []).push(b)
    }
  }
  return groups
}

function add(p) {
  const groups = groupsOf(p)
  if (Object.keys(groups).length) {
    // 套餐含可选项：先让收银员选，再入购物车
    choiceTarget.value = p
    choicePick.value = {}
    for (const g of Object.keys(groups)) choicePick.value[g] = Number(groups[g][0].componentId)
    return
  }
  pushCart(p, [])
}

function confirmChoice() {
  const p = choiceTarget.value
  if (!p) return
  const choices = Object.entries(choicePick.value).map(([groupId, componentId]) => ({
    groupId: Number(groupId),
    componentId: Number(componentId)
  }))
  choiceTarget.value = null
  pushCart(p, choices)
}

function pushCart(p, choices) {
  const suffix = choices.length
    ? '（' + choices.map((c) => {
        const b = (p.bundle || []).find((x) => Number(x.componentId) === c.componentId)
        return b ? b.name : ''
      }).filter(Boolean).join('/') + '）'
    : ''
  const key = p.id + '|' + choices.map((c) => c.groupId + ':' + c.componentId).sort().join(',')
  const hit = cart.value.find((it) => it.key === key)
  if (hit) {
    hit.qty += 1
    return
  }
  cart.value.push({
    key, id: p.id, name: p.name + suffix, price: p.price, listPrice: p.price, qty: 1, choices
  })
}

function inc(it) {
  it.qty += 1
}

function dec(it) {
  it.qty -= 1
  if (it.qty <= 0) remove(it)
}

function remove(it) {
  cart.value = cart.value.filter((x) => x.id !== it.id)
}

function clear() {
  cart.value = []
}

function reprice(it) {
  const input = window.prompt('改价（元）：' + it.name, it.price.toFixed(2))
  if (input === null) return
  const next = Number(input)
  if (!Number.isFinite(next) || next <= 0) {
    toast('价格必须大于 0', true)
    return
  }
  it.price = Math.round(next * 100) / 100
}

function checkout() {
  if (!cart.value.length) return
  paying.value = true
}

// v03 P2：结算区的收款方式按钮先记下方式，弹窗打开时直接选中对应方式
const payPreset = ref('cash')
function openPay(method) {
  if (!cart.value.length || submitting.value) return
  payPreset.value = method
  paying.value = true
}

async function loadStockAlerts() {
  try {
    stockAlerts.value = await api.stockAlerts()
  } catch (e) { /* 库存模块不可用时不打扰收银 */ }
}

async function loadHeld() {
  try {
    const list = await api.listOrders(50)
    heldOrders.value = (list || []).filter((o) => o.status === 'held')
  } catch (e) {
    // 挂单列表失败不打断收银
  }
}

async function holdOrder() {
  if (!cart.value.length || submitting.value) return
  submitting.value = true
  try {
    await api.createOrder({
      hold: true,
      items: cart.value.map((it) => ({
        productId: String(it.id),
        qty: it.qty,
        ...(it.price !== it.listPrice ? { price: it.price } : {}),
        ...(it.choices && it.choices.length ? { choices: it.choices } : {})
      }))
    })
    cart.value = []
    toast('已挂单，顾客回来点「收款」即可')
    await loadHeld()
  } catch (e) {
    toast('挂单失败：' + (e.message || '未知错误'), true)
  } finally {
    submitting.value = false
  }
}

async function refreshQuote() {
  if (!member.value) {
    memberQuote.value = null
    return
  }
  try {
    // 购物车变了要重新报价，否则会员折扣按旧的应收算
    memberQuote.value = await api.memberQuote(member.value.id, payable.value)
  } catch (e) {
    memberQuote.value = null
  }
}

async function onMemberPicked(m) {
  member.value = m
  await refreshQuote()
}

// 应收变化（加/减商品、改价）时刷新会员报价
watch(payable, () => { refreshQuote() })

function startPayHeld(order) {
  heldTarget.value = order
  paying.value = true
}

async function pay(payload) {
  if (submitting.value) return
  submitting.value = true
  const method = payload && payload.method ? payload.method : 'cash'
  const cashAmount = payload && payload.cash != null ? payload.cash : total.value
  // 组合支付：分账明细由弹窗算好并校验过合计=应收
  const payments = payload && payload.payments
    ? payload.payments
        .filter((p) => Number(p.amount) > 0)
        .map((p) => ({ method: p.method, amount: Number(p.amount), memberId: p.method === 'member' && member.value ? member.value.id : 0 }))
    : null
  // 挂单收款：只补支付记录，不新建订单
  if (heldTarget.value) {
    try {
      await api.payOrder(heldTarget.value.orderNo, {
        cash: cashAmount,
        payments: [{ method, amount: heldTarget.value.payable }]
      })
      heldTarget.value = null
      paying.value = false
      toast('已收款')
      await loadHeld()
    } catch (e) {
      toast('收款失败：' + (e.message || '未知错误'), true)
    } finally {
      submitting.value = false
    }
    return
  }
  const body = {
    idempotencyKey: crypto.randomUUID ? crypto.randomUUID() : 'k-' + Date.now(),
    cash: cashAmount,
    // 记账式支付：扫码只记录方式，金额等于应收（ADR-004）
    memberId: member.value ? member.value.id : 0,
    payments: payments || [{ method, amount: payTotal.value, memberId: payload && payload.memberId ? payload.memberId : 0 }],
    items: cart.value.map((it) => ({
      productId: String(it.id),
      qty: it.qty,
      // 只有改过价才带 price，避免把目录价当作"改价"请求
      ...(it.price !== it.listPrice ? { price: it.price } : {})
    }))
  }
  try {
    const order = await api.createOrder(body)
    paying.value = false
    cart.value = []
    toast('已提交出票，等待打印机')
    member.value = null
    memberQuote.value = null
    watchPrint(order.orderNo)
  } catch (e) {
    toast('结算失败：' + (e.message || '未知错误'), true)
  } finally {
    submitting.value = false
  }
}

async function watchPrint(orderNo) {
  for (let i = 0; i < 40; i++) {
    await sleep(400)
    try {
      const order = await api.getOrder(orderNo)
      if (order.printStatus === 2) return toast('出票完成')
      if (order.printStatus === 3) return toast('打印失败，已进入待重打', true)
    } catch (e) {
      // 网络抖动忽略，继续轮询
    }
  }
}

async function loadProducts() {
  try {
    const list = await api.listProducts(false)
    products.value = list
    const cats = Array.from(new Set(list.map((p) => p.category).filter(Boolean)))
    // 固定两个入口：「全部」（默认，保证新品立刻可见）与「热销」
    categories.value = ['全部', '热销'].concat(cats)
    if (!categories.value.includes(activeCat.value)) activeCat.value = '全部'
  } catch (e) {
    // 商品拉不到时保持空列表，收银台会提示没有商品
  }
}

async function refreshHealth() {
  try {
    health.value = await api.health()
    agentError.value = ''
  } catch (e) {
    health.value = null
    agentError.value = e.message || '连接失败'
  }
}

function onKey(e) {
  // 管理页：只保留 Esc 关弹窗（F1 等快捷键属收银台专注模式）
  if (view.value !== 'cashier') {
    if (e.key === 'Escape') paying.value = false
    return
  }
  // 输入框聚焦时（搜索框/改价等）不劫持 +/- 与 Ctrl+W，避免打字丢失
  const tag = (e.target && e.target.tagName) || ''
  const typing = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
  if (typing && e.key !== 'Escape') return

  if (e.key === 'F1') {
    e.preventDefault()
    if (grid.value) grid.value.focus()
    return
  }
  // v03 快捷键体系：F2 改价 / F4 挂单 / F5 取单 / Ctrl+W 清空 / +- 数量
  if (e.key === 'F2' && cart.value.length) {
    e.preventDefault()
    reprice(cart.value[cart.value.length - 1])
    return
  }
  if (e.key === 'F4' && cart.value.length) {
    e.preventDefault()
    holdOrder()
    return
  }
  if (e.key === 'F5' && heldOrders.value.length) {
    e.preventDefault()
    startPayHeld(heldOrders.value[0])
    return
  }
  if ((e.key === 'w' || e.key === 'W') && e.ctrlKey && cart.value.length) {
    e.preventDefault()
    clear()
    toast('已清空购物车')
    return
  }
  if ((e.key === '+' || e.key === '=') && cart.value.length) {
    e.preventDefault()
    inc(cart.value[cart.value.length - 1])
    return
  }
  if ((e.key === '-' || e.key === '_') && cart.value.length) {
    e.preventDefault()
    dec(cart.value[cart.value.length - 1])
    return
  }
  if (e.key === 'Escape') {
    paying.value = false
  }
}

async function boot() {
  if (!getToken()) return
  try {
    user.value = await api.me()
    loadProducts()
    refreshHealth()
    startTimers()
    // 刷新后若 token 仍有效，要把已有挂单重新拉出来（否则顶部挂单条是空的）
    loadHeld()
    loadStockAlerts()
  } catch (e) {
    user.value = null
  }
}

function startTimers() {
  clearInterval(healthTimer)
  healthTimer = setInterval(refreshHealth, 5000)
}

function onLogin(staff) {
  user.value = staff
  loadProducts()
  refreshHealth()
  startTimers()
  loadHeld()
  loadStockAlerts()
}

async function onLogout() {
  try {
    await api.logout()
  } catch (e) {
    // 服务端不可达也要能退出本地
  }
  clearToken()
  user.value = null
  clearInterval(healthTimer)
}

function onAuthExpired() {
  user.value = null
  clearInterval(healthTimer)
  toast('登录已过期，请重新登录', true)
}

onMounted(() => {
  loadHeld()
  api.getSettings().then((s) => { roundMode.value = (s && s.round_mode) || 'none' }).catch(() => {})
  boot()
  window.addEventListener('keydown', onKey)
  window.addEventListener('pos:auth-expired', onAuthExpired)
})

onUnmounted(() => {
  clearInterval(healthTimer)
  clearTimeout(toastTimer)
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('pos:auth-expired', onAuthExpired)
})
</script>
