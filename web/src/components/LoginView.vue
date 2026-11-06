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
        <div class="lt-name">Posd · 单机收银</div>
        <div class="lt-sub">开源 · 本地直连打印 · 离线可用</div>
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
          <div class="login-title">收银台登录</div>
          <div class="login-sub">账号为本机账号，离线也可登录</div>
        </div>
      </div>

      <label class="login-field">
        <span>账号（手机号或用户名）</span>
        <input ref="userInput" v-model.trim="username" placeholder="admin" @keyup.enter="submit" />
      </label>
      <label class="login-field">
        <span>密码</span>
        <input v-model.trim="pin" type="password" placeholder="4 位以上" @keyup.enter="submit" />
      </label>

      <p class="login-hint">
        首次使用：账号 <b>admin</b> · 密码 <b>1234</b>（登录后在「员工」里修改）
      </p>

      <div class="login-row">
        <label class="login-remember">
          <input type="checkbox" v-model="remember" />
          记住密码
        </label>
        <span class="login-lock-hint">连错 5 次锁定 5 分钟</span>
      </div>

      <p v-if="error" class="login-error">{{ error }}</p>

      <button class="login-btn" :disabled="busy" @click="submit">{{ busy ? '登录中…' : '登 录' }}</button>

      <p class="login-version">v0.1 · Posd · 数据留在本店</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { setToken } from '../lib/http.js'

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
      error.value = e.message || '登录失败次数过多，请稍后重试'
    } else if (e.status === 401) {
      error.value = '账号或 PIN 不正确'
    } else {
      error.value = '登录失败：' + (e.message || '未知错误')
    }
  } finally {
    busy.value = false
  }
}
</script>
