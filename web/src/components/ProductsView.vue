<template>
  <section class="orders products">
    <header class="orders-head">
      <h2>{{ tr('prod.title') }}</h2>
      <span class="grow"></span>
      <button class="btn-line" @click="$emit('templates')">{{ tr('prod.templates') }}</button>
      <button class="btn-line" @click="openImport">{{ tr('prod.importExcel') }}</button>
      <button class="btn-line" @click="openPurchase">{{ tr('prod.stockIn') }}</button>
      <div class="stock-entry">
        <button class="btn-line" @click="stockMenuOpen = !stockMenuOpen">{{ tr('prod.stocktake') }} ▾</button>
        <div v-if="stockMenuOpen" class="stock-menu-mask" @click="stockMenuOpen = false"></div>
        <div v-if="stockMenuOpen" class="stock-menu">
          <button @click="startStocktake">{{ tr('prod.startStocktake') }}</button>
          <button @click="openStockHistory">{{ tr('prod.history') }}</button>
        </div>
      </div>
      <button class="primary" @click="openCreate">{{ tr('prod.new') }}</button>
    </header>

    <div class="filters-row">
      <input class="f-search wide" v-model="keyword" :placeholder="tr('prod.searchPh')" @input="load" />
      <label class="f-pill">
        <select v-model="catFilter">
          <option value="">{{ tr('prod.catAll') }}</option>
          <option v-for="c in catList" :key="c" :value="c">{{ tr('prod.catPrefix') }}{{ c }}</option>
        </select>
      </label>
      <button class="link-btn" @click="catOpen = true">{{ tr('prod.catManage') }}</button>
      <span class="bar-hint">{{ tr('prod.importHint') }}</span>
      <span class="grow"></span>
    </div>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <p v-if="alerts.length" class="orders-warn">
      {{ tr('prod.lowStock', { n: alerts.length, list: alerts.map((a) => a.name + ' ' + a.qty).join(', ') }) }}
    </p>

    <table class="orders-table products-table tbl">
      <colgroup>
        <col style="width: 23%" />
        <col style="width: 15%" />
        <col style="width: 10%" />
        <col style="width: 10%" />
        <col style="width: 9%" />
        <col style="width: 9%" />
        <col style="width: 24%" />
      </colgroup>
      <thead>
        <tr>
          <th>{{ tr('prod.name') }}</th>
          <th>{{ tr('prod.barcode') }}</th>
          <th>{{ tr('prod.category') }}</th>
          <th class="num">{{ tr('prod.price') }}</th>
          <th class="num">{{ tr('prod.stock') }}</th>
          <th>{{ tr('prod.status') }}</th>
          <th>{{ tr('prod.ops') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id" :class="{ dim: !p.active }">
          <td>
            <img v-if="p.image" :src="'/pos/v1/' + p.image" class="product-thumb" alt="" />
            <span v-else class="product-thumb product-thumb-empty">{{ tr('prod.noImage') }}</span>
            {{ p.name }} <span v-if="p.hot" class="chip chip-busy">{{ tr('prod.hot') }}</span>
          </td>
          <td class="mono" :class="{ 'barcode-empty': !p.barcode }">{{ p.barcode || '—' }}</td>
          <td>{{ p.category || '—' }}</td>
          <td class="num">¥{{ p.price.toFixed(2) }}</td>
          <td class="num">
            <template v-if="stockOf(p.id)">
              <span :class="{ 'stock-low': isLow(p.id) }">{{ stockOf(p.id).qty }}</span>
            </template>
            <span v-else class="muted">{{ tr('prod.untracked') }}</span>
          </td>
          <td>
            <span class="p-status" :class="p.active ? 'on' : 'off'">
              <i></i>{{ p.active ? tr('prod.onSale') : tr('prod.offSale') }}
            </span>
          </td>
          <td class="ops ops-links">
            <button class="link-brand" @click="openStock(p)">{{ tr('prod.stock') }}</button>
            <button class="link-brand" @click="openEdit(p)">{{ tr('prod.edit') }}</button>
            <button class="link-muted" @click="toggle(p)">{{ p.active ? tr('prod.offSale') : tr('prod.putOn') }}</button>
            <button class="link-muted" @click="remove(p)">{{ tr('prod.delete') }}</button>
          </td>
        </tr>
        <tr v-if="!list.length" class="empty-row">
          <td colspan="7" class="empty-cell">
            <EmptyState
              :title="tr('p2.emptyTitle')"
              :desc="tr('p2.emptyDesc')"
              :actions="[
                { label: tr('p2.importSamples'), primary: true, onClick: importTpl },
                { label: tr('p2.newProduct'), onClick: openCreate },
                { label: tr('p2.reload'), onClick: () => load(2) }
              ]"
            />
          </td>
        </tr>
      </tbody>
    </table>
    <div class="orders-foot">
      <span>{{ tr('p2.deleteHint') }}</span>
      <span>{{ tr('p2.total', { n: shown.length }) }}</span>
    </div>

    <div v-if="labelOpen" class="mask" @click.self="labelOpen = false">
      <div class="modal" style="max-width: 560px; width: 92vw">
        <h3>{{ tr('p2.labelTitle') }}</h3>
        <p class="muted" style="margin: 0 0 8px">{{ tr('p2.labelHint') }}</p>
        <div style="max-height: 44vh; overflow: auto">
          <table class="orders-table">
            <thead><tr><th style="width:44px"></th><th>{{ tr('p2.product') }}</th><th class="num" style="width:90px">{{ tr('p2.price') }}</th><th class="num" style="width:90px">{{ tr('p2.copies') }}</th></tr></thead>
            <tbody>
              <tr v-for="p in labelCandidates" :key="p.id">
                <td><input type="checkbox" v-model="labelPick[p.id]" /></td>
                <td>{{ p.name }}</td>
                <td class="num">¥{{ Number(p.price).toFixed(2) }}</td>
                <td class="num"><input type="number" min="1" max="20" v-model.number="labelQty" style="width:70px" /></td>
              </tr>
              <tr v-if="!labelCandidates.length"><td colspan="4" class="empty-cell">{{ tr('p2.noOnSale') }}</td></tr>
            </tbody>
          </table>
        </div>
        <p v-if="labelError" class="orders-error">{{ labelError }}</p>
        <div v-if="editing && editing.id && form.kind === 'single'" style="grid-column: 1 / -1; border-top: 1px dashed var(--line); margin-top: 10px; padding-top: 10px">
          <p class="muted" style="margin: 4px 0">{{ tr('p2.bomHint') }}</p>
          <div v-for="(b, i) in bomItems" :key="i" style="display:flex; gap:8px; margin-bottom:6px">
            <select v-model.number="b.componentId" style="flex:1">
              <option :value="0" disabled>{{ tr('p2.pickMaterial') }}</option>
              <option v-for="p in singleProducts.filter((x) => Number(x.id) !== editing.id)" :key="p.id" :value="Number(p.id)">{{ p.name }}</option>
            </select>
            <input type="number" step="0.001" min="0.001" v-model.number="b.qty" style="width:100px" :title="tr('p2.qtyTitle')" />
            <input type="number" step="0.1" min="0" max="50" v-model.number="b.lossPct" style="width:90px" :title="tr('p2.lossTitle')" />
            <button type="button" @click="bomItems.splice(i, 1)">{{ tr('prod.delete') }}</button>
          </div>
          <button type="button" @click="bomItems.push({ componentId: 0, qty: 1, lossPct: 0 })">{{ tr('p2.addMaterial') }}</button>
          <button type="button" class="btn-line sm" :disabled="bomSaving" @click="saveBOM">{{ bomSaving ? tr('p2.saving') : tr('p2.saveBom') }}</button>
          <label class="btn-line sm" style="display:inline-block; cursor:pointer">
            {{ tr('p2.importBom') }}
            <input type="file" accept=".csv,text/csv" style="display:none" @change="importBOM" />
          </label>
          <span class="muted mini">{{ tr('p2.bomCsv') }}</span>
          <span v-if="bomCost > 0" class="muted" style="margin-left: 8px">{{ tr('p2.rollupCost', { v: (bomCost / 100).toFixed(2) }) }}</span>
          <p v-if="bomError" class="orders-error">{{ bomError }}</p>
        </div>
        <div class="modal-actions">
          <button @click="labelOpen = false">{{ tr('p2.cancel') }}</button>
          <button class="primary" :disabled="!labelCount || labelPrinting" @click="printLabels">
            {{ labelPrinting ? tr('p2.printing') : tr('p2.printN', { n: labelCount }) }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="purchase" class="mask" @click.self="closePurchase">
      <div class="modal" style="max-width: 760px; width: 94vw">
        <h3>{{ tr('p2.purchaseTitle') }}</h3>
        <p class="muted" style="margin: 0 0 10px">
          {{ tr('p2.purchaseHint') }}
        </p>
        <div class="purchase-head">
          <label>{{ tr('p2.supplier') }}
            <input v-model="purchase.supplier" :placeholder="tr('p2.supplierPh')" />
          </label>
          <label>{{ tr('p2.note') }}
            <input v-model="purchase.note" :placeholder="tr('p2.notePh')" />
          </label>
        </div>
        <div style="max-height: 44vh; overflow: auto">
          <table class="orders-table">
            <thead>
              <tr>
                <th>{{ tr('p2.product') }}</th>
                <th style="width: 110px">{{ tr('p2.qty') }}</th>
                <th style="width: 130px">{{ tr('p2.unitCost') }}</th>
                <th style="width: 110px" class="num">{{ tr('p2.subtotal') }}</th>
                <th style="width: 64px"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(it, i) in purchase.items" :key="i">
                <td>
                  <select v-model.number="it.productId">
                    <option :value="0">{{ tr('p2.pickProduct') }}</option>
                    <option v-for="p in purchaseProducts" :key="p.id" :value="Number(p.id)">{{ p.name }}</option>
                  </select>
                </td>
                <td><input type="number" min="1" step="1" v-model.number="it.qty" style="width: 92px" /></td>
                <td><input type="number" min="0" step="0.01" v-model.number="it.costYuan" style="width: 112px" /></td>
                <td class="num">{{ lineAmount(it) }}</td>
                <td>
                  <button class="purchase-del" :disabled="purchase.items.length === 1" @click="purchase.items.splice(i, 1)">{{ tr('prod.delete') }}</button>
                </td>
              </tr>
              <tr v-if="!purchase.items.length">
                <td colspan="5" class="empty-cell">{{ tr('p2.atLeastOne') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <button class="purchase-add" @click="addPurchaseLine">{{ tr('p2.addLine') }}</button>
        <div class="purchase-foot">
          <span class="muted">{{ tr('p2.totalAmount') }}</span>
          <b class="purchase-total">¥{{ purchaseTotal }}</b>
        </div>
        <p v-if="purchaseHint" class="muted" style="margin: 0 0 6px">{{ purchaseHint }}</p>
        <p v-if="purchaseError" class="orders-error">{{ purchaseError }}</p>
        
        <div class="modal-actions">
          <button @click="closePurchase">{{ tr('p2.cancel') }}</button>
          <button class="primary" :disabled="!purchaseValid || purchaseSaving" @click="savePurchase">
            {{ purchaseSaving ? tr('p2.saving') : tr('p2.savePurchase') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="historyOpen" class="mask" @click.self="historyOpen = false">
      <div class="modal" style="max-width: 760px; width: 92vw">
        <h3>{{ tr('p2.historyTitle') }} <span class="muted">{{ tr('p2.historyRecent', { n: historyList.length }) }}</span></h3>
        <p class="muted" style="margin: 0 0 8px">
          {{ tr('p2.historyHint') }}
        </p>
        <div style="max-height: 52vh; overflow: auto">
          <table class="orders-table">
            <thead>
              <tr>
                <th style="width: 78px">{{ tr('p2.sheetNo') }}</th>
                <th style="width: 92px">{{ tr('p2.status') }}</th>
                <th>{{ tr('p2.time') }}</th>
                <th style="width: 110px">{{ tr('p2.operator') }}</th>
                <th style="width: 96px" class="num">{{ tr('p2.diffCount') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in historyList" :key="s.id" class="history-row" @click="openHistoryDetail(s)">
                <td class="mono">#{{ s.id }}</td>
                <td>
                  <span class="chip" :class="s.status === 'done' ? 'chip-ok' : 'chip-busy'">
                    {{ s.status === 'done' ? tr('p2.done') : tr('p2.inProgress') }}
                  </span>
                </td>
                <td class="mono">{{ s.createdAt }}</td>
                <td>{{ s.operator || '—' }}</td>
                <td class="num">{{ s.diffCount }}</td>
              </tr>
              <tr v-if="historyLoading">
                <td colspan="5" class="empty-cell">{{ tr('p2.loading') }}</td>
              </tr>
              <tr v-else-if="!historyList.length">
                <td colspan="5" class="empty-cell">{{ tr('p2.noStocktakes') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="historyError" class="orders-error">{{ historyError }}</p>
        
        <div class="modal-actions">
          <button @click="historyOpen = false">{{ tr('p2.close') }}</button>
          <button class="primary" :disabled="historyLoading" @click="startStocktake">{{ tr('prod.startStocktake') }}</button>
        </div>
      </div>
    </div>

    <div v-if="sheet" class="mask" @click.self="sheet = null">
      <div class="modal" style="max-width: 640px; width: 92vw">
        <h3>{{ tr('p2.sheetTitle', { id: sheet.id }) }} <span class="muted">{{ sheet.status === 'done' ? tr('p2.done') : tr('p2.inProgress') }}</span></h3>
        <p class="muted" style="margin: 0 0 8px">
          {{ tr('p2.sheetHint') }}
        </p>
        <div style="max-height: 46vh; overflow: auto">
          <table class="orders-table">
            <thead><tr><th>{{ tr('p2.product') }}</th><th class="num">{{ tr('p2.book') }}</th><th class="num">{{ tr('p2.actual') }}</th><th class="num">{{ tr('p2.diff') }}</th></tr></thead>
            <tbody>
              <tr v-for="it in sheet.items" :key="it.productId">
                <td>{{ it.name || ('#' + it.productId) }}</td>
                <td class="num">{{ it.systemQty }}</td>
                <td class="num">
                  <input type="number" min="0" style="width: 90px"
                    :value="it.actualQty" :disabled="sheet.status === 'done'"
                    @input="setActual(it, $event.target.value)" />
                </td>
                <td class="num" :class="{ 'stock-low': it.actualQty !== null && it.diff !== 0 }">
                  {{ it.actualQty === null ? '—' : (it.diff > 0 ? '+' + it.diff : it.diff) }}
                </td>
              </tr>
              <tr v-if="!sheet.items.length"><td colspan="4" class="empty-cell">{{ tr('p2.noStockRows') }}</td></tr>
            </tbody>
          </table>
        </div>
        
        <div class="modal-actions">
          <button @click="sheet = null">{{ tr('p2.close') }}</button>
          <button :disabled="sheet.status === 'done'" @click="saveActual">{{ tr('p2.saveActual') }}</button>
          <button class="primary" :disabled="sheet.status === 'done'" @click="closeSheet">{{ tr('p2.closeSheet') }}</button>
        </div>
        <p v-if="sheetError" class="orders-error">{{ sheetError }}</p>
      </div>
    </div>

    <div v-if="stockTarget" class="mask" @click.self="stockTarget = null">
      <div class="modal">
        <h3>{{ tr('p2.stockTitle', { name: stockTarget.name }) }}</h3>
        <div class="settings-form" style="max-width: 320px">
          <label>{{ tr('p2.currentQty') }}
            <input type="number" v-model.number="stockForm.qty" />
          </label>
          <label>{{ tr('p2.warnQty') }}
            <input type="number" min="0" v-model.number="stockForm.warnQty" />
          </label>
        </div>
        <div style="display:flex; gap:8px; margin: 8px 0; flex-wrap: wrap">
          <input type="number" v-model.number="stockForm.delta" :placeholder="tr('p2.deltaPh')" style="width: 110px" />
          <input v-model="stockForm.reason" :placeholder="tr('p2.reasonPh')" style="flex:1" />
          <button :disabled="!stockForm.delta" @click="applyAdjust">{{ tr('p2.adjust') }}</button>
        </div>
        
        <div class="modal-actions">
          <button @click="stockTarget = null">{{ tr('p2.cancel') }}</button>
          <button class="primary" @click="saveStock">{{ tr('p2.save') }}</button>
        </div>
        <p v-if="stockError" class="orders-error">{{ stockError }}</p>
      </div>
    </div>

    <div v-if="editing" class="mask light-mask" @click.self="editing = null">
      <div class="modal product-modal edit-modal">
        <div class="pay-head">
          <h3>{{ editing.id ? tr('p2.editTitle', { name: editing.name || '' }) : tr('p2.newTitle') }}</h3>
          <button class="modal-x" :title="tr('p2.close')" @click="editing = null">×</button>
        </div>
        <div class="product-form two-col">
          <label>{{ tr('p2.nameReq') }} <input v-model="form.name" /></label>
          <label>{{ tr('p2.barcode') }} <input v-model="form.barcode" :placeholder="tr('p2.barcodePh')" /></label>
          <label>{{ tr('p2.priceReq') }} <input type="number" step="0.01" v-model.number="form.price" /></label>
          <label>{{ tr('p2.costOpt') }} <input type="number" step="0.01" v-model.number="form.cost" /></label>
          <label>{{ tr('p2.sort') }} <input type="number" v-model.number="form.sort" /></label>
          <label>{{ tr('p2.category') }}
            <input v-model="form.category" list="pos-cats" :placeholder="tr('p2.categoryPh')" />
            <datalist id="pos-cats">
              <option v-for="c in catList" :key="c" :value="c" />
            </datalist>
          </label>
          <label class="checkbox"><input type="checkbox" v-model="form.hot" /> {{ tr('p2.hotRec') }}</label>
          <label>{{ tr('p2.type') }}
            <select v-model="form.kind">
              <option value="single">{{ tr('p2.single') }}</option>
              <option value="bundle">{{ tr('p2.bundle') }}</option>
            </select>
          </label>
          <div class="field">
            <span class="pf-label">{{ tr('p2.status') }}</span>
            <div class="pf-status">
              <button
                type="button"
                class="p-status on"
                :disabled="!editing.id"
                @click="toggle(editing)"
              >
                <i></i>{{ editing.active ? tr('p2.onSale') : tr('p2.offSale') }}
              </button>
              <span class="pf-note">{{ tr('p2.statusHint') }}</span>
            </div>
          </div>
          <div class="pf-image">
            <label class="pf-label">{{ tr('p2.imageOpt') }}</label>
            <input type="file" accept="image/png,image/jpeg" @change="onPickImage" />
            <span class="pf-note">{{ tr('p2.imageHint') }}</span>
          </div>
          <div v-if="form.kind === 'bundle'" style="grid-column: 1 / -1">
            <p class="muted" style="margin: 4px 0">{{ tr('p2.bundleHint') }}</p>
            <div v-for="(b, i) in form.bundle" :key="i" style="display:flex; gap:8px; margin-bottom:6px">
              <select v-model.number="b.componentId" style="flex:1">
                <option :value="0" disabled>{{ tr('p2.pickProduct') }}</option>
                <option v-for="p in singleProducts" :key="p.id" :value="Number(p.id)">{{ p.name }}</option>
              </select>
              <input type="number" min="1" v-model.number="b.qty" style="width:80px" :title="tr('p2.qty')" />
              <input type="number" min="0" v-model.number="b.groupId" style="width:90px" :placeholder="tr('p2.groupPh')" :title="tr('p2.groupTitle')" />
              <button type="button" @click="form.bundle.splice(i, 1)">{{ tr('prod.delete') }}</button>
            </div>
            <button type="button" @click="form.bundle.push({ componentId: 0, qty: 1, groupId: 0 })">{{ tr('p2.addComponent') }}</button>
            <p class="muted" style="margin: 6px 0 0; font-size: 12px">
              {{ tr('p2.groupHint') }}
            </p>
          </div>
        </div>
        
        <div class="modal-actions">
          <button v-if="editing.id" class="link-danger del" @click="remove(editing)">{{ tr('p2.deleteProduct') }}</button>
          <span class="grow"></span>
          <button class="cancel" @click="editing = null">{{ tr('p2.cancel') }}</button>
          <button class="primary save" :disabled="saving" @click="save">{{ saving ? tr('p2.saving') : tr('p2.saveProduct') }}</button>
        </div>
        <p v-if="formError" class="orders-error">{{ formError }}</p>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>

    <CategoryDialog :open="catOpen" @close="catOpen = false" @changed="load" />
    <ImportDialog :open="importOpen" @close="importOpen = false" @done="onImportDone" />
  </section>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr, locale } from '../i18n.js'
import EmptyState from './EmptyState.vue'
import ImportDialog from './ImportDialog.vue'
import CategoryDialog from './CategoryDialog.vue'

const emit = defineEmits(['changed', 'templates'])

const list = ref([])
const keyword = ref('')
const includeInactive = ref(true)
const error = ref('')
const editing = ref(null)
const saving = ref(false)
const formError = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
let toastTimer = null

const form = reactive({ name: '', barcode: '', category: '', price: 0, cost: 0, sort: 0, hot: false })

// v03 P5：工具栏「类目：全部」下拉 + 列表按类目过滤（纯前端，不改查询口径）
const catFilter = ref('')
const catList = computed(() => {
  const s = new Set()
  for (const p of list.value) if (p.category) s.add(p.category)
  return [...s]
})
const shown = computed(() =>
  catFilter.value ? list.value.filter((p) => p.category === catFilter.value) : list.value
)

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toastMsg.value = ''), 2400)
}

let retryTimer = null

// 商品页可能在 token 就绪前挂载过一次，请求 401 后不会自动恢复；
// 这里失败自动重试 2 次（页面空态会显示"重新加载"兜底）。
async function load(attempt = 0) {
  error.value = ''
  try {
    list.value = await api.listProducts(includeInactive.value)
    loadStock()
  } catch (e) {
    error.value = tr('p2.loadProductsFailed', { msg: e.message || '' })
    if (attempt < 2) {
      clearTimeout(retryTimer)
      retryTimer = setTimeout(() => load(attempt + 1), 600 * (attempt + 1))
    }
  }
}

async function onPickImage(e) {
  const file = e.target.files && e.target.files[0]
  if (!file || !editing.value || !editing.value.id) {
    formError.value = tr('p2.saveFirst')
    e.target.value = ''
    return
  }
  try {
    const res = await api.uploadProductImage(editing.value.id, file)
    form.image = res.image
    toast(tr('p2.imageUploaded'))
    await load()
  } catch (err) {
    formError.value = tr('p2.uploadFailed', { msg: err.message || '' })
  } finally {
    e.target.value = ''
  }
}

function openCreate() {
  Object.assign(form, { name: '', barcode: '', category: '', price: 0, cost: 0, sort: 0, hot: false })
  formError.value = ''
  editing.value = { id: 0 }
}

const importOpen = ref(false)
const bomItems = ref([])
const bomCost = ref(0)
const bomSaving = ref(false)
const bomError = ref('')

async function loadBOM(id) {
  bomItems.value = []
  bomCost.value = 0
  bomError.value = ''
  try {
    const rows = await api.productBOM(id)
    bomItems.value = (rows || []).map((b) => ({
      componentId: Number(b.componentId),
      qty: Number(b.qtyMilli) / 1000,
      lossPct: Number(b.lossBp) / 100
    }))
    const p = list.value.find((x) => Number(x.id) === Number(id))
    if (p) bomCost.value = Math.round(Number(p.cost || 0) * 100)
  } catch (e) { bomError.value = e.message || tr('p2.bomLoadFailed') }
}

async function importBOM(e) {
  const file = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!file) return
  const text = await file.text()
  const byName = new Map((list.value || []).map((p) => [p.name, p]))
  const groups = new Map()
  let bad = 0
  for (const line of text.split(/\r?\n/)) {
    const t = line.trim()
    if (!t || t.startsWith('#')) continue
    const cols = t.split(',').map((x) => x.trim())
    if (cols.length < 3) { bad++; continue }
    const [parentName, compName, qtyRaw, lossRaw] = cols
    const parent = byName.get(parentName)
    const comp = byName.get(compName)
    const qty = Number(qtyRaw)
    if (!parent || !comp || !(qty > 0)) { bad++; continue }
    if (!groups.has(parent.id)) groups.set(parent.id, [])
    groups.get(parent.id).push({
      componentId: Number(comp.id),
      qtyMilli: Math.round(qty * 1000),
      lossBp: Math.round(Number(lossRaw || 0) * 100)
    })
  }
  let ok = 0
  for (const [id, rows] of groups) {
    try { await api.setProductBOM(id, rows); ok++ } catch { bad++ }
  }
  toast(tr('p2.bomImported', { ok, bad }), bad > 0)
}

async function saveBOM() {
  bomSaving.value = true
  bomError.value = ''
  try {
    const rows = bomItems.value
      .filter((b) => b.componentId > 0 && b.qty > 0)
      .map((b) => ({ componentId: Number(b.componentId), qtyMilli: Math.round(Number(b.qty) * 1000), lossBp: Math.round(Number(b.lossPct || 0) * 100) }))
    const r = await api.setProductBOM(editing.value.id, rows)
    bomCost.value = (r && r.costMinor) || 0
    toast(tr('p2.bomSaved'))
  } catch (e) {
    bomError.value = e.message || tr('p2.bomSaveFailed')
  } finally { bomSaving.value = false }
}
const catOpen = ref(false)

function openImport() {
  importOpen.value = true
}

async function onImportDone() {
  await load()
  emit('changed')
}

function importTpl() {
  // 打开首次向导 Step1（选业态应用模板），商品导入后本空态自动消失。
  window.dispatchEvent(new CustomEvent('pos:setup'))
}

function openEdit(p) {
  loadSingles()
  Object.assign(form, {
    name: p.name,
    barcode: p.barcode || '',
    category: p.category || '',
    price: p.price,
    cost: p.cost || 0,
    sort: p.sort || 0,
    hot: !!p.hot,
    kind: p.kind || 'single',
    bundle: (p.bundle || []).map((b) => ({ componentId: Number(b.componentId), qty: b.qty, groupId: Number(b.groupId || 0) }))
  })
  formError.value = ''
  editing.value = p
  loadBOM(p.id)
}

const singleProducts = ref([])
const stockMap = ref({})
const alerts = ref([])
const stockTarget = ref(null)
const stockForm = reactive({ qty: 0, warnQty: 0, delta: 0, reason: '' })
const stockError = ref('')

function stockOf(id) { return stockMap.value[String(id)] }
function isLow(id) {
  const s = stockOf(id)
  return !!s && s.warnQty > 0 && s.qty <= s.warnQty
}

async function loadStock() {
  try {
    const [rows, low] = await Promise.all([api.listStock(), api.stockAlerts()])
    const map = {}
    for (const r of rows || []) map[String(r.productId)] = r
    stockMap.value = map
    alerts.value = low || []
  } catch (e) { /* 库存模块不可用时不阻塞商品页 */ }
}

const labelOpen = ref(false)
const labelPick = ref({})
const labelQty = ref(1)
const labelPrinting = ref(false)
const labelError = ref('')
const labelCandidates = ref([])

async function openLabels() {
  labelOpen.value = true
  labelError.value = ''
  labelPick.value = {}
  labelQty.value = 1
  try {
    labelCandidates.value = ((await api.listProducts(false)) || []).filter((p) => (p.kind || 'single') === 'single')
  } catch (e) {
    labelCandidates.value = []
    labelError.value = tr('p2.labelLoadFailed', { msg: e.message || '' })
  }
}

const labelCount = computed(() => {
  const n = Object.keys(labelPick.value).filter((k) => labelPick.value[k]).length
  return n * (Number(labelQty.value) || 0)
})

async function printLabels() {
  labelPrinting.value = true
  labelError.value = ''
  try {
    const ids = Object.keys(labelPick.value).filter((k) => labelPick.value[k]).map(Number)
    const res = await api.printLabels({ productIds: ids, qty: Number(labelQty.value) || 1 })
    labelOpen.value = false
    toast(tr('p2.labelsPrinted', { n: res.printed }))
  } catch (e) {
    labelError.value = tr('p2.printFailed', { msg: e.message || '' })
  } finally {
    labelPrinting.value = false
  }
}

const purchase = ref(null)
const purchaseProducts = ref([])
const purchaseSaving = ref(false)
const purchaseError = ref('')

async function openPurchase() {
  purchase.value = { supplier: '', note: '', items: [{ productId: 0, qty: 1, costYuan: 0 }] }
  purchaseError.value = ''
  // 用全量商品（不受搜索框/下架过滤影响），且不复用 singleProducts（那个列表实际一直是空的）
  try {
    purchaseProducts.value = (await api.listProducts(true)) || []
  } catch (e) {
    purchaseProducts.value = []
    purchaseError.value = tr('p2.listLoadFailed', { msg: e.message || '' })
  }
}

function closePurchase() {
  purchase.value = null
  purchaseError.value = ''
}

function addPurchaseLine() {
  if (purchase.value) purchase.value.items.push({ productId: 0, qty: 1, costYuan: 0 })
}

function yuanCost(it) {
  const n = Number(it.costYuan)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function lineAmount(it) {
  const q = Number(it.qty)
  if (!Number.isFinite(q) || q <= 0) return '—'
  return '¥' + (q * yuanCost(it)).toFixed(2)
}

const purchaseTotal = computed(() => {
  if (!purchase.value) return '0.00'
  const sum = purchase.value.items.reduce((s, it) => {
    const q = Number(it.qty)
    return s + (Number.isFinite(q) && q > 0 ? q * yuanCost(it) : 0)
  }, 0)
  return sum.toFixed(2)
})

const purchaseHint = computed(() => {
  if (!purchase.value) return ''
  const items = purchase.value.items
  if (!items.length) return tr('p2.vAtLeastOne')
  if (items.some((it) => !(Number(it.productId) > 0))) return tr('p2.vPick')
  if (items.some((it) => !(Number(it.qty) > 0))) return tr('p2.vQty')
  if (items.some((it) => Number(it.qty) !== Math.floor(Number(it.qty)))) return tr('p2.vInt')
  if (items.some((it) => Number(it.costYuan) < 0)) return tr('p2.vCost')
  return ''
})

const purchaseValid = computed(() => !!purchase.value && purchase.value.items.length > 0 && purchaseHint.value === '')

async function savePurchase() {
  if (!purchaseValid.value || purchaseSaving.value) return
  purchaseSaving.value = true
  purchaseError.value = ''
  try {
    const items = purchase.value.items.map((it) => ({
      productId: Number(it.productId),
      qty: Number(it.qty),
      // 元 → 分只在这里换算一次，落库一律分
      unitCost: Math.round(yuanCost(it) * 100)
    }))
    await api.createPurchase({
      supplier: (purchase.value.supplier || '').trim(),
      note: (purchase.value.note || '').trim(),
      operator: '',
      items
    })
    closePurchase()
    toast(tr('p2.purchaseSaved'))
    await loadStock()
    emit('changed')
  } catch (e) {
    purchaseError.value = tr('p2.purchaseFailed', { msg: e.message || '' })
  } finally {
    purchaseSaving.value = false
  }
}

const stockMenuOpen = ref(false)
const historyOpen = ref(false)
const historyList = ref([])
const historyLoading = ref(false)
const historyError = ref('')

async function startStocktake() {
  stockMenuOpen.value = false
  await openStocktake()
}

async function openStockHistory() {
  stockMenuOpen.value = false
  historyOpen.value = true
  await loadHistory()
}

async function loadHistory() {
  historyLoading.value = true
  historyError.value = ''
  try {
    const rows = (await api.listStocktakes(20)) || []
    // 列表接口不带明细；差异条数逐张取（本地库，张数很少，逐张并发即可）
    const enriched = await Promise.all(
      rows.map(async (s) => {
        try {
          const d = await api.getStocktake(s.id)
          const items = d.items || []
          return { ...s, items, diffCount: items.filter((i) => i.actualQty !== null && i.diff !== 0).length }
        } catch (e) {
          return { ...s, items: [], diffCount: 0 }
        }
      })
    )
    historyList.value = enriched
  } catch (e) {
    historyList.value = []
    historyError.value = tr('p2.historyLoadFailed', { msg: e.message || '' })
  } finally {
    historyLoading.value = false
  }
}

// 详情直接复用现有盘点弹窗：已关单时输入框与按钮本就 disabled，只读成立
async function openHistoryDetail(s) {
  historyOpen.value = false
  sheetError.value = ''
  if (s.items && s.items.length) {
    sheet.value = s
    return
  }
  try {
    sheet.value = await api.getStocktake(s.id)
  } catch (e) {
    sheetError.value = tr('p2.sheetLoadFailed', { msg: e.message || '' })
  }
}

const sheet = ref(null)
const sheetError = ref('')

async function openStocktake() {
  sheetError.value = ''
  try {
    const list = await api.listStocktakes(10)
    const open = (list || []).find((s) => s.status === 'open')
    sheet.value = open ? await api.getStocktake(open.id) : await api.createStocktake({ note: tr('p2.stocktakeNote'), operator: '' })
  } catch (e) {
    sheetError.value = tr('p2.sheetCreateFailed', { msg: e.message || '' })
  }
}

function setActual(it, raw) {
  const n = raw === '' ? null : Number(raw)
  it.actualQty = n === null || Number.isNaN(n) ? null : n
  it.diff = it.actualQty === null ? 0 : it.actualQty - it.systemQty
}

async function saveActual() {
  try {
    const items = sheet.value.items.filter((i) => i.actualQty !== null).map((i) => ({ productId: i.productId, actualQty: i.actualQty }))
    if (!items.length) { sheetError.value = tr('p2.noActual'); return }
    sheet.value = await api.setStocktakeItems(sheet.value.id, items)
    toast(tr('p2.actualSaved'))
  } catch (e) {
    sheetError.value = tr('p2.saveFailed', { msg: e.message || '' })
  }
}

async function closeSheet() {
  try {
    const items = sheet.value.items.filter((i) => i.actualQty !== null).map((i) => ({ productId: i.productId, actualQty: i.actualQty }))
    if (items.length) await api.setStocktakeItems(sheet.value.id, items)
    sheet.value = await api.closeStocktake(sheet.value.id, locale.value === 'zh-CN' ? '店长' : 'Manager')
    await loadStock()
    toast(tr('p2.stocktakeDone'))
  } catch (e) {
    sheetError.value = tr('p2.closeSheetFailed', { msg: e.message || '' })
  }
}

function openStock(p) {
  stockTarget.value = p
  stockError.value = ''
  const s = stockOf(p.id)
  stockForm.qty = s ? s.qty : 0
  stockForm.warnQty = s ? s.warnQty : 0
  stockForm.delta = 0
  stockForm.reason = ''
}

async function saveStock() {
  try {
    await api.setStock(stockTarget.value.id, { qty: Number(stockForm.qty), warnQty: Number(stockForm.warnQty) })
    stockTarget.value = null
    await loadStock()
    toast(tr('p2.stockSaved'))
  } catch (e) {
    stockError.value = tr('p2.saveFailed', { msg: e.message || '' })
  }
}

async function applyAdjust() {
  try {
    await api.adjustStock(stockTarget.value.id, { delta: Number(stockForm.delta), reason: stockForm.reason })
    stockForm.delta = 0
    stockForm.reason = ''
    await loadStock()
    const s = stockOf(stockTarget.value.id)
    if (s) stockForm.qty = s.qty
    toast(tr('p2.stockAdjusted'))
  } catch (e) {
    stockError.value = tr('p2.adjustFailed', { msg: e.message || '' })
  }
}

async function loadSingles() {
  try {
    const list = await api.listProducts(true)
    singleProducts.value = (list || []).filter((p) => (p.kind || 'single') === 'single')
  } catch (e) { /* 忽略：没有可选组件时用户可先建单品 */ }
}

async function save() {
  saving.value = true
  formError.value = ''
  const body = {
    name: form.name,
    barcode: form.barcode,
    category: form.category,
    price: Number(form.price || 0),
    cost: Number(form.cost || 0),
    sort: Number(form.sort || 0),
    hot: !!form.hot,
    kind: form.kind,
    bundle: (form.bundle || []).map((b) => ({
      componentId: Number(b.componentId),
      qty: Number(b.qty),
      groupId: Number(b.groupId || 0)
    }))
  }
  try {
    if (editing.value.id) {
      await api.updateProduct(editing.value.id, body)
    } else {
      await api.createProduct(body)
    }
    editing.value = null
  form.kind = 'single'
  form.bundle = []
    toast(tr('p2.saved'))
    await load()
    emit('changed')
  } catch (e) {
    formError.value = e.message || tr('p2.saveFailedShort')
  } finally {
    saving.value = false
  }
}

async function toggle(p) {
  try {
    await api.setProductActive(p.id, !p.active)
    await load()
    emit('changed')
  } catch (e) {
    toast(tr('p2.opFailed', { msg: e.message || '' }), true)
  }
}

async function remove(p) {
  try {
    await api.deleteProduct(p.id)
    toast(tr('p2.deleted'))
    await load()
    emit('changed')
  } catch (e) {
    toast(tr('p2.deleteFailed', { msg: e.message || '' }), true)
  }
}

onMounted(() => {
  load()
  window.addEventListener('pos:setup-done', onSetupDone)
})

onUnmounted(() => {
  window.removeEventListener('pos:setup-done', onSetupDone)
})

function onSetupDone() {
  // 首次向导/模板导入完成（setup_done=true）后刷新，空态随数据出现消失。
  load()
}
</script>
