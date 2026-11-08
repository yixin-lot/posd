<template>
  <section class="orders">
    <header class="orders-head">
      <h2>{{ tr('set.title') }}</h2>
      <span class="grow"></span>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <!-- v03 P7：左 200px 导航卡 + 右内容卡，按分组切换 -->
    <div class="settings-shell">
      <aside class="settings-nav">
        <button
          v-for="t in tabs"
          :key="t.key"
          :class="{ active: tab === t.key }"
          @click="tab = t.key"
        >
          {{ tr('set.tab.' + t.key) }}
        </button>
      </aside>

      <div class="settings-body">
        <div v-show="tab === 'shop'" class="shift-card">
          <h3>{{ tr('set.receiptHeader') }}</h3>
          <div class="settings-form">
            <label>{{ tr('set.shopName') }}<input v-model="form.shop_name" maxlength="20" :placeholder="tr('setx.shopNamePh')" /></label>
            <label>{{ tr('set.receiptFooter') }}<input v-model="form.receipt_footer" maxlength="60" :placeholder="tr('setx.receiptFooterPh')" /></label>
            <label>{{ tr('set.phone') }}<input v-model="form.shop_phone" maxlength="20" :placeholder="tr('setx.optional')" /></label>
            <label>{{ tr('set.footer') }}<input v-model="form.shop_footer" maxlength="30" :placeholder="tr('setx.shopFooterPh')" /></label>
          </div>
        </div>

        <div v-show="tab === 'cashier'" class="shift-card">
          <h3>{{ tr('setx.refundStockTitle') }}</h3>
          <label>{{ tr('set.deductTiming') }}
            <select v-model="form.stock_deduct_timing">
              <option value="on_order">{{ tr('set.deductOnOrder') }}</option>
              <option value="on_serve">{{ tr('set.deductOnServe') }}</option>
            </select>
          </label>
          <label v-if="form.stock_deduct_timing === 'on_serve'">{{ tr('set.serveTrigger') }}
            <select v-model="form.serve_trigger">
              <option value="auto_print">{{ tr('set.triggerPrint') }}</option>
              <option value="manual">{{ tr('set.triggerManual') }}</option>
            </select>
          </label>
          <label class="check">
            <input type="checkbox" :checked="form.refund_restock === 'true'" @change="form.refund_restock = $event.target.checked ? 'true' : 'false'" />
            {{ tr('setx.refundStockHint') }}
          </label>

          <h3>{{ tr('set.rounding') }}</h3>
          <label class="perm-row">
            {{ tr('setx.rounding') }}
            <select v-model="form.round_mode" style="margin-left: 8px">
              <option value="none">{{ tr('set.roundNone') }}</option>
              <option value="jiao">{{ tr('set.roundJiao') }}</option>
              <option value="yuan">{{ tr('set.roundYuan') }}</option>
            </select>
          </label>

          <h3 style="margin-top: 18px">{{ tr('set.points') }}</h3>
          <div class="settings-form">
            <label>{{ tr('setx.pointsPerYuan') }}
              <input v-model="form.points_earn_per_yuan" type="number" min="0" step="1" />
            </label>
            <label>{{ tr('setx.pointsValue') }}
              <input v-model="form.points_per_yuan" type="number" min="1" step="1" />
            </label>
            <label>{{ tr('setx.pointsExpiry') }}
              <input v-model="form.points_expire_months" type="number" min="0" step="1" />
            </label>
            <label>{{ tr('setx.tiers') }}
              <input v-model="form.member_tiers" :placeholder="tr('setx.tiersPh')" />
            </label>
            <label>{{ tr('setx.topupGift') }}
              <input v-model="form.topup_bonus_rules" placeholder="500:50,1000:150" />
            </label>
          </div>
        </div>

        <div v-show="tab === 'account'" class="shift-card">
          <h3>{{ tr('set.perms') }}</h3>
          <p class="muted" style="margin: 0 0 10px">{{ tr('set.permsHint') }}</p>
          <label class="perm-row">
            <input type="checkbox" v-model="perms.refund" />
            {{ tr('setx.allowRefund') }}
          </label>
          <label class="perm-row">
            <input type="checkbox" v-model="perms.discount" />
            {{ tr('setx.allowDiscount') }}
          </label>
          <label class="perm-row">
            <input type="checkbox" v-model="perms.price" />
            {{ tr('setx.allowPrice') }}
          </label>
          <label class="perm-row">
            <input type="checkbox" v-model="perms.void" />
            {{ tr('setx.allowVoid') }}
          </label>
        </div>

        <div v-show="tab === 'print'">
          <div class="shift-card print-card">
            <h3>{{ tr('set.print') }}</h3>
            <p class="settings-sub">{{ tr('set.printHint') }}</p>

            <div class="set-label">{{ tr('set.printMode') }}</div>
            <div class="print-mode-grid">
              <button
                v-for="m in printModes"
                :key="m.key"
                type="button"
                class="print-mode-opt"
                :class="{ on: form.print_mode === m.key }"
                @click="form.print_mode = m.key"
              >
                <span class="pm-check" v-if="form.print_mode === m.key">✓</span>
                <i class="pm-radio"></i>
                <b>{{ tr(m.label) }}</b>
                <span class="pm-desc">{{ tr(m.desc) }}</span>
                <span class="pm-desc">{{ tr(m.desc2) }}</span>
              </button>
            </div>
            <p class="muted mini">
              {{ tr('setx.printNoneHint') }}
            </p>

            <div class="set-label">{{ tr('set.printer') }}</div>
            <div class="printer-row">
              <label class="printer-port">{{ tr('setx.port') }}
                <select v-model="portMode">
                  <option value="auto">{{ tr('set.autoDetect') }}</option>
                  <option value="custom">{{ tr('set.manual') }}</option>
                </select>
              </label>
              <input
                v-if="portMode === 'custom'"
                class="printer-input"
                v-model="receiptTarget"
                maxlength="60"
                :placeholder="tr('setx.detectPh')"
                @input="checkReceiptInput"
              />
              <span class="printer-state" :class="receiptState.ok ? 'ok' : 'off'">
                <i></i>{{ receiptState.text }}
              </span>
              <button
                class="btn-line"
                :disabled="receiptTesting || (portMode === 'custom' && !printerValid(receiptTarget))"
                @click="testReceiptPrinter"
              >
                {{ receiptTesting ? tr('setx.testing') : tr('setx.testPrint') }}
              </button>
              <span class="muted mini">{{ tr('setx.template58') }}</span>
            </div>
            <p v-if="receiptHint" class="printer-hint">{{ receiptHint }}</p>
            <p v-if="receiptTestMsg" class="muted mini">{{ receiptTestMsg }}</p>

            <div class="set-label">{{ tr('set.cloudApi') }}</div>
            <div class="cloud-grid">
              <label>{{ tr('set.serverUrl') }}<input disabled placeholder="https://print.example.com" /></label>
              <label>{{ tr('set.deviceSn') }}<input disabled placeholder="—" /></label>
              <label>{{ tr('set.apiKey') }}<input disabled :placeholder="tr('setx.blankDisabled')" /></label>
              <label>{{ tr('set.apiSecret') }}<input disabled :placeholder="tr('setx.blankDisabled')" /></label>
            </div>
            <p class="muted mini">{{ tr('set.cloudHint') }}</p>
          </div>

          <div class="shift-card">
  <h3>{{ tr('set.intl') }}</h3>
          <div class="settings-form">
            <label>{{ tr('set.language') }}
              <select v-model="form.locale" style="margin-left: 8px">
                <option value="en-US">English (US)</option>
                <option value="zh-CN">{{ tr('setx.zhOption') }}</option>
              </select>
            </label>
            <label>{{ tr('set.currency') }}
              <select v-model="form.currency" style="margin-left: 8px">
                <option value="USD">USD</option>
                <option value="CNY">CNY</option>
                <option value="EUR">EUR</option>
                <option value="GBP">GBP</option>
                <option value="AUD">AUD</option>
                <option value="CAD">CAD</option>
              </select>
            </label>
            <label>{{ tr('set.symbol') }}<input v-model="form.currency_symbol" maxlength="3" placeholder="SYM" /></label>
            <label>{{ tr('set.timezone') }}
              <select v-model="form.timezone" style="margin-left: 8px">
                <option value="America/New_York">America/New_York</option>
                <option value="America/Chicago">America/Chicago</option>
                <option value="America/Los_Angeles">America/Los_Angeles</option>
                <option value="Asia/Shanghai">Asia/Shanghai</option>
                <option value="Europe/London">Europe/London</option>
                <option value="Australia/Sydney">Australia/Sydney</option>
              </select>
            </label>
            <label>{{ tr('set.timeFormat') }}
              <select v-model="form.time_format" style="margin-left: 8px">
                <option value="auto">{{ tr('set.followLang') }}</option>
                <option value="12h">{{ tr('set.h12') }}</option>
                <option value="24h">{{ tr('set.h24') }}</option>
              </select>
            </label>
            <label>{{ tr('set.taxRate') }}<input v-model="form.tax_rate" type="number" min="0" step="0.01" placeholder="0" /></label>
            <label class="check">
              <input type="checkbox" :checked="form.tax_inclusive === 'true'" @change="form.tax_inclusive = $event.target.checked ? 'true' : 'false'" />
              {{ tr('set.taxInclusive') }}
            </label>
          </div>

          <h3>{{ tr('set.kitchen') }}</h3>
            <p class="muted" style="margin: 0 0 10px">
              {{ tr('setx.kitchenHint') }}
            </p>
            <div class="settings-form">
              <label>{{ tr('set.kitchenPrinter') }}<input v-model="form.kitchen_printer" maxlength="40" :placeholder="tr('setx.kitchenPrinterPh')" /></label>
              <label>{{ tr('set.kitchenCats') }}<input v-model="form.kitchen_categories" maxlength="60" :placeholder="tr('setx.kitchenCatsPh2')" /></label>
            </div>
            <h3 style="margin-top: 18px">{{ tr('set.stations') }}</h3>
            <p class="muted" style="margin: 0 0 10px">{{ tr('set.stationsHint') }}</p>
            <table class="orders-table tbl" style="table-layout: fixed">
              <colgroup>
                <col style="width: 24%" /><col style="width: 34%" /><col style="width: 30%" /><col style="width: 12%" />
              </colgroup>
              <thead>
                <tr><th>{{ tr('set.station') }}</th><th>{{ tr('set.printer') }}</th><th>{{ tr('set.categories') }}</th><th></th></tr>
              </thead>
              <tbody>
                <tr v-for="st in stations" :key="st.ID">
                  <td>{{ st.Name }}</td>
                  <td class="mono">{{ st.Printer }}</td>
                  <td>{{ st.Categories || tr('setx.all') }}</td>
                  <td><button class="station-del" @click="removeStation(st)">{{ tr('set.delete') }}</button></td>
                </tr>
                <tr v-if="!stations.length"><td colspan="4" class="empty-row">{{ tr('set.noStations') }}</td></tr>
              </tbody>
            </table>
            <div class="station-form">
              <label>{{ tr('set.stationName') }}<input v-model="newStation.name" maxlength="20" :placeholder="tr('setx.stationNamePh')" /></label>
              <label>{{ tr('set.printerName') }}<input v-model="newStation.printer" maxlength="60" :placeholder="tr('setx.detectPh')" @input="checkPrinterInput" /></label>
              <label>{{ tr('setx.catsLabel') }}<input v-model="newStation.categories" :placeholder="tr('setx.catsPh')" /></label>
            </div>
            <p v-if="printerHint" class="printer-hint">{{ printerHint }}</p>
            <p class="muted" style="margin: 6px 0 8px; font-size: 12px">
              {{ tr('setx.tcpHint') }}
            </p>
            <div class="station-actions">
              <button class="primary" :disabled="!newStation.name || !printerValid(newStation.printer)" @click="addStation">{{ tr('set.addStation') }}</button>
            </div>
            <p v-if="stationError" class="orders-error">{{ stationError }}</p>
          </div>

          <div class="shift-card">
            <h3>{{ tr('set.queue') }}</h3>
            <p class="muted" style="margin: 0 0 10px">{{ tr('set.queueHint') }}</p>
            <p v-if="jobsError" class="orders-error">{{ jobsError }}</p>
            <table class="orders-table tbl" style="table-layout: fixed">
              <colgroup>
                <col style="width: 26%" /><col style="width: 20%" /><col style="width: 18%" /><col style="width: 14%" /><col style="width: 22%" />
              </colgroup>
              <thead>
                <tr><th>{{ tr('set.order') }}</th><th>{{ tr('set.target') }}</th><th>{{ tr('set.status') }}</th><th>{{ tr('set.attempts') }}</th><th>{{ tr('set.time') }}</th></tr>
              </thead>
              <tbody>
                <tr v-for="j in printJobs" :key="j.id">
                  <td class="mono">{{ j.orderNo }}</td>
                  <td>{{ jobTarget(j) }}</td>
                  <td><span class="tag" :class="jobTag(j.status)">{{ jobText(j.status) }}</span></td>
                  <td>{{ j.attempts }}</td>
                  <td class="mono">{{ jobTime(j.createdAt) }}</td>
                </tr>
                <tr v-if="!printJobs.length">
                  <td colspan="5" class="empty-row">{{ tr('set.noJobs') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-show="tab === 'about'" class="shift-card">
          <h3>{{ tr('set.about') }}</h3>
          <div class="shift-info">
            <div><span>{{ tr('set.version') }}</span><b>v0.3.1</b></div>
            <div><span>{{ tr('set.mode') }}</span><b>{{ tr('set.modeStandalone') }}</b></div>
            <div><span>{{ tr('set.dataPath') }}</span><b>{{ tr('set.dataPathLocal') }}</b></div>
            <div><span>{{ tr('set.channel') }}</span><b>{{ tr('setx.channelLocal') }}</b></div>
          </div>
          <p class="muted" style="margin: 10px 0 0">{{ tr('set.aboutNote') }}</p>

          <div class="set-label" style="margin-top: 14px">{{ tr('set.lan') }}</div>
          <p class="muted" style="margin: 0 0 6px">{{ tr('set.lanHint') }}</p>
          <div class="lan-list">
            <div v-for="u in netUrls" :key="u" class="lan-row">
              <a :href="u" target="_blank" rel="noopener" class="lan-url">{{ u }}</a>
              <button type="button" class="btn-line sm" @click="copyUrl(u)">{{ copied === u ? tr('set.copied') : tr('set.copy') }}</button>
            </div>
            <p v-if="!lanEnabled" class="muted mini">{{ tr('set.lanDisabledHint') }}</p>
            <button type="button" class="btn-line sm" style="margin: 4px 0" @click="allowFirewall">{{ tr('set.firewallAllow') }}</button>
            <p v-if="fwMsg" class="muted mini">{{ fwMsg }}</p>
            <p v-else-if="!netUrls.length" class="muted mini">{{ tr('set.lanNone') }}</p>
          </div>

          <div class="set-label" style="margin-top: 14px">{{ tr('set.repo') }}</div>
          <p style="margin: 0">
            <a :href="repoUrl" target="_blank" rel="noopener">{{ repoLabel }}</a>
            · <a :href="repoUrl + '/stargazers'" target="_blank" rel="noopener">★ {{ tr('set.star') }}</a>
            · Apache-2.0
          </p>
        </div>

        <div class="settings-actions">
          <button class="primary" :disabled="!isManager || saving" @click="save">
            {{ saving ? tr('setx.saving') : tr('setx.save') }}
          </button>
          <span v-if="!isManager" class="muted">{{ tr('set.managerOnly') }}</span>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'
import { t as tr, locale } from '../i18n.js'

const props = defineProps({ role: { type: String, default: 'cashier' } })

const form = reactive({
  shop_name: '', shop_phone: '', shop_footer: '', receipt_footer: '', round_mode: 'none',
  kitchen_printer: '', kitchen_categories: '', print_mode: 'local',
  locale: 'en-US', currency: 'USD', currency_symbol: '$', timezone: 'America/New_York',
  time_format: 'auto', tax_rate: '0', tax_inclusive: 'true', refund_restock: 'false', stock_deduct_timing: 'on_order', serve_trigger: 'auto_print'
})

// v03 P7 左侧导航卡（分组切换）与打印模式三选项
const tab = ref('print')
const tabs = [
  { key: 'print' },
  { key: 'shop' },
  { key: 'cashier' },
  { key: 'account' },
  { key: 'about' }
]
const printModes = [
  { key: 'local', label: 'setx.modeLocal', desc: 'setx.modeLocalDesc', desc2: 'setx.modeLocalDesc2' },
  { key: 'cloud', label: 'setx.modeCloud', desc: 'setx.modeCloudDesc', desc2: 'setx.modeCloudDesc2' },
  { key: 'none', label: 'setx.modeNone', desc: 'setx.modeNoneDesc', desc2: 'setx.modeNoneDesc2' }
]
// v03 P7：打印机「端口」下拉，自动检测为默认，自定义时才手填地址
const portMode = ref('auto')
const perms = reactive({ refund: false, discount: false, price: false, void: false })
const loading = ref(false)
const netUrls = ref([])
const lanEnabled = ref(false)
const fwMsg = ref('')
// 国内默认走 Gitee（GitHub 常不可达），国际版走 GitHub
const repoUrl = computed(() => (locale.value === 'zh-CN' ? 'https://gitee.com/yixin-lot/posd' : 'https://github.com/yixin-lot/posd'))
const repoLabel = computed(() => (locale.value === 'zh-CN' ? 'gitee.com/yixin-lot/posd' : 'github.com/yixin-lot/posd'))
const copied = ref('')
const saving = ref(false)
const error = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
let timer = null

const isManager = computed(() => props.role === 'manager')
const stations = ref([])
const printJobs = ref([])
const jobsError = ref('')
const newStation = reactive({ name: '', printer: '', categories: '' })
const stationError = ref('')
const printerHint = ref('')
const receiptTarget = ref('')
const receiptTesting = ref(false)
const receiptTestMsg = ref('')
const receiptHint = ref('')

// 打印机状态胶囊：测试通过才算「就绪」，未指定则提示落文件调试
const receiptState = computed(() => {
  const v = String(receiptTarget.value || '').trim()
  if (!v) return { ok: false, text: tr('setx.notSpecified') }
  if (receiptTestMsg.value.startsWith('✓')) return { ok: true, text: tr('setx.ready') + ' · ' + v }
  return { ok: false, text: tr('setx.pending') }
})

async function testReceiptPrinter() {
  receiptTesting.value = true
  receiptTestMsg.value = ''
  try {
    const r = await api.testPrinter(receiptTarget.value)
    receiptTestMsg.value = (r.ok ? '✓ ' : '✗ ') + (r.message || '')
  } catch (e) {
    receiptTestMsg.value = '✗ ' + (e.message || tr('setx.testFailed'))
  } finally {
    receiptTesting.value = false
  }
}
function checkReceiptInput() {
  const v = String(receiptTarget.value || '').trim()
  receiptHint.value = (v.startsWith('tcp://') && !printerValid(v))
    ? tr('setx.tcpFormat')
    : ''
}

// 打印机名：Windows 队列名（如 POS-58）或 tcp://host:port（网口打印机）
function printerValid(p) {
  const v = String(p || '').trim()
  if (!v) return false
  if (v.startsWith('tcp://')) {
    return /^tcp:\/\/[^/\s]+:\d{1,5}$/.test(v)
  }
  return v.length <= 40
}

function checkPrinterInput() {
  const v = String(newStation.printer || '').trim()
  printerHint.value = (v.startsWith('tcp://') && !printerValid(v))
    ? tr('setx.tcpFormat')
    : ''
}

async function loadStations() {
  try {
    stations.value = await api.listStations()
  } catch (e) {
    stationError.value = tr('setx.loadStationsFailed')
  }
}

// 打印队列（GET /pos/v1/print/jobs）：最近 10 条，只读
async function loadPrintJobs() {
  jobsError.value = ''
  try {
    const data = await api.listPrintJobs()
    printJobs.value = Array.isArray(data && data.jobs) ? data.jobs : []
  } catch (e) {
    jobsError.value = tr('setx.loadJobsFailed')
    printJobs.value = []
  }
}
function jobTarget(j) {
  // 后端 target 枚举：front（前台小票）/ kitchen（后厨单）/ report（交接单）/ void（作废单）。
  // 指定了打印机时优先显示打印机名（一笔单可以同时进两台机器）。
  if (j.printer) return j.printer
  return { front: tr('setx.targetReceipt'), receipt: tr('setx.targetReceipt'), kitchen: tr('setx.targetKitchen'), void: tr('setx.targetVoid'), report: tr('setx.targetReport') }[j.target] || j.target || '—'
}
function jobText(s) {
  return { queued: tr('setx.jobQueued'), printing: tr('setx.jobPrinting'), done: tr('setx.jobDone'), dead: tr('setx.jobDead') }[s] || s || '—'
}
function jobTag(s) {
  if (s === 'done') return 'tag-paid'
  if (s === 'dead') return 'tag-bad'
  if (s === 'printing') return 'tag-pending'
  return 'tag-stop'
}
function jobTime(ms) {
  if (!ms) return '—'
  const d = new Date(Number(ms))
  const p = (n) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

async function addStation() {
  stationError.value = ''
  const v = String(newStation.printer || '').trim()
  if (v.startsWith('tcp://') && !printerValid(v)) {
    printerHint.value = tr('setx.tcpFormat')
    return
  }
  try {
    await api.createStation({ ...newStation, printer: v })
    newStation.name = ''
    newStation.printer = ''
    newStation.categories = ''
    printerHint.value = ''
    await loadStations()
  } catch (e) {
    stationError.value = tr('setx.createFailed', { msg: e.message || '' })
  }
}

async function removeStation(st) {
  if (!window.confirm(tr('setx.deleteConfirm', { name: st.name }))) return
  try {
    await api.deleteStation(st.ID)
    await loadStations()
  } catch (e) {
    stationError.value = tr('setx.deleteFailed', { msg: e.message || '' })
  }
}

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(timer)
  timer = setTimeout(() => (toastMsg.value = ''), 2400)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getSettings()
    form.shop_name = data.shop_name || ''
    form.shop_phone = data.shop_phone || ''
    form.shop_footer = data.shop_footer || ''
    form.receipt_footer = data.receipt_footer || ''
    form.round_mode = data.round_mode || 'none'
    form.points_earn_per_yuan = data.points_earn_per_yuan || '1'
    form.points_per_yuan = data.points_per_yuan || '100'
    form.points_expire_months = data.points_expire_months || '0'
    form.member_tiers = data.member_tiers || ''
    form.topup_bonus_rules = data.topup_bonus_rules || ''
    form.kitchen_printer = data.kitchen_printer || ''
    form.kitchen_categories = data.kitchen_categories || ''
    form.print_mode = data.print_mode || 'local'
    form.locale = data.locale || 'en-US'
    form.currency = data.currency || 'USD'
    form.currency_symbol = data.currency_symbol || '$'
    form.timezone = data.timezone || 'America/New_York'
    form.time_format = data.time_format || 'auto'
    form.tax_rate = data.tax_rate || '0'
    form.tax_inclusive = data.tax_inclusive || 'true'
    form.refund_restock = data.refund_restock || 'false'
    form.stock_deduct_timing = data.stock_deduct_timing || 'on_order'
    form.serve_trigger = data.serve_trigger || 'auto_print'
    perms.refund = data.perm_refund_cashier === 'true'
    perms.discount = data.perm_discount_cashier === 'true'
    perms.price = data.perm_change_price_cashier === 'true'
    perms.void = data.perm_void_cashier === 'true'
  } catch (e) {
    error.value = tr('setx.loadFailed', { msg: e.message || '' })
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const data = await api.saveSettings({
      ...form,
      perm_refund_cashier: perms.refund ? 'true' : 'false',
      perm_discount_cashier: perms.discount ? 'true' : 'false',
      perm_change_price_cashier: perms.price ? 'true' : 'false',
      perm_void_cashier: perms.void ? 'true' : 'false',
      round_mode: form.round_mode,
      points_earn_per_yuan: form.points_earn_per_yuan,
      points_per_yuan: form.points_per_yuan,
      points_expire_months: form.points_expire_months,
      member_tiers: form.member_tiers,
      kitchen_printer: form.kitchen_printer,
      kitchen_categories: form.kitchen_categories
    })
    Object.assign(form, { shop_name: data.shop_name, shop_phone: data.shop_phone, shop_footer: data.shop_footer, receipt_footer: data.receipt_footer || '', round_mode: data.round_mode || 'none' })
    perms.refund = data.perm_refund_cashier === 'true'
    perms.discount = data.perm_discount_cashier === 'true'
    perms.price = data.perm_change_price_cashier === 'true'
    perms.void = data.perm_void_cashier === 'true'
    toast(tr('setx.saved'))
  } catch (e) {
    toast(tr('setx.saveFailed', { msg: e.message || '' }), true)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
  loadPrintJobs()
  loadStations()
  api.systemNet().then((r) => { netUrls.value = (r && r.urls) || []; lanEnabled.value = !!(r && r.lanEnabled) }).catch(() => {})
})

async function allowFirewall() {
  fwMsg.value = tr('set.firewallWorking')
  try {
    const r = await api.allowFirewall()
    fwMsg.value = (r && r.message) || ''
  } catch (e) {
    fwMsg.value = (e && e.message) || tr('set.firewallFailed')
  }
}

async function copyUrl(u) {
  try {
    await navigator.clipboard.writeText(u)
    copied.value = u
    setTimeout(() => { copied.value = '' }, 1500)
  } catch { copied.value = '' }
}
</script>

