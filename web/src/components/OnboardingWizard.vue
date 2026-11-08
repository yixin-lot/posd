<template>
  <div v-if="open" class="ob-mask">
    <div class="ob-modal">
      <div class="ob-head">
        <div class="ob-steps">
          <span v-for="(s, i) in steps" :key="i" class="ob-step" :class="{ cur: step === i + 1, done: step > i + 1 }">
            {{ i + 1 }} {{ s }}
          </span>
        </div>
        <button class="ob-close" :title="tr('ob.skipAll')" @click="finish">×</button>
      </div>

      <!-- Step 1 选业态 -->
      <div v-if="step === 1" class="ob-body">
        <h3 class="ob-title">{{ tr('ob.step1Title') }}</h3>
        <p class="ob-sub">{{ tr('ob.step1Sub') }}</p>
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
        <h3 class="ob-title">{{ tr('ob.step2Title') }}</h3>
        <p class="ob-sub">{{ tr('ob.step2Sub') }}</p>
        <div class="ob-form">
          <label class="ob-field">
            <span>{{ tr('ob.shopName') }}</span>
            <input v-model="shopName" maxlength="40" :placeholder="tr('ob.shopNamePh')" />
          </label>
          <label class="ob-field">
            <span>{{ tr('ob.phone') }}</span>
            <input v-model="shopPhone" maxlength="20" :placeholder="tr('ob.phonePh')" />
          </label>
          <label class="ob-field">
            <span>{{ tr('ob.footer') }}</span>
            <input v-model="shopFooter" maxlength="60" :placeholder="tr('ob.footerPh')" />
          </label>
          <label class="ob-field">
            <span>{{ tr('ob.rounding') }}</span>
            <select v-model="roundMode" @change="roundModeTouched = true">
              <option value="none">{{ tr('ob.roundNone') }}</option>
              <option value="jiao">{{ tr('ob.roundJiao') }}</option>
              <option value="yuan">{{ tr('ob.roundYuan') }}</option>
            </select>
          </label>
        </div>
      </div>

      <!-- Step 3 员工 / PIN 安全 -->
      <div v-else-if="step === 3" class="ob-body">
        <h3 class="ob-title">{{ tr('ob.step3Title') }}</h3>
        <p class="ob-sub">{{ tr('ob.step3Sub1') }}</p>
        <p class="ob-sub">{{ tr('ob.step3Sub2') }}</p>
        <p class="ob-note">{{ tr('ob.step3Note') }}</p>
      </div>

      <!-- Step 4 完成页 -->
      <div v-else class="ob-body">
        <h3 class="ob-title">{{ tr('ob.step4Title') }}</h3>
        <div class="ob-tips">
          <div class="ob-tip">
            <b>{{ tr('ob.tip1') }}</b>
            <span>{{ tr('ob.tip1Text') }}</span>
          </div>
          <div class="ob-tip">
            <b>{{ tr('ob.tip2') }}</b>
            <span>{{ tr('ob.tip2Text') }}</span>
          </div>
          <div class="ob-tip">
            <b>{{ tr('ob.tip3') }}</b>
            <span>{{ tr('ob.tip3Text') }}</span>
          </div>
        </div>
      </div>

      <div class="ob-foot">
        <button v-if="step > 1" class="ob-btn" @click="step--">{{ tr('ob.back') }}</button>
        <span class="ob-spacer"></span>
        <button v-if="step < 4" class="ob-btn ghost" @click="next">{{ tr('ob.skip') }}</button>
        <button class="ob-btn primary" @click="step < 4 ? next() : finish()">
          {{ step < 4 ? tr('ob.next') : tr('ob.finish') }}
        </button>
      </div>
      <p v-if="error" class="ob-error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { t as tr } from '../i18n.js'
import { request } from '../lib/http.js'
import TemplatePicker from './TemplatePicker.vue'

// 状态由后端 setting setup_done 驱动（默认 false）；完成后 PUT setup_done=true，
// 并广播 pos:setup-done 让 DemoBanner 收起演示角标。setup_done=true 后不再出现（含重启）。
const props = defineProps({ role: { type: String, default: 'cashier' } })

const steps = computed(() => [tr('ob.step1'), tr('ob.step2'), tr('ob.step3'), tr('ob.step4')])
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
    error.value = tr('ob.saveShopFailed', { msg: e.message || '' })
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
    error.value = tr('ob.saveFailed', { msg: e.message || '' })
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
