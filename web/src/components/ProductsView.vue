<template>
  <section class="orders products">
    <header class="orders-head">
      <h2>商品管理</h2>
      <span class="grow"></span>
      <button class="btn-line" @click="$emit('templates')">业态模板</button>
      <button class="btn-line" @click="openImport">批量导入 Excel</button>
      <button class="btn-line" @click="openPurchase">入库</button>
      <div class="stock-entry">
        <button class="btn-line" @click="stockMenuOpen = !stockMenuOpen">盘点 ▾</button>
        <div v-if="stockMenuOpen" class="stock-menu-mask" @click="stockMenuOpen = false"></div>
        <div v-if="stockMenuOpen" class="stock-menu">
          <button @click="startStocktake">开始新盘点</button>
          <button @click="openStockHistory">查看历史</button>
        </div>
      </div>
      <button class="primary" @click="openCreate">＋ 新建商品</button>
    </header>

    <div class="filters-row">
      <input class="f-search wide" v-model="keyword" placeholder="搜索名称 / 条码 / 拼音" @input="load" />
      <label class="f-pill">
        <select v-model="catFilter">
          <option value="">类目：全部</option>
          <option v-for="c in catList" :key="c" :value="c">类目：{{ c }}</option>
        </select>
      </label>
      <button class="link-btn" @click="catOpen = true">类目管理</button>
      <span class="bar-hint">批量导入 ≤ 1000 行，模板可下载</span>
      <span class="grow"></span>
    </div>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <p v-if="alerts.length" class="orders-warn">
      有 {{ alerts.length }} 个商品库存偏低：{{ alerts.map((a) => a.name + ' ' + a.qty).join('、') }}
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
          <th>名称</th>
          <th>条码</th>
          <th>类目</th>
          <th class="num">售价</th>
          <th class="num">库存</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id" :class="{ dim: !p.active }">
          <td>
            <img v-if="p.image" :src="'/pos/v1/' + p.image" class="product-thumb" alt="" />
            <span v-else class="product-thumb product-thumb-empty">无图</span>
            {{ p.name }} <span v-if="p.hot" class="chip chip-busy">热销</span>
          </td>
          <td class="mono" :class="{ 'barcode-empty': !p.barcode }">{{ p.barcode || '—' }}</td>
          <td>{{ p.category || '—' }}</td>
          <td class="num">¥{{ p.price.toFixed(2) }}</td>
          <td class="num">
            <template v-if="stockOf(p.id)">
              <span :class="{ 'stock-low': isLow(p.id) }">{{ stockOf(p.id).qty }}</span>
            </template>
            <span v-else class="muted">未跟踪</span>
          </td>
          <td>
            <span class="p-status" :class="p.active ? 'on' : 'off'">
              <i></i>{{ p.active ? '在售' : '停售' }}
            </span>
          </td>
          <td class="ops ops-links">
            <button class="link-brand" @click="openStock(p)">库存</button>
            <button class="link-brand" @click="openEdit(p)">编辑</button>
            <button class="link-muted" @click="toggle(p)">{{ p.active ? '停售' : '上架' }}</button>
            <button class="link-muted" @click="remove(p)">删除</button>
          </td>
        </tr>
        <tr v-if="!list.length" class="empty-row">
          <td colspan="7" class="empty-cell">
            <EmptyState
              title="还没有商品"
              desc="先导入一套示例商品（按业态一键配齐），或直接新增你的第一个商品。"
              :actions="[
                { label: '导入示例商品', primary: true, onClick: importTpl },
                { label: '新增商品', onClick: openCreate },
                { label: '重新加载', onClick: () => load(2) }
              ]"
            />
          </td>
        </tr>
      </tbody>
    </table>
    <div class="orders-foot">
      <span>有订单历史的商品只能下架，不能删除；下架后收银台不可售卖</span>
      <span>共 {{ shown.length }} 个商品</span>
    </div>

    <div v-if="labelOpen" class="mask" @click.self="labelOpen = false">
      <div class="modal" style="max-width: 560px; width: 92vw">
        <h3>打印价签</h3>
        <p class="muted" style="margin: 0 0 8px">选商品、填每款份数；有barcode的会一并打条码。</p>
        <div style="max-height: 44vh; overflow: auto">
          <table class="orders-table">
            <thead><tr><th style="width:44px"></th><th>商品</th><th class="num" style="width:90px">售价</th><th class="num" style="width:90px">份数</th></tr></thead>
            <tbody>
              <tr v-for="p in labelCandidates" :key="p.id">
                <td><input type="checkbox" v-model="labelPick[p.id]" /></td>
                <td>{{ p.name }}</td>
                <td class="num">¥{{ Number(p.price).toFixed(2) }}</td>
                <td class="num"><input type="number" min="1" max="20" v-model.number="labelQty" style="width:70px" /></td>
              </tr>
              <tr v-if="!labelCandidates.length"><td colspan="4" class="empty-cell">还没有在售商品</td></tr>
            </tbody>
          </table>
        </div>
        <p v-if="labelError" class="orders-error">{{ labelError }}</p>
        <div class="modal-actions">
          <button @click="labelOpen = false">取消</button>
          <button class="primary" :disabled="!labelCount || labelPrinting" @click="printLabels">
            {{ labelPrinting ? '打印中…' : '打印 ' + labelCount + ' 张' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="purchase" class="mask" @click.self="closePurchase">
      <div class="modal" style="max-width: 760px; width: 94vw">
        <h3>采购入库</h3>
        <p class="muted" style="margin: 0 0 10px">
          录入本次进货的商品、数量与进价，保存后库存自动增加，并留下进货单与审计记录。
        </p>
        <div class="purchase-head">
          <label>供应商（选填）
            <input v-model="purchase.supplier" placeholder="例如 杭州茶叶批发" />
          </label>
          <label>备注（选填）
            <input v-model="purchase.note" placeholder="例如 九月补货" />
          </label>
        </div>
        <div style="max-height: 44vh; overflow: auto">
          <table class="orders-table">
            <thead>
              <tr>
                <th>商品</th>
                <th style="width: 110px">数量</th>
                <th style="width: 130px">进价（元）</th>
                <th style="width: 110px" class="num">小计</th>
                <th style="width: 64px"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(it, i) in purchase.items" :key="i">
                <td>
                  <select v-model.number="it.productId">
                    <option :value="0">请选择商品</option>
                    <option v-for="p in purchaseProducts" :key="p.id" :value="Number(p.id)">{{ p.name }}</option>
                  </select>
                </td>
                <td><input type="number" min="1" step="1" v-model.number="it.qty" style="width: 92px" /></td>
                <td><input type="number" min="0" step="0.01" v-model.number="it.costYuan" style="width: 112px" /></td>
                <td class="num">{{ lineAmount(it) }}</td>
                <td>
                  <button class="purchase-del" :disabled="purchase.items.length === 1" @click="purchase.items.splice(i, 1)">删除</button>
                </td>
              </tr>
              <tr v-if="!purchase.items.length">
                <td colspan="5" class="empty-cell">至少留一行明细</td>
              </tr>
            </tbody>
          </table>
        </div>
        <button class="purchase-add" @click="addPurchaseLine">＋ 添加一行</button>
        <div class="purchase-foot">
          <span class="muted">合计金额</span>
          <b class="purchase-total">¥{{ purchaseTotal }}</b>
        </div>
        <p v-if="purchaseHint" class="muted" style="margin: 0 0 6px">{{ purchaseHint }}</p>
        <p v-if="purchaseError" class="orders-error">{{ purchaseError }}</p>
        <div class="modal-actions">
          <button @click="closePurchase">取消</button>
          <button class="primary" :disabled="!purchaseValid || purchaseSaving" @click="savePurchase">
            {{ purchaseSaving ? '保存中…' : '保存入库' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="historyOpen" class="mask" @click.self="historyOpen = false">
      <div class="modal" style="max-width: 760px; width: 92vw">
        <h3>盘点历史 <span class="muted">最近 {{ historyList.length }} 张</span></h3>
        <p class="muted" style="margin: 0 0 8px">
          点一行看明细；进行中的单可以继续录实盘，已关单只读。
        </p>
        <div style="max-height: 52vh; overflow: auto">
          <table class="orders-table">
            <thead>
              <tr>
                <th style="width: 78px">单号</th>
                <th style="width: 92px">状态</th>
                <th>时间</th>
                <th style="width: 110px">操作人</th>
                <th style="width: 96px" class="num">差异条数</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in historyList" :key="s.id" class="history-row" @click="openHistoryDetail(s)">
                <td class="mono">#{{ s.id }}</td>
                <td>
                  <span class="chip" :class="s.status === 'done' ? 'chip-ok' : 'chip-busy'">
                    {{ s.status === 'done' ? '已关单' : '进行中' }}
                  </span>
                </td>
                <td class="mono">{{ s.createdAt }}</td>
                <td>{{ s.operator || '—' }}</td>
                <td class="num">{{ s.diffCount }}</td>
              </tr>
              <tr v-if="historyLoading">
                <td colspan="5" class="empty-cell">加载中…</td>
              </tr>
              <tr v-else-if="!historyList.length">
                <td colspan="5" class="empty-cell">还没有盘点单，点下面「开始新盘点」</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="historyError" class="orders-error">{{ historyError }}</p>
        <div class="modal-actions">
          <button @click="historyOpen = false">关闭</button>
          <button class="primary" :disabled="historyLoading" @click="startStocktake">开始新盘点</button>
        </div>
      </div>
    </div>

    <div v-if="sheet" class="mask" @click.self="sheet = null">
      <div class="modal" style="max-width: 640px; width: 92vw">
        <h3>库存盘点 #{{ sheet.id }} <span class="muted">{{ sheet.status === 'done' ? '已关单' : '进行中' }}</span></h3>
        <p class="muted" style="margin: 0 0 8px">
          录实盘数量后关单：系统按「实盘 − 账面」调整库存并留审计。
        </p>
        <div style="max-height: 46vh; overflow: auto">
          <table class="orders-table">
            <thead><tr><th>商品</th><th class="num">账面</th><th class="num">实盘</th><th class="num">差异</th></tr></thead>
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
              <tr v-if="!sheet.items.length"><td colspan="4" class="empty-cell">没有建库存行的商品</td></tr>
            </tbody>
          </table>
        </div>
        <div class="modal-actions">
          <button @click="sheet = null">关闭</button>
          <button :disabled="sheet.status === 'done'" @click="saveActual">保存实盘</button>
          <button class="primary" :disabled="sheet.status === 'done'" @click="closeSheet">关单并调账</button>
        </div>
        <p v-if="sheetError" class="orders-error">{{ sheetError }}</p>
      </div>
    </div>

    <div v-if="stockTarget" class="mask" @click.self="stockTarget = null">
      <div class="modal">
        <h3>库存 · {{ stockTarget.name }}</h3>
        <div class="settings-form" style="max-width: 320px">
          <label>当前数量
            <input type="number" v-model.number="stockForm.qty" />
          </label>
          <label>告警阈值（低于等于该值提示补货，0=不提示）
            <input type="number" min="0" v-model.number="stockForm.warnQty" />
          </label>
        </div>
        <div style="display:flex; gap:8px; margin: 8px 0; flex-wrap: wrap">
          <input type="number" v-model.number="stockForm.delta" placeholder="±数量" style="width: 110px" />
          <input v-model="stockForm.reason" placeholder="原因（入库/报损）" style="flex:1" />
          <button :disabled="!stockForm.delta" @click="applyAdjust">调整</button>
        </div>
        <div class="modal-actions">
          <button @click="stockTarget = null">取消</button>
          <button class="primary" @click="saveStock">保存</button>
        </div>
        <p v-if="stockError" class="orders-error">{{ stockError }}</p>
      </div>
    </div>

    <div v-if="editing" class="mask light-mask" @click.self="editing = null">
      <div class="modal product-modal edit-modal">
        <div class="pay-head">
          <h3>{{ editing.id ? '编辑商品 · ' + (editing.name || '') : '新建商品' }}</h3>
          <button class="modal-x" title="关闭" @click="editing = null">×</button>
        </div>
        <div class="product-form two-col">
          <label>名称* <input v-model="form.name" /></label>
          <label>条码 <input v-model="form.barcode" placeholder="留空则不打印条码" /></label>
          <label>售价* <input type="number" step="0.01" v-model.number="form.price" /></label>
          <label>进价（可选） <input type="number" step="0.01" v-model.number="form.cost" /></label>
          <label>排序 <input type="number" v-model.number="form.sort" /></label>
          <label>类目
            <input v-model="form.category" list="pos-cats" placeholder="选择或输入类目" />
            <datalist id="pos-cats">
              <option v-for="c in catList" :key="c" :value="c" />
            </datalist>
          </label>
          <label class="checkbox"><input type="checkbox" v-model="form.hot" /> 热销推荐</label>
          <label>类型
            <select v-model="form.kind">
              <option value="single">单品</option>
              <option value="bundle">套餐（组合）</option>
            </select>
          </label>
          <div class="field">
            <span class="pf-label">状态</span>
            <div class="pf-status">
              <button
                type="button"
                class="p-status on"
                :disabled="!editing.id"
                @click="toggle(editing)"
              >
                <i></i>{{ editing.active ? '在售' : '停售' }}
              </button>
              <span class="pf-note">切换后收银台即时生效</span>
            </div>
          </div>
          <div class="pf-image">
            <label class="pf-label">商品图片（可选）</label>
            <input type="file" accept="image/png,image/jpeg" @change="onPickImage" />
            <span class="pf-note">支持 jpg / png，≤1MB</span>
          </div>
          <div v-if="form.kind === 'bundle'" style="grid-column: 1 / -1">
            <p class="muted" style="margin: 4px 0">套餐组件（后厨按组件分档口出单）</p>
            <div v-for="(b, i) in form.bundle" :key="i" style="display:flex; gap:8px; margin-bottom:6px">
              <select v-model.number="b.componentId" style="flex:1">
                <option :value="0" disabled>选择商品</option>
                <option v-for="p in singleProducts" :key="p.id" :value="Number(p.id)">{{ p.name }}</option>
              </select>
              <input type="number" min="1" v-model.number="b.qty" style="width:80px" title="数量" />
              <input type="number" min="0" v-model.number="b.groupId" style="width:90px" placeholder="选项组" title="选项组：同号互为替代，0=固定" />
              <button type="button" @click="form.bundle.splice(i, 1)">删除</button>
            </div>
            <button type="button" @click="form.bundle.push({ componentId: 0, qty: 1, groupId: 0 })">+ 添加组件</button>
            <p class="muted" style="margin: 6px 0 0; font-size: 12px">
              选项组填 1、2…：同组商品互为替代（如奶茶/果茶），下单时选一个；填 0 表示每次都有。
            </p>
          </div>
        </div>
        <div class="modal-actions">
          <button v-if="editing.id" class="link-danger del" @click="remove(editing)">删除商品</button>
          <span class="grow"></span>
          <button class="cancel" @click="editing = null">取消</button>
          <button class="primary save" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存商品' }}</button>
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
    error.value = '加载商品失败：' + (e.message || '未知错误')
    if (attempt < 2) {
      clearTimeout(retryTimer)
      retryTimer = setTimeout(() => load(attempt + 1), 600 * (attempt + 1))
    }
  }
}

async function onPickImage(e) {
  const file = e.target.files && e.target.files[0]
  if (!file || !editing.value || !editing.value.id) {
    formError.value = '先保存商品，再上传图片'
    e.target.value = ''
    return
  }
  try {
    const res = await api.uploadProductImage(editing.value.id, file)
    form.image = res.image
    toast('图片已上传')
    await load()
  } catch (err) {
    formError.value = '上传失败：' + (err.message || '未知错误')
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
    labelError.value = '商品加载失败：' + (e.message || '未知错误')
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
    toast('已打印 ' + res.printed + ' 张价签')
  } catch (e) {
    labelError.value = '打印失败：' + (e.message || '未知错误')
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
    purchaseError.value = '商品列表加载失败：' + (e.message || '未知错误')
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
  if (!items.length) return '至少需要一行明细'
  if (items.some((it) => !(Number(it.productId) > 0))) return '每行都要选择商品'
  if (items.some((it) => !(Number(it.qty) > 0))) return '数量必须大于 0'
  if (items.some((it) => Number(it.qty) !== Math.floor(Number(it.qty)))) return '数量必须是整数'
  if (items.some((it) => Number(it.costYuan) < 0)) return '进价不能为负数'
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
    toast('入库成功，库存已更新')
    await loadStock()
    emit('changed')
  } catch (e) {
    purchaseError.value = '入库失败：' + (e.message || '未知错误')
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
    historyError.value = '读取盘点历史失败：' + (e.message || '未知错误')
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
    sheetError.value = '读取盘点单失败：' + (e.message || '未知错误')
  }
}

const sheet = ref(null)
const sheetError = ref('')

async function openStocktake() {
  sheetError.value = ''
  try {
    const list = await api.listStocktakes(10)
    const open = (list || []).find((s) => s.status === 'open')
    sheet.value = open ? await api.getStocktake(open.id) : await api.createStocktake({ note: '盘点', operator: '' })
  } catch (e) {
    sheetError.value = '生成盘点单失败：' + (e.message || '未知错误')
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
    if (!items.length) { sheetError.value = '还没有录入任何实盘数量'; return }
    sheet.value = await api.setStocktakeItems(sheet.value.id, items)
    toast('实盘已保存')
  } catch (e) {
    sheetError.value = '保存失败：' + (e.message || '未知错误')
  }
}

async function closeSheet() {
  try {
    const items = sheet.value.items.filter((i) => i.actualQty !== null).map((i) => ({ productId: i.productId, actualQty: i.actualQty }))
    if (items.length) await api.setStocktakeItems(sheet.value.id, items)
    sheet.value = await api.closeStocktake(sheet.value.id, '店长')
    await loadStock()
    toast('盘点完成，库存已按实盘调整')
  } catch (e) {
    sheetError.value = '关单失败：' + (e.message || '未知错误')
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
    toast('库存已保存')
  } catch (e) {
    stockError.value = '保存失败：' + (e.message || '未知错误')
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
    toast('库存已调整')
  } catch (e) {
    stockError.value = '调整失败：' + (e.message || '未知错误')
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
    hot: !!form.hot
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
    toast('已保存')
    await load()
    emit('changed')
  } catch (e) {
    formError.value = e.message || '保存失败'
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
    toast('操作失败：' + (e.message || ''), true)
  }
}

async function remove(p) {
  try {
    await api.deleteProduct(p.id)
    toast('已删除')
    await load()
    emit('changed')
  } catch (e) {
    toast('删除失败：' + (e.message || ''), true)
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
