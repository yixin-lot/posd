<template>
  <section class="orders">
    <header class="orders-head">
      <h2>设置</h2>
      <span class="muted">改完保存，下一张小票立即生效</span>
      <span class="grow"></span>
      <button @click="load" :disabled="loading">{{ loading ? '读取中…' : '重新读取' }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <div class="shift-card" style="max-width: 720px">
      <h3>小票抬头</h3>
      <div class="settings-form">
        <label>店名<input v-model="form.shop_name" maxlength="20" placeholder="例如：楼下奶茶" /></label>
        <label>小票页脚<input v-model="form.receipt_footer" maxlength="60" placeholder="如：谢谢惠顾 / 电话 138xxxx（可留空）" /></label>
        <label>电话<input v-model="form.shop_phone" maxlength="20" placeholder="选填" /></label>
        <label>脚注<input v-model="form.shop_footer" maxlength="30" placeholder="例如：谢谢光临，欢迎再来" /></label>
      </div>

      <h3 style="margin-top: 18px">抹零</h3>
      <label class="perm-row">
        抹零方式
        <select v-model="form.round_mode" style="margin-left: 8px">
          <option value="none">不抹零</option>
          <option value="jiao">抹分（13.87 → 13.80）</option>
          <option value="yuan">抹角（13.87 → 13.00）</option>
        </select>
      </label>

      <h3 style="margin-top: 18px">积分规则</h3>
      <div class="settings-form" style="max-width: 420px">
        <label>每消费 1 元得积分
          <input v-model="form.points_earn_per_yuan" type="number" min="0" step="1" />
        </label>
        <label>多少积分抵 1 元
          <input v-model="form.points_per_yuan" type="number" min="1" step="1" />
        </label>
        <label>积分有效期（月，0=永不过期）
          <input v-model="form.points_expire_months" type="number" min="0" step="1" />
        </label>
        <label>会员等级规则（阈值元:名称:折扣%:积分倍率，逗号分隔）
          <input v-model="form.member_tiers" placeholder="0:普通会员:0:1,1000:银卡会员:2:2" />
        </label>
        <label>储值赠送（满X元送Y元，逗号分隔，留空=不送）
          <input v-model="form.topup_bonus_rules" placeholder="500:50,1000:150" />
        </label>
      </div>

      <h3 style="margin-top: 18px">收银员权限</h3>
      <p class="muted" style="margin: 0 0 10px">默认都关闭：退款与打折需要店长操作。</p>
      <label class="perm-row">
        <input type="checkbox" v-model="perms.refund" />
        允许收银员退款
      </label>
      <label class="perm-row">
        <input type="checkbox" v-model="perms.discount" />
        允许收银员整单打折
      </label>
      <label class="perm-row">
        <input type="checkbox" v-model="perms.price" />
        允许收银员单品改价
      </label>
      <label class="perm-row">
        <input type="checkbox" v-model="perms.void" />
        允许收银员作废订单
      </label>
      <div class="settings-actions">
        <button class="primary" :disabled="!isManager || saving" @click="save">
          {{ saving ? '保存中…' : '保存' }}
        </button>
        <span v-if="!isManager" class="muted">只有店长可以修改</span>
      </div>
    </div>

    <h2 class="print-section-title">打印</h2>

    <div class="shift-card" style="max-width: 720px">
      <h3>小票打印机</h3>
      <p class="muted" style="margin: 0 0 10px">
        小票打印机在启动时指定：<code>posd --printer "打印机名称"</code>；不指定则把纸面字节写入
        <code>data/print.bin</code>，用于无纸调试。
      </p>
      <div class="settings-form" style="max-width: 520px">
        <label>打印机名<input v-model="receiptTarget" maxlength="60" placeholder="例如：POS-58 或 tcp://192.168.1.50:9100" @input="checkReceiptInput" /></label>
        <p v-if="receiptHint" class="printer-hint">{{ receiptHint }}</p>
      </div>
      <div style="display: flex; align-items: center; gap: 12px">
        <button class="primary" :disabled="!printerValid(receiptTarget) || receiptTesting" @click="testReceiptPrinter">
          {{ receiptTesting ? '测试中…' : '打印测试' }}
        </button>
        <span v-if="receiptTestMsg" class="muted" :style="{ color: receiptTestMsg.startsWith('✓') ? 'var(--brand)' : 'var(--money)' }">{{ receiptTestMsg }}</span>
      </div>
      <p class="muted" style="margin: 6px 0 0; font-size: 12px">
        网口打印机填 <code>tcp://IP:9100</code>，需先给打印机设固定 IP；测试不发送纸面内容。
      </p>
    </div>

    <div class="shift-card" style="max-width: 720px">
      <h3>后厨打印</h3>
      <p class="muted" style="margin: 0 0 10px">
        填打印机名后，开台/下单会同时打一张后厨单；留空表示不出后厨单，类目留空表示全部类目都出。
      </p>
      <div class="settings-form">
        <label>后厨打印机名<input v-model="form.kitchen_printer" maxlength="40" placeholder="例如：厨房打印机" /></label>
        <label>出后厨单的类目（逗号分隔）<input v-model="form.kitchen_categories" maxlength="60" placeholder="例如：奶茶,小吃（留空=全部）" /></label>
      </div>
      <h3 style="margin-top: 18px">后厨档口</h3>
      <p class="muted" style="margin: 0 0 10px">一个档口一台打印机；类目留空表示该档口收全部菜品。</p>
      <table class="orders-table">
        <thead>
          <tr><th>档口</th><th>打印机</th><th>类目</th><th style="width:80px"></th></tr>
        </thead>
        <tbody>
          <tr v-for="st in stations" :key="st.ID">
            <td>{{ st.Name }}</td>
            <td class="mono">{{ st.Printer }}</td>
            <td>{{ st.Categories || '全部' }}</td>
            <td><button class="danger" @click="removeStation(st)">删除</button></td>
          </tr>
          <tr v-if="!stations.length"><td colspan="4" class="empty-row">还没有档口（会回落到上面的单台后厨打印机）</td></tr>
        </tbody>
      </table>
      <div class="settings-form" style="max-width: 520px; margin-top: 10px">
        <label>档口名<input v-model="newStation.name" maxlength="20" placeholder="例如：水吧" /></label>
        <label>打印机名<input v-model="newStation.printer" maxlength="60" placeholder="例如：POS-58 或 tcp://192.168.1.50:9100" @input="checkPrinterInput" /></label>
        <p v-if="printerHint" class="printer-hint">{{ printerHint }}</p>
        <label>类目（逗号分隔，可空）<input v-model="newStation.categories" placeholder="奶茶,果茶" /></label>
      </div>
      <p class="muted" style="margin: 6px 0 8px; font-size: 12px">
        网口打印机填 <code>tcp://IP:9100</code>，需先给打印机设固定 IP。
      </p>
      <button class="primary" :disabled="!newStation.name || !printerValid(newStation.printer)" @click="addStation">新建档口</button>
      <p v-if="stationError" class="orders-error">{{ stationError }}</p>
    </div>

    <div class="shift-card" style="max-width: 720px">
      <h3>打印队列</h3>
      <p class="muted" style="margin: 0 0 10px">最近 10 条打印任务（单号 / 目标 / 状态 / 重试次数）。</p>
      <table class="orders-table">
        <thead>
          <tr><th>单号</th><th>目标</th><th>状态</th><th>重试</th><th>最后错误</th></tr>
        </thead>
        <tbody>
          <tr v-for="j in printJobs" :key="j.id">
            <td class="mono">{{ j.orderNo }}</td>
            <td>{{ j.target }}{{ j.printer ? ' · ' + j.printer : '' }}</td>
            <td>{{ jobStatusText(j.status) }}</td>
            <td>{{ j.attempts }}</td>
            <td class="muted">{{ j.error || '—' }}</td>
          </tr>
          <tr v-if="!printJobs.length"><td colspan="5" class="empty-row">还没有打印任务</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'

const props = defineProps({ role: { type: String, default: 'cashier' } })

const form = reactive({ shop_name: '', shop_phone: '', shop_footer: '', receipt_footer: '', round_mode: 'none', kitchen_printer: '', kitchen_categories: '' })
const perms = reactive({ refund: false, discount: false, price: false, void: false })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
let timer = null

const isManager = computed(() => props.role === 'manager')
const stations = ref([])
const newStation = reactive({ name: '', printer: '', categories: '' })
const stationError = ref('')
const printJobs = ref([])

const JOB_STATUS = { queued: '排队中', printed: '已打印', failed: '待重打' }
function jobStatusText(s) { return JOB_STATUS[s] || s }

async function loadPrintJobs() {
  try {
    printJobs.value = await api.printJobs(10)
  } catch (e) { /* 队列不可读时不阻塞设置页 */ }
}
const printerHint = ref('')
const receiptTarget = ref('')
const receiptTesting = ref(false)
const receiptTestMsg = ref('')
const receiptHint = ref('')

async function testReceiptPrinter() {
  receiptTesting.value = true
  receiptTestMsg.value = ''
  try {
    const r = await api.testPrinter(receiptTarget.value)
    receiptTestMsg.value = (r.ok ? '✓ ' : '✗ ') + (r.message || '')
  } catch (e) {
    receiptTestMsg.value = '✗ ' + (e.message || '测试失败')
  } finally {
    receiptTesting.value = false
  }
}
function checkReceiptInput() {
  const v = String(receiptTarget.value || '').trim()
  receiptHint.value = (v.startsWith('tcp://') && !printerValid(v))
    ? '格式应为 tcp://IP:9100（网口打印机），IP 为打印机固定地址'
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
    ? '格式应为 tcp://IP:9100（网口打印机），IP 为打印机固定地址'
    : ''
}

async function loadStations() {
  try {
    stations.value = await api.listStations()
  } catch (e) {
    stationError.value = '读取档口失败'
  }
}

async function addStation() {
  stationError.value = ''
  const v = String(newStation.printer || '').trim()
  if (v.startsWith('tcp://') && !printerValid(v)) {
    printerHint.value = '格式应为 tcp://IP:9100（网口打印机），IP 为打印机固定地址'
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
    stationError.value = '新建失败：' + (e.message || '未知错误')
  }
}

async function removeStation(st) {
  if (!window.confirm('删除档口「' + st.name + '」？')) return
  try {
    await api.deleteStation(st.ID)
    await loadStations()
  } catch (e) {
    stationError.value = '删除失败：' + (e.message || '未知错误')
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
    perms.refund = data.perm_refund_cashier === 'true'
    perms.discount = data.perm_discount_cashier === 'true'
    perms.price = data.perm_change_price_cashier === 'true'
    perms.void = data.perm_void_cashier === 'true'
  } catch (e) {
    error.value = '读取设置失败：' + (e.message || '未知错误')
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
      topup_bonus_rules: form.topup_bonus_rules,
      kitchen_printer: form.kitchen_printer,
      kitchen_categories: form.kitchen_categories
    })
    Object.assign(form, { shop_name: data.shop_name, shop_phone: data.shop_phone, shop_footer: data.shop_footer, receipt_footer: data.receipt_footer || '', round_mode: data.round_mode || 'none' })
    perms.refund = data.perm_refund_cashier === 'true'
    perms.discount = data.perm_discount_cashier === 'true'
    perms.price = data.perm_change_price_cashier === 'true'
    perms.void = data.perm_void_cashier === 'true'
    toast('已保存，下一张小票生效')
  } catch (e) {
    toast('保存失败：' + (e.message || '未知错误'), true)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
  loadStations()
  loadPrintJobs()
})
</script>
