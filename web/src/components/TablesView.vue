<template>
  <section class="orders">
    <header class="orders-head">
      <h2>桌台</h2>
      <span class="muted">开台 = 建挂单，结账 = 收款并清台；挂单仍可在订单页查看</span>
      <span class="grow"></span>
      <button v-if="isManager" @click="startCreate">新建桌台</button>
      <button @click="load" :disabled="loading">{{ loading ? '读取中…' : '刷新' }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="tables.length" class="tbl-grid">
      <div v-for="t in tables" :key="t.id" class="tbl-card" :class="{ occ: t.status === 'occupied' }">
        <div class="tbl-top">
          <span class="tbl-name">{{ t.name }}</span>
          <span class="tbl-status" :class="t.status === 'occupied' ? 'busy' : 'free'">
            {{ t.status === 'occupied' ? '占用' : '空闲' }}
          </span>
        </div>
        <div class="tbl-meta">
          <span v-if="t.status === 'occupied'">人数 {{ t.guests }}</span>
          <span v-if="t.status === 'occupied' && t.openedAt">开台 {{ t.openedAt }}</span>
          <span v-else>等待开台</span>
        </div>
        <div class="tbl-actions">
          <button v-if="t.status === 'free'" class="primary" @click="startOpen(t)">开台</button>
          <template v-else>
            <button @click="startAdd(t)">加菜</button>
          <button @click="startMerge(t)">并台</button>
          <button @click="startTransfer(t)">转台</button>
          <button class="primary" @click="startCheckout(t)">结账</button>
            <button @click="askRelease(t)">清台</button>
          </template>
        </div>
      </div>
    </div>
    <EmptyState
      v-else
      title="还没有桌台"
      desc="点「新建桌台」添加桌位；空闲桌可开台点单，结账后自动回到空闲"
      :actions="isManager ? [{ label: '新建桌台', primary: true, onClick: startCreate }] : []"
    />

    <!-- 新建桌台 -->
    <div v-if="createOpen" class="mask" @click.self="createOpen = false">
      <div class="modal">
        <h3>新建桌台</h3>
        <input v-model="newName" maxlength="20" placeholder="桌名，例如：A1" @keyup.enter="doCreate" />
        <div class="modal-actions">
          <button @click="createOpen = false">取消</button>
          <button class="primary" :disabled="creating || !newName.trim()" @click="doCreate">
            {{ creating ? '创建中…' : '创建' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 开台：选商品 + 人数 -->
    <div v-if="openTarget" class="mask" @click.self="openTarget = null">
      <div class="modal product-modal">
        <h3>开台 · {{ openTarget.name }}</h3>
        <div class="product-form">
          <label>人数<input type="number" min="0" v-model.number="openGuests" /></label>
        </div>
        <p class="muted" style="margin: 4px 0 8px">选商品（数量填 0 表示不要）</p>
        <div class="tbl-pick">
          <div v-for="p in products" :key="p.id" class="tbl-pick-item">
            <span>{{ p.name }} <b>¥{{ Number(p.price).toFixed(2) }}</b></span>
            <input type="number" min="0" v-model.number="openQty[p.id]" placeholder="数量" />
          </div>
        </div>
        <div class="modal-actions">
          <button @click="openTarget = null">取消</button>
          <button class="primary" :disabled="opening || !hasOpenItems" @click="doOpen">
            {{ opening ? '开台中…' : '确认开台' }}
          </button>
        </div>
        <p v-if="openError" class="orders-error">{{ openError }}</p>
      </div>
    </div>

    <!-- 结账：复用 PaymentModal，金额取挂单应收 -->
    <PaymentModal
      v-if="paying"
      :total="payTotal"
      :submitting="submitting"
      @confirm="doCheckout"
      @close="paying = false"
    />

    <!-- 清台二次确认 -->
    <div v-if="moveTarget" class="mask" @click.self="moveTarget = null">
      <div class="modal">
        <h3>{{ moveMode === 'merge' ? '并台' : '转台' }} · {{ moveTarget.name }}</h3>
        <p class="muted" v-if="moveMode === 'merge'">把本桌菜品并入另一张桌，本桌随后释放。</p>
        <p class="muted" v-else>把本桌订单挪到一张空桌，菜品不变。</p>
        <div class="settings-form" style="max-width: 320px">
          <label>目标桌台
            <select v-model.number="moveTo">
              <option :value="0" disabled>请选择</option>
              <option v-for="c in moveCandidates" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
          <p v-if="!moveCandidates.length" class="muted" style="margin: 6px 0 0; font-size: 12px">
            {{ moveMode === 'merge' ? '没有其他占用中的桌台可并入（先开台再并台）' : '没有空闲桌台可转入（先清台或结账腾出空桌）' }}
          </p>
        </div>
        <div class="modal-actions">
          <button @click="moveTarget = null">取消</button>
          <button class="primary" :disabled="!moveTo || moving" @click="doMove">{{ moving ? '处理中…' : '确认' }}</button>
        </div>
        <p v-if="moveError" class="orders-error">{{ moveError }}</p>
      </div>
    </div>

    <div v-if="addOpen" class="mask" @click.self="addOpen = false">
      <div class="modal">
        <h3>加菜 · {{ addTarget && addTarget.name }}</h3>
        <div class="tbl-pick">
          <div v-for="p in products" :key="p.id" class="tbl-pick-item">
            <span>{{ p.name }} <b>¥{{ Number(p.price).toFixed(2) }}</b></span>
            <input type="number" min="0" v-model.number="addQty[p.id]" placeholder="数量" />
          </div>
        </div>
        <div class="modal-actions">
          <button @click="addOpen = false">取消</button>
          <button class="primary" @click="doAdd">确认加菜</button>
        </div>
      </div>
    </div>

    <div v-if="releaseTarget" class="mask" @click.self="releaseTarget = null">
      <div class="modal">
        <h3>清台 · {{ releaseTarget.name }}</h3>
        <p class="muted" style="margin: 0">
          确定清台？挂单不会删除，仍可在订单页查看并收款。
        </p>
        <div class="modal-actions">
          <button @click="releaseTarget = null">取消</button>
          <button class="primary" :disabled="releasing" @click="doRelease">
            {{ releasing ? '清台中…' : '确认清台' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'
import EmptyState from './EmptyState.vue'
import PaymentModal from './PaymentModal.vue'

const props = defineProps({ role: { type: String, default: 'cashier' } })

const tables = ref([])
const products = ref([])
const loading = ref(false)
const error = ref('')

const isManager = computed(() => props.role === 'manager')

// 新建
const createOpen = ref(false)
const newName = ref('')
const creating = ref(false)

// 开台
const openTarget = ref(null)
const openGuests = ref(1)
const openQty = reactive({})
const opening = ref(false)
const openError = ref('')

// 结账
const paying = ref(false)
const checkoutTarget = ref(null)
const addTarget = ref(null)
const addQty = ref({})
const addOpen = ref(false)
const payTotal = ref(0)
const submitting = ref(false)

// 清台
const releaseTarget = ref(null)
const releasing = ref(false)

// toast
const toastMsg = ref('')
const toastBad = ref(false)
let timer = null

const hasOpenItems = computed(() => Object.values(openQty).some((q) => Number(q) > 0))

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(timer)
  timer = setTimeout(() => (toastMsg.value = ''), 2600)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    tables.value = await api.listTables()
  } catch (e) {
    error.value = '读取桌台失败：' + (e.message || '未知错误')
  } finally {
    loading.value = false
  }
}

async function loadProducts() {
  try {
    products.value = await api.listProducts(false)
  } catch (e) {
    // 商品拉不到时开台弹窗为空，不打断桌台页
  }
}

// 新建桌台
function startCreate() {
  newName.value = ''
  createOpen.value = true
}

async function doCreate() {
  const name = newName.value.trim()
  if (!name || creating.value) return
  creating.value = true
  try {
    await api.createTable(name)
    newName.value = ''
    createOpen.value = false
    toast('已新建桌台「' + name + '」')
    await load()
  } catch (e) {
    toast('新建失败：' + (e.message || '未知错误'), true)
  } finally {
    creating.value = false
  }
}

// 开台
function startOpen(t) {
  openTarget.value = t
  openGuests.value = 1
  Object.keys(openQty).forEach((k) => delete openQty[k])
  openError.value = ''
}

async function doOpen() {
  if (!openTarget.value || opening.value) return
  opening.value = true
  openError.value = ''
  const items = Object.entries(openQty)
    .filter(([, q]) => Number(q) > 0)
    .map(([id, q]) => ({ productId: String(id), qty: Number(q) }))
  try {
    await api.openTable(openTarget.value.id, { guests: Number(openGuests.value) || 0, items })
    openTarget.value = null
    toast('已开台，点单已挂起，可随时结账')
    await load()
  } catch (e) {
    openError.value = '开台失败：' + (e.message || '未知错误')
  } finally {
    opening.value = false
  }
}

// 结账：金额取挂单应收
const moveTarget = ref(null)
const moveMode = ref('merge')
const moveTo = ref(0)
const moving = ref(false)
const moveError = ref('')
const moveCandidates = ref([])

function startMerge(t) {
  moveTarget.value = t
  moveMode.value = 'merge'
  moveTo.value = 0
  moveError.value = ''
  moveCandidates.value = tables.value.filter((x) => x.id !== t.id && x.status === 'occupied')
}

function startTransfer(t) {
  moveTarget.value = t
  moveMode.value = 'transfer'
  moveTo.value = 0
  moveError.value = ''
  moveCandidates.value = tables.value.filter((x) => x.id !== t.id && x.status === 'free')
}

async function doMove() {
  moving.value = true
  moveError.value = ''
  try {
    if (moveMode.value === 'merge') {
      // 返回 {from, into, order}：order 为并台后目标桌挂单（Go 结构，PascalCase，Payable 为分）
      const res = await api.mergeTables(moveTarget.value.id, moveTo.value)
      moveTarget.value = null
      await load()
      const order = res && res.order
      if (order && Array.isArray(order.items)) {
        const cnt = order.items.reduce((s, it) => s + Number(it.qty || 0), 0)
        const payable = Number(order.payable || 0)
        toast('已并入 ' + (res.into ? res.into.name : '') + '，共 ' + cnt + ' 件商品，应收 ¥' + payable.toFixed(2))
      } else {
        toast('已并台')
      }
    } else {
      // 返回 {from, to}：转台后 load() 刷新卡片（原桌空闲、目标桌更新桌号与开台时间）
      const res = await api.transferTable(moveTarget.value.id, moveTo.value)
      moveTarget.value = null
      await load()
      toast(res && res.to ? '已转到 ' + res.to.name : '已转台')
    }
  } catch (e) {
    moveError.value = '操作失败：' + (e.message || '未知错误')
  } finally {
    moving.value = false
  }
}

function startAdd(t) {
  addTarget.value = t
  addQty.value = {}
  addOpen.value = true
}

async function doAdd() {
  const items = Object.entries(addQty.value)
    .map(([productId, qty]) => ({ productId: String(productId), qty: Number(qty) || 0 }))
    .filter((it) => it.qty > 0)
  if (!items.length) {
    toast('请选择要加的菜', true)
    return
  }
  try {
    const res = await api.addTableItems(addTarget.value.id, { items })
    addOpen.value = false
    addTarget.value = null
    await load()
    // 加菜接口返回最新挂单（Payable 为分），提示新的应收金额
    const payableYuan = res && res.order ? Number(res.order.payable || 0) : null
    toast(payableYuan != null ? '已加菜，本桌应收 ¥' + payableYuan.toFixed(2) : '已加菜')
  } catch (e) {
    toast('加菜失败：' + (e.message || '未知错误'), true)
  }
}

async function startCheckout(t) {
  try {
    const orders = await api.listOrders(100)
    const o = (orders || []).find((x) => x.orderNo === t.orderNo)
    if (!o) {
      toast('找不到挂单 ' + t.orderNo + '，请先在订单页收款', true)
      return
    }
    checkoutTarget.value = t
    payTotal.value = Number(o.payable)
    paying.value = true
  } catch (e) {
    toast('读取挂单失败：' + (e.message || '未知错误'), true)
  }
}

async function doCheckout(payload) {
  if (!checkoutTarget.value || submitting.value) return
  submitting.value = true
  const method = payload && payload.method ? payload.method : 'cash'
  const cashAmount = payload && payload.cash != null ? payload.cash : payTotal.value
  try {
    await api.checkoutTable(checkoutTarget.value.id, {
      cash: cashAmount,
      payments: [{ method, amount: payTotal.value, memberId: 0 }]
    })
    paying.value = false
    checkoutTarget.value = null
    toast('已结账并清台')
    await load()
  } catch (e) {
    toast('结账失败：' + (e.message || '未知错误'), true)
  } finally {
    submitting.value = false
  }
}

// 清台
function askRelease(t) {
  releaseTarget.value = t
}

async function doRelease() {
  if (!releaseTarget.value || releasing.value) return
  releasing.value = true
  try {
    await api.releaseTable(releaseTarget.value.id)
    releaseTarget.value = null
    toast('已清台，挂单仍在订单页')
    await load()
  } catch (e) {
    toast('清台失败：' + (e.message || '未知错误'), true)
  } finally {
    releasing.value = false
  }
}

onMounted(() => {
  load()
  loadProducts()
})
</script>

<style scoped>
.tbl-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 12px;
}

.tbl-card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 12px 14px;
}

.tbl-card.occ {
  border-color: var(--brand);
}

.tbl-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tbl-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--text);
}

.tbl-status {
  font-size: 12px;
  padding: 2px 9px;
  border-radius: 999px;
}

.tbl-status.free {
  color: var(--ok);
  border: 1px solid var(--ok);
}

.tbl-status.busy {
  color: var(--brand);
  border: 1px solid var(--brand);
}

.tbl-meta {
  margin: 8px 0 10px;
  font-size: 12px;
  color: var(--muted);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tbl-actions {
  display: flex;
  gap: 8px;
}

/* 窄屏（<900px）：桌台两列网格，按钮紧凑不换行 */
@media (max-width: 899px) {
  .tbl-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .tbl-actions {
    flex-wrap: nowrap;
    gap: 6px;
  }

  .tbl-actions button {
    padding: 5px 8px;
    font-size: 12px;
    white-space: nowrap;
  }
}

.tbl-pick {
  max-height: 220px;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 4px 10px;
  margin: 0 0 12px;
}

.tbl-pick-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 0;
  font-size: 13px;
}

.tbl-pick-item input {
  width: 62px;
  text-align: right;
}
</style>
