<template>
  <div v-if="open" class="ob-mask">
    <div class="ob-modal">
      <div class="ob-head">
        <div class="ob-steps">
          <span v-for="(s, i) in steps" :key="i" class="ob-step" :class="{ cur: step === i + 1, done: step > i + 1 }">
            {{ i + 1 }} {{ s }}
          </span>
        </div>
        <button class="ob-close" title="跳过并完成" @click="finish">×</button>
      </div>

      <!-- Step 1 选业态 -->
      <div v-if="step === 1" class="ob-body">
        <h3 class="ob-title">选一个和你店里最像的业态</h3>
        <p class="ob-sub">会导入一套示例商品作为起点，之后随时可改；也可以先跳过保持默认。</p>
        <TemplatePicker
          :role="role"
          :open="true"
          embedded
          @applied="onApplied"
          @close="next"
        />
      </div>

      <!-- Step 2 店铺信息 -->
      <div v-else-if="step === 2" class="ob-body">
        <h3 class="ob-title">店铺信息（可跳过）</h3>
        <p class="ob-sub">显示在收银台状态栏和小票抬头，之后可在「设置」里修改。</p>
        <div class="ob-form">
          <label class="ob-field">
            <span>店名</span>
            <input v-model="shopName" maxlength="40" placeholder="如：沪上·真茶铺" />
          </label>
          <label class="ob-field">
            <span>电话（可选）</span>
            <input v-model="shopPhone" maxlength="20" placeholder="显示在小票上" />
          </label>
          <label class="ob-field">
            <span>小票脚注（可选）</span>
            <input v-model="shopFooter" maxlength="60" placeholder="如：欢迎下次光临" />
          </label>
          <label class="ob-field">
            <span>抹零方式</span>
            <select v-model="roundMode" @change="roundModeTouched = true">
              <option value="none">不抹零（按实收）</option>
              <option value="jiao">抹分（13.87 → 13.80）</option>
              <option value="yuan">抹角（13.87 → 13.00）</option>
            </select>
          </label>
        </div>
      </div>

      <!-- Step 3 员工 / PIN 安全 -->
      <div v-else-if="step === 3" class="ob-body">
        <h3 class="ob-title">员工与安全（可跳过）</h3>
        <p class="ob-sub">当前店长账号：<b>admin</b>，默认 PIN <b>1234</b>。</p>
        <p class="ob-sub">正式使用前建议在「员工」页修改 PIN 并新建店员账号，避免默认口令泄露。</p>
        <p class="ob-note">收银员只能收银；退款 / 折扣 / 改价 / 作废默认仅店长，可在「设置」里放开。</p>
      </div>

      <!-- Step 4 完成页 -->
      <div v-else class="ob-body">
        <h3 class="ob-title">开业前，记住这三件事</h3>
        <div class="ob-tips">
          <div class="ob-tip">
            <b>挂单怎么用</b>
            <span>收款时点「挂单」，单子会留在收银台顶部，点一下就能继续收。</span>
          </div>
          <div class="ob-tip">
            <b>扫码支付是记账式</b>
            <span>微信 / 支付宝付款只记方式、不核对到账，按应收金额收款即可。</span>
          </div>
          <div class="ob-tip">
            <b>类目在「商品」页管理</b>
            <span>商品按类目分组，新增、改名、删除都在「商品」页的「类目」里。</span>
          </div>
        </div>
      </div>

      <div class="ob-foot">
        <button v-if="step > 1" class="ob-btn" @click="step--">上一步</button>
        <span class="ob-spacer"></span>
        <button v-if="step < 4" class="ob-btn ghost" @click="next">跳过</button>
        <button class="ob-btn primary" @click="step < 4 ? next() : finish()">
          {{ step < 4 ? '下一步' : '完成' }}
        </button>
      </div>
      <p v-if="error" class="ob-error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { request } from '../lib/http.js'
import TemplatePicker from './TemplatePicker.vue'

// 状态由后端 setting setup_done 驱动（默认 false）；完成后 PUT setup_done=true，
// 并广播 pos:setup-done 让 DemoBanner 收起演示角标。setup_done=true 后不再出现（含重启）。
const props = defineProps({ role: { type: String, default: 'cashier' } })

const steps = ['选业态', '店铺信息', '员工安全', '完成']
const open = ref(false)
const step = ref(1)
const shopName = ref('')
const shopPhone = ref('')
const shopFooter = ref('')
const roundMode = ref('none')
const roundModeTouched = ref(false)
const error = ref('')

async function loadSettings() {
  try {
    const s = await request('/pos/v1/settings')
    shopName.value = s.shop_name || ''
    shopPhone.value = s.shop_phone || ''
    shopFooter.value = s.shop_footer || ''
    roundMode.value = s.round_mode || 'none'
  } catch (e) {
    // 拉不到用空表单，保存时再报错
  }
}

function onApplied() {
  next()
}

function next() {
  if (step.value === 2) saveShop()
  if (step.value < 4) step.value += 1
}

async function saveShop() {
  const body = {}
  if (shopName.value !== '') body.shop_name = shopName.value
  if (shopPhone.value !== '') body.shop_phone = shopPhone.value
  if (shopFooter.value !== '') body.shop_footer = shopFooter.value
  if (roundModeTouched.value) body.round_mode = roundMode.value
  if (!Object.keys(body).length) return
  try {
    await request('/pos/v1/settings', { method: 'PUT', body: JSON.stringify(body) })
  } catch (e) {
    error.value = '保存店铺信息失败：' + (e.message || '未知错误')
  }
}

async function finish() {
  if (step.value === 2) saveShop()
  error.value = ''
  try {
    await request('/pos/v1/settings', {
      method: 'PUT',
      body: JSON.stringify({ setup_done: 'true' })
    })
    open.value = false
    window.dispatchEvent(new CustomEvent('pos:setup-done'))
  } catch (e) {
    error.value = '保存失败：' + (e.message || '未知错误')
  }
}

function onSetup() {
  open.value = true
  step.value = 1
  loadSettings()
}

onMounted(async () => {
  window.addEventListener('pos:setup', onSetup)
  // 首次使用（setup_done 未完成）自动弹一次，避免店主只看到演示角标却不知道从哪开始。
  try {
    const st = await request('/pos/v1/settings')
    const done = st && (st.setup_done === 'true' || st.setup_done === true)
    if (!done && props.role === 'manager') {
      open.value = true
      step.value = 1
      loadSettings()
    }
  } catch (e) {
    // 读不到设置就不打扰
  }
})

onUnmounted(() => {
  window.removeEventListener('pos:setup', onSetup)
})
</script>

<style scoped>
.ob-mask {
  position: fixed;
  inset: 0;
  z-index: 55;
  background: rgba(2, 6, 17, 0.78);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 8vh 24px 24px;
}

.ob-modal {
  width: min(760px, 100%);
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 18px 22px 16px;
  max-height: 86vh;
  overflow: auto;
}

.ob-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.ob-steps {
  display: flex;
  gap: 8px;
}

.ob-step {
  font-size: 12px;
  color: var(--muted);
  border: 1px solid var(--line);
  border-radius: 20px;
  padding: 4px 12px;
}

.ob-step.cur {
  color: var(--brand);
  border-color: var(--brand);
}

.ob-step.done {
  color: var(--ok);
  border-color: var(--ok);
}

.ob-close {
  background: transparent;
  border-color: transparent;
  font-size: 20px;
  line-height: 1;
  padding: 2px 8px;
  color: var(--muted);
}

.ob-close:hover {
  color: var(--text);
}

.ob-body {
  padding-bottom: 6px;
}

.ob-title {
  margin: 0 0 6px;
  font-size: 17px;
  color: var(--text);
}

.ob-sub {
  margin: 0 0 14px;
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}

.ob-note {
  margin: 0;
  font-size: 12px;
  color: var(--money);
  border: 1px solid var(--money-soft);
  border-radius: 8px;
  padding: 8px 10px;
}

.ob-tips {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ob-tip {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 13px;
  color: var(--text);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 10px 12px;
  background: var(--panel-2);
}

.ob-tip b {
  color: var(--brand);
}

.ob-tip span {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.6;
}

.ob-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ob-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  color: var(--text);
}

.ob-field input {
  font-size: 14px;
}

.ob-foot {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--line);
}

.ob-spacer {
  flex: 1;
}

.ob-btn {
  padding: 8px 16px;
  font-size: 13px;
  border-radius: 8px;
}

.ob-btn.ghost {
  background: transparent;
  color: var(--muted);
}

.ob-btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.ob-error {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--danger);
}
</style>
