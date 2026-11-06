<template>
  <div class="login-wrap">
    <div class="login-card">
      <svg class="login-logo" viewBox="0 0 48 48" aria-hidden="true">
        <rect width="48" height="48" rx="12" fill="#0d9488" />
        <path d="M14 20h16l-1.5 12a4 4 0 0 1-4 3.6h-5a4 4 0 0 1-4-3.6L14 20z" fill="#fff" />
        <path d="M30 21h2.5a4 4 0 0 1 0 8H30" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" />
        <path d="M17.5 24.5l3-3 3 3 3-3 3 3" fill="none" stroke="#04211f" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
      <div class="login-brand">开源收银 POS</div>
      <div class="login-sub">Posd · Apache-2.0 · 本地直连打印 · 离线可用</div>

      <label class="login-field">
        <span>账号</span>
        <input ref="userInput" v-model.trim="username" placeholder="admin" @keyup.enter="submit" />
      </label>
      <label class="login-field">
        <span>PIN</span>
        <input v-model.trim="pin" type="password" placeholder="4 位以上" @keyup.enter="submit" />
      </label>

      <p v-if="error" class="login-error">{{ error }}</p>

      <button class="login-btn" :disabled="busy" @click="submit">{{ busy ? '登录中…' : '登录' }}</button>
      <p class="login-hint">首次使用：店长 admin / 1234（上线前请修改 PIN）</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/index.js'
import { setToken } from '../lib/http.js'

const emit = defineEmits(['success'])
const username = ref('admin')
const pin = ref('')
const busy = ref(false)
const error = ref('')
const userInput = ref(null)

// v03：账号框默认聚焦，扫码枪/键盘直接开打
onMounted(() => {
  if (userInput.value) userInput.value.focus()
})

async function submit() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await api.login(username.value, pin.value)
    setToken(result.token)
    emit('success', result.staff)
  } catch (e) {
    error.value = e.status === 401 ? '账号或 PIN 不正确' : '登录失败：' + (e.message || '未知错误')
  } finally {
    busy.value = false
  }
}
</script>
