<template>
  <section class="orders">
    <header class="orders-head">
      <h2>{{ tr('tbl.title') }}</h2>
      <span class="muted">{{ tr('tbl.hint') }}</span>
      <span class="grow"></span>
      <button v-if="isManager" @click="startCreate">{{ tr('tbl.new') }}</button>
      <button @click="load" :disabled="loading">{{ loading ? tr('tbl.loading') : tr('tbl.refresh') }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div v-if="tables.length" class="tbl-grid">
      <div v-for="t in tables" :key="t.id" class="tbl-card" :class="{ occ: t.status === 'occupied' }">
        <div class="tbl-top">
          <span class="tbl-name">{{ t.name }}</span>
          <span class="tbl-status" :class="t.status === 'occupied' ? 'busy' : 'free'">
            {{ t.status === 'occupied' ? tr('tbl.occupied') : tr('tbl.free') }}
          </span>
        </div>
        <div class="tbl-meta">
          <span v-if="t.status === 'occupied'">{{ tr('tbl.guests', { n: t.guests }) }}</span>
          <span v-if="t.status === 'occupied' && t.openedAt">{{ tr('tbl.openedAt', { at: t.openedAt }) }}</span>
          <span v-else>{{ tr('tbl.waiting') }}</span>
        </div>
        <div class="tbl-actions">
          <button v-if="t.status === 'free'" class="primary" @click="startOpen(t)">{{ tr('tbl.open') }}</button>
          <template v-else>
            <button @click="startAdd(t)">{{ tr('tbl.addItems') }}</button>
          <button @click="startMerge(t)">{{ tr('tbl.merge') }}</button>
          <button @click="startTransfer(t)">{{ tr('tbl.transfer') }}</button>
          <button class="primary" @click="startCheckout(t)">{{ tr('tbl.checkout') }}</button>
            <button @click="askRelease(t)">{{ tr('tbl.release') }}</button>
          </template>
        </div>
      </div>
    </div>
    <EmptyState
      v-else
      :title="tr('tbl.emptyTitle')"
      :desc="tr('tbl.emptyDesc')"
      :actions="isManager ? [{ label: tr('tbl.new'), primary: true, onClick: startCreate }] : []"
    />

    <!-- 新建桌台 -->
    <div v-if="createOpen" class="mask" @click.self="createOpen = false">
      <div class="modal">
        <h3>{{ tr('tbl.new') }}</h3>
        <input v-model="newName" maxlength="20" :placeholder="tr('tbl.namePh')" @keyup.enter="doCreate" />
        <div class="modal-actions">
          <button @click="createOpen = false">{{ tr('tbl.cancel') }}</button>
          <button class="primary" :disabled="creating || !newName.trim()" @click="doCreate">
            {{ creating ? tr('tbl.creating') : tr('tbl.create') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 开台：选商品 + 人数 -->
    <div v-if="openTarget" class="mask" @click.self="openTarget = null">
      <div class="modal product-modal">
        <h3>{{ tr('tbl.openTitle', { name: openTarget.name }) }}</h3>
        <div class="product-form">
          <label>{{ tr('tbl.guestsLabel') }}<input type="number" min="0" v-model.number="openGuests" /></label>
        </div>
        <p class="muted" style="margin: 4px 0 8px">{{ tr('tbl.pickItems') }}</p>
        <div class="tbl-pick">
          <div v-for="p in products" :key="p.id" class="tbl-pick-item">
            <span>{{ p.name }} <b>¥{{ Number(p.price).toFixed(2) }}</b></span>
            <input type="number" min="0" v-model.number="openQty[p.id]" :placeholder="tr('tbl.qtyPh')" />
          </div>
        </div>
        <div class="modal-actions">
          <button @click="openTarget = null">{{ tr('tbl.cancel') }}</button>
          <button class="primary" :disabled="opening || !hasOpenItems" @click="doOpen">
            {{ opening ? tr('tbl.opening') : tr('tbl.confirmOpen') }}
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
        <h3>{{ moveMode === 'merge' ? tr('tbl.merge') : tr('tbl.transfer') }} · {{ moveTarget.name }}</h3>
        <p class="muted" v-if="moveMode === 'merge'">{{ tr('tbl.mergeHint') }}</p>
        <p class="muted" v-else>{{ tr('tbl.transferHint') }}</p>
        <div class="settings-form" style="max-width: 320px">
          <label>{{ tr('tbl.target') }}
            <select v-model.number="moveTo">
              <option :value="0" disabled>{{ tr('tbl.pickTarget') }}</option>
              <option v-for="c in moveCandidates" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
          <p v-if="!moveCandidates.length" class="muted" style="margin: 6px 0 0; font-size: 12px">
            {{ moveMode === 'merge' ? tr('tbl.noMerge') : tr('tbl.noTransfer') }}
          </p>
        </div>
        <div class="modal-actions">
          <button @click="moveTarget = null">{{ tr('tbl.cancel') }}</button>
          <button class="primary" :disabled="!moveTo || moving" @click="doMove">{{ moving ? tr('tbl.working') : tr('tbl.confirm') }}</button>
        </div>
        <p v-if="moveError" class="orders-error">{{ moveError }}</p>
      </div>
    </div>

    <div v-if="addOpen" class="mask" @click.self="addOpen = false">
      <div class="modal">
        <h3>{{ tr('tbl.addTitle', { name: addTarget && addTarget.name }) }}</h3>
        <div class="tbl-pick">
          <div v-for="p in products" :key="p.id" class="tbl-pick-item">
            <span>{{ p.name }} <b>¥{{ Number(p.price).toFixed(2) }}</b></span>
            <input type="number" min="0" v-model.number="addQty[p.id]" :placeholder="tr('tbl.qtyPh')" />
          </div>
        </div>
        <div class="modal-actions">
          <button @click="addOpen = false">{{ tr('tbl.cancel') }}</button>
          <button class="primary" @click="doAdd">{{ tr('tbl.confirmAdd') }}</button>
        </div>
      </div>
    </div>

    <div v-if="releaseTarget" class="mask" @click.self="releaseTarget = null">
      <div class="modal">
        <h3>{{ tr('tbl.releaseTitle', { name: releaseTarget.name }) }}</h3>
        <p class="muted" style="margin: 0">
          {{ tr('tbl.releaseHint') }}
        </p>
        <div class="modal-actions">
          <button @click="releaseTarget = null">{{ tr('tbl.cancel') }}</button>
          <button class="primary" :disabled="releasing" @click="doRelease">
            {{ releasing ? tr('tbl.releasing') : tr('tbl.confirmRelease') }}
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
import { t as tr } from '../i18n.js'
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
    error.value = tr('tbl.loadFailed', { msg: e.message || '' })
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
    toast(tr('tbl.created', { name }))
    await load()
  } catch (e) {
    toast(tr('tbl.createFailed', { msg: e.message || '' }), true)
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
    toast(tr('tbl.opened'))
    await load()
  } catch (e) {
    openError.value = tr('tbl.openFailed', { msg: e.message || '' })
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
        toast(tr('tbl.merged', { name: res.into ? res.into.name : '', n: cnt, total: payable.toFixed(2) }))
      } else {
        toast(tr('tbl.mergeDone'))
      }
    } else {
      // 返回 {from, to}：转台后 load() 刷新卡片（原桌空闲、目标桌更新桌号与开台时间）
      const res = await api.transferTable(moveTarget.value.id, moveTo.value)
      moveTarget.value = null
      await load()
      toast(res && res.to ? tr('tbl.transferred', { name: res.to.name }) : tr('tbl.transferDone'))
    }
  } catch (e) {
    moveError.value = tr('tbl.opFailed', { msg: e.message || '' })
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
    toast(tr('tbl.pickAdd'), true)
    return
  }
  try {
    const res = await api.addTableItems(addTarget.value.id, { items })
    addOpen.value = false
    addTarget.value = null
    await load()
    // 加菜接口返回最新挂单（Payable 为分），提示新的应收金额
    const payableYuan = res && res.order ? Number(res.order.payable || 0) : null
    toast(payableYuan != null ? tr('tbl.added', { total: payableYuan.toFixed(2) }) : tr('tbl.addDone'))
  } catch (e) {
    toast(tr('tbl.addFailed', { msg: e.message || '' }), true)
  }
}

async function startCheckout(t) {
  try {
    const orders = await api.listOrders(100)
    const o = (orders || []).find((x) => x.orderNo === t.orderNo)
    if (!o) {
      toast(tr('tbl.noHeld', { no: t.orderNo }), true)
      return
    }
    checkoutTarget.value = t
    payTotal.value = Number(o.payable)
    paying.value = true
  } catch (e) {
    toast(tr('tbl.heldFailed', { msg: e.message || '' }), true)
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
    toast(tr('tbl.paid'))
    await load()
  } catch (e) {
    toast(tr('tbl.payFailed', { msg: e.message || '' }), true)
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
    toast(tr('tbl.released'))
    await load()
  } catch (e) {
    toast(tr('tbl.releaseFailed', { msg: e.message || '' }), true)
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
