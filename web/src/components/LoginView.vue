<template>
  <div class="login-wrap">
    <!-- v03 P1：左上角品牌块（不是卡内大 logo），中央 420px 深色卡 -->
    <div class="login-topbrand">
      <svg class="lt-logo" viewBox="0 0 48 48" aria-hidden="true">
        <rect width="48" height="48" rx="12" fill="#0d9488" />
        <path d="M14 20h16l-1.5 12a4 4 0 0 1-4 3.6h-5a4 4 0 0 1-4-3.6L14 20z" fill="#fff" />
        <path d="M30 21h2.5a4 4 0 0 1 0 8H30" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" />
      </svg>
      <div>
        <div class="lt-name">{{ t('brand.name') }}</div>
        <div class="lt-sub">{{ t('brand.tagline') }}</div>
      </div>
      <div class="lang-switch">
        <button v-for="l in locales" :key="l.id" :class="{ active: locale === l.id }" @click="setLocale(l.id)">{{ l.label }}</button>
      </div>
    </div>

    <div class="login-card">
      <div class="login-card-head">
        <svg class="login-logo" viewBox="0 0 48 48" aria-hidden="true">
          <rect width="48" height="48" rx="12" fill="#0d9488" />
          <path d="M14 20h16l-1.5 12a4 4 0 0 1-4 3.6h-5a4 4 0 0 1-4-3.6L14 20z" fill="#fff" />
          <path d="M30 21h2.5a4 4 0 0 1 0 8H30" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" />
          <path d="M17.5 24.5l3-3 3 3 3-3 3 3" fill="none" stroke="#04211f" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <div>
          <div class="login-title">{{ t('login.title') }}</div>
          <div class="login-sub">{{ t('login.sub') }}</div>
        </div>
      </div>

      <label class="login-field">
        <span>{{ t('login.account') }}</span>
        <input ref="userInput" v-model.trim="username" placeholder="admin" @keyup.enter="submit" />
      </label>
      <label class="login-field">
        <span>{{ t('login.password') }}</span>
        <input v-model.trim="pin" type="password" :placeholder="t('login.pinPlaceholder')" @keyup.enter="submit" />
      </label>

      <p class="login-hint">{{ t('login.hint', { user: 'admin', pin: '1234' }) }}</p>

      <div class="login-row">
        <label class="login-remember">
          <input type="checkbox" v-model="remember" />
          {{ t('login.remember') }}
        </label>
        <span class="login-lock-hint">{{ t('login.lock') }}</span>
      </div>

      <p v-if="error" class="login-error">{{ error }}</p>

      <button class="login-btn" :disabled="busy" @click="submit">{{ busy ? t('login.busy') : t('login.submit') }}</button>

      <p class="login-version">
        <span v-if="version">Posd v{{ version }}</span>
        <span v-else>{{ t('brand.name') }}</span>
        · {{ t('login.footer') }} · Apache-2.0
      </p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { setToken } from '../lib/http.js'
import { t, locale, locales, setLocale } from '../i18n.js'

const emit = defineEmits(['success'])
const LAST_USER_KEY = 'pos.lastUser'
const username = ref('admin')
const pin = ref('')
const busy = ref(false)
const error = ref('')
const userInput = ref(null)
// v03 P1：记住账号（PIN 不落盘——本机明文存 PIN 等于把门店密码公开）
const remember = ref(false)

// v03 P1「记住密码」：本机单店场景，凭据留在本机 localStorage，换机不同步；
// 老版本只存账号（LAST_USER_KEY），读不到凭据时仍回填账号。
const CRED_KEY = 'pos.lastCred'

onMounted(() => {
  // /version 不需要登录：拿真实构建版本，避免页面写死后过期
  fetch('/version')
    .then((r) => (r.ok ? r.json() : null))
    .then((v) => { if (v && v.version) version.value = String(v.version).replace(/^v/, '') })
    .catch(() => {})
  try {
    const raw = localStorage.getItem(CRED_KEY)
    if (raw) {
      const c = JSON.parse(raw)
      if (c && c.u) {
        username.value = c.u
        pin.value = c.p || ''
        remember.value = true
      }
    } else {
      const saved = localStorage.getItem(LAST_USER_KEY)
      if (saved) {
        username.value = saved
        remember.value = true
      }
    }
  } catch (e) {
    // 隐私模式禁用 storage 时忽略
  }
  if (userInput.value) userInput.value.focus()
})

async function submit() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await api.login(username.value, pin.value)
    setToken(result.token)
    try {
      if (remember.value) {
        localStorage.setItem(CRED_KEY, JSON.stringify({ u: username.value, p: pin.value }))
        localStorage.setItem(LAST_USER_KEY, username.value)
      } else {
        localStorage.removeItem(CRED_KEY)
        localStorage.removeItem(LAST_USER_KEY)
      }
    } catch (e) {
      // storage 不可用不影响登录
    }
    emit('success', result.staff)
  } catch (e) {
    if (e.status === 423) {
      // 连错 5 次锁定：后端带剩余秒数，别用「PIN 不正确」盖掉
      error.value = e.message || 'Too many attempts, try later'
    } else if (e.status === 401) {
      error.value = t('login.bad')
    } else {
      error.value = t('login.bad') + ' (' + (e.message || '') + ')'
    }
  } finally {
    busy.value = false
  }
}
</script>
