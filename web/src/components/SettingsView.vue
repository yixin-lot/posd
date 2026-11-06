<template>
  <section class="orders">
    <header class="orders-head">
      <h2>设置</h2>
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
          {{ t.label }}
        </button>
      </aside>

      <div class="settings-body">
        <div v-show="tab === 'shop'" class="shift-card">
          <h3>小票抬头</h3>
          <div class="settings-form">
            <label>店名<input v-model="form.shop_name" maxlength="20" placeholder="例如：楼下奶茶" /></label>
            <label>小票页脚<input v-model="form.receipt_footer" maxlength="60" placeholder="如：谢谢惠顾 / 电话 138xxxx（可留空）" /></label>
            <label>电话<input v-model="form.shop_phone" maxlength="20" placeholder="选填" /></label>
            <label>脚注<input v-model="form.shop_footer" maxlength="30" placeholder="例如：谢谢光临，欢迎再来" /></label>
          </div>
        </div>

        <div v-show="tab === 'cashier'" class="shift-card">
          <h3>抹零</h3>
          <label class="perm-row">
            抹零方式
            <select v-model="form.round_mode" style="margin-left: 8px">
              <option value="none">不抹零</option>
              <option value="jiao">抹分（13.87 → 13.80）</option>
              <option value="yuan">抹角（13.87 → 13.00）</option>
            </select>
          </label>

          <h3 style="margin-top: 18px">积分规则</h3>
          <div class="settings-form">
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
        </div>

        <div v-show="tab === 'account'" class="shift-card">
          <h3>收银员权限</h3>
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
        </div>

        <div v-show="tab === 'print'">
          <div class="shift-card print-card">
            <h3>打印设置</h3>
            <p class="settings-sub">开源内核支持本地直连；云打印 API 为可选开放集成，Key/Secret 留空即不启用</p>

            <div class="set-label">打印模式</div>
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
                <b>{{ m.label }}</b>
                <span class="pm-desc">{{ m.desc }}</span>
                <span class="pm-desc">{{ m.desc2 }}</span>
              </button>
            </div>
            <p class="muted mini">
              「不打印」只记账不出票：订单照常落库，需要时可在订单页点重打。
            </p>

            <div class="set-label">打印机</div>
            <div class="printer-row">
              <label class="printer-port">端口
                <select v-model="portMode">
                  <option value="auto">自动检测</option>
                  <option value="custom">手动填写</option>
                </select>
              </label>
              <input
                v-if="portMode === 'custom'"
                class="printer-input"
                v-model="receiptTarget"
                maxlength="60"
                placeholder="例如：POS-58 或 tcp://192.168.1.50:9100"
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
                {{ receiptTesting ? '测试中…' : '测试打印' }}
              </button>
              <span class="muted mini">58mm 小票模板（内置默认）</span>
            </div>
            <p v-if="receiptHint" class="printer-hint">{{ receiptHint }}</p>
            <p v-if="receiptTestMsg" class="muted mini">{{ receiptTestMsg }}</p>

            <div class="set-label">云打印 API（未启用）</div>
            <div class="cloud-grid">
              <label>服务地址<input disabled placeholder="https://print.example.com" /></label>
              <label>设备 SN<input disabled placeholder="—" /></label>
              <label>API Key<input disabled placeholder="留空即不启用" /></label>
              <label>API Secret<input disabled placeholder="留空即不启用" /></label>
            </div>
            <p class="muted mini">切换云打印后，小票由云端模板渲染，与本地直连互斥</p>
          </div>

          <div class="shift-card">
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
            <table class="orders-table tbl" style="table-layout: fixed">
              <colgroup>
                <col style="width: 24%" /><col style="width: 34%" /><col style="width: 30%" /><col style="width: 12%" />
              </colgroup>
              <thead>
                <tr><th>档口</th><th>打印机</th><th>类目</th><th></th></tr>
              </thead>
              <tbody>
                <tr v-for="st in stations" :key="st.ID">
                  <td>{{ st.Name }}</td>
                  <td class="mono">{{ st.Printer }}</td>
                  <td>{{ st.Categories || '全部' }}</td>
                  <td><button class="station-del" @click="removeStation(st)">删除</button></td>
                </tr>
                <tr v-if="!stations.length"><td colspan="4" class="empty-row">还没有档口（会回落到上面的单台后厨打印机）</td></tr>
              </tbody>
            </table>
            <div class="station-form">
              <label>档口名<input v-model="newStation.name" maxlength="20" placeholder="例如：水吧" /></label>
              <label>打印机名<input v-model="newStation.printer" maxlength="60" placeholder="例如：POS-58 或 tcp://192.168.1.50:9100" @input="checkPrinterInput" /></label>
              <label>类目（逗号分隔，可空）<input v-model="newStation.categories" placeholder="奶茶,果茶" /></label>
            </div>
            <p v-if="printerHint" class="printer-hint">{{ printerHint }}</p>
            <p class="muted" style="margin: 6px 0 8px; font-size: 12px">
              网口打印机填 <code>tcp://IP:9100</code>，需先给打印机设固定 IP。
            </p>
            <div class="station-actions">
              <button class="primary" :disabled="!newStation.name || !printerValid(newStation.printer)" @click="addStation">新建档口</button>
            </div>
            <p v-if="stationError" class="orders-error">{{ stationError }}</p>
          </div>

          <div class="shift-card">
            <h3>打印队列</h3>
            <p class="muted" style="margin: 0 0 10px">最近 10 条打印任务（单号 / 目标 / 状态 / 重试次数 / 时间）。</p>
            <p v-if="jobsError" class="orders-error">{{ jobsError }}</p>
            <table class="orders-table tbl" style="table-layout: fixed">
              <colgroup>
                <col style="width: 26%" /><col style="width: 20%" /><col style="width: 18%" /><col style="width: 14%" /><col style="width: 22%" />
              </colgroup>
              <thead>
                <tr><th>单号</th><th>目标</th><th>状态</th><th>重试次数</th><th>时间</th></tr>
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
                  <td colspan="5" class="empty-row">还没有打印任务</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-show="tab === 'about'" class="shift-card">
          <h3>关于</h3>
          <div class="shift-info">
            <div><span>版本</span><b>v0.1.0-go</b></div>
            <div><span>运行模式</span><b>单机 standalone</b></div>
            <div><span>数据存放</span><b>本机 data/pos.db</b></div>
            <div><span>打印通道</span><b>本地直连 / 文件调试</b></div>
          </div>
          <p class="muted" style="margin: 10px 0 0">开源收银 POS：数据留在本店，不上传云端。</p>
        </div>

        <div class="settings-actions">
          <button class="primary" :disabled="!isManager || saving" @click="save">
            {{ saving ? '保存中…' : '保存' }}
          </button>
          <span v-if="!isManager" class="muted">只有店长可以修改</span>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'

const props = defineProps({ role: { type: String, default: 'cashier' } })

const form = reactive({ shop_name: '', shop_phone: '', shop_footer: '', receipt_footer: '', round_mode: 'none', kitchen_printer: '', kitchen_categories: '', print_mode: 'local' })

// v03 P7 左侧导航卡（分组切换）与打印模式三选项
const tab = ref('print')
const tabs = [
  { key: 'print', label: '打印设置' },
  { key: 'shop', label: '门店信息' },
  { key: 'cashier', label: '收银参数' },
  { key: 'account', label: '账号与权限' },
  { key: 'about', label: '关于' }
]
const printModes = [
  { key: 'local', label: '本地直连', desc: 'ESC/POS 指令', desc2: '默认 · 无需联网' },
  { key: 'cloud', label: '云打印 API', desc: '可选 · 云端模板', desc2: '与设备管理' },
  { key: 'none', label: '不打印', desc: '仅收银', desc2: '适合测试环境' }
]
// v03 P7：打印机「端口」下拉，自动检测为默认，自定义时才手填地址
const portMode = ref('auto')
const perms = reactive({ refund: false, discount: false, price: false, void: false })
const loading = ref(false)
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
  if (!v) return { ok: false, text: '未指定 · 未连接' }
  if (receiptTestMsg.value.startsWith('✓')) return { ok: true, text: '就绪 · ' + v }
  return { ok: false, text: '待验证' }
})

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

// 打印队列（GET /pos/v1/print/jobs）：最近 10 条，只读
async function loadPrintJobs() {
  jobsError.value = ''
  try {
    const data = await api.listPrintJobs()
    printJobs.value = Array.isArray(data && data.jobs) ? data.jobs : []
  } catch (e) {
    jobsError.value = '读取打印队列失败'
    printJobs.value = []
  }
}
function jobTarget(j) {
  // 后端 target 枚举：front（前台小票）/ kitchen（后厨单）/ report（交接单）/ void（作废单）。
  // 指定了打印机时优先显示打印机名（一笔单可以同时进两台机器）。
  if (j.printer) return j.printer
  return { front: '小票', receipt: '小票', kitchen: '后厨单', void: '作废单', report: '交接单' }[j.target] || j.target || '—'
}
function jobText(s) {
  return { queued: '排队中', printing: '打印中', done: '已完成', dead: '失败' }[s] || s || '—'
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
    form.print_mode = data.print_mode || 'local'
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
  loadPrintJobs()
  loadStations()
})
</script>

