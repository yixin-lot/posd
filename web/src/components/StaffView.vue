<template>
  <section class="orders">
    <header class="orders-head">
      <h2>员工管理</h2>
      <span class="settings-hint">设置页 · 仅店长可见</span>
      <span class="grow"></span>
      <button class="primary" @click="openCreate">新建员工</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <table class="orders-table">
      <thead>
        <tr>
          <th>姓名</th>
          <th style="width: 160px">账号</th>
          <th style="width: 100px">角色</th>
          <th style="width: 90px">状态</th>
          <th style="width: 210px"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in list" :key="s.id" :class="{ dim: !s.active }">
          <td>{{ s.name }}</td>
          <td class="mono">{{ s.username }}</td>
          <td>{{ s.role === 'manager' ? '店长' : '收银员' }}</td>
          <td>
            <span class="chip" :class="s.active ? 'chip-ok' : 'chip-idle'">{{ s.active ? '在职' : '已停用' }}</span>
          </td>
          <td class="ops">
            <template v-if="s.username === 'admin'">
              <button @click="openEdit(s)">改 PIN</button>
              <span class="chip chip-idle">内置店长</span>
            </template>
            <template v-else>
              <button @click="openEdit(s)">编辑</button>
              <button @click="toggle(s)">{{ s.active ? '停用' : '启用' }}</button>
              <button @click="remove(s)">删除</button>
            </template>
          </td>
        </tr>
        <tr v-if="!list.length">
          <td colspan="5" class="empty-row">没有员工</td>
        </tr>
      </tbody>
    </table>

    <div v-if="editing" class="mask" @click.self="editing = null">
      <div class="modal product-modal">
        <h3>{{ editing.id ? '编辑员工' : '新建员工' }}</h3>
        <div class="product-form">
          <label>姓名* <input v-model="form.name" placeholder="员工姓名" /></label>
          <label>账号* <input v-model="form.username" placeholder="登录用账号" :disabled="form.username === 'admin'" /></label>
          <label>PIN{{ editing.id ? '（留空不修改）' : '*' }}
            <input v-model="form.pin" type="password" :placeholder="editing.id ? '留空保持原 PIN' : '至少 4 位'" />
          </label>
          <label>角色
            <select v-model="form.role" :disabled="form.username === 'admin'">
              <option value="cashier">收银员</option>
              <option value="manager">店长</option>
            </select>
          </label>
        </div>
        <div class="modal-actions">
          <button @click="editing = null">取消</button>
          <button class="primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
        <p v-if="formError" class="orders-error">{{ formError }}</p>
      </div>
    </div>

    <div v-if="toastMsg" class="orders-toast" :class="{ bad: toastBad }">{{ toastMsg }}</div>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/index.js'

const list = ref([])
const error = ref('')
const editing = ref(null)
const saving = ref(false)
const formError = ref('')
const toastMsg = ref('')
const toastBad = ref(false)
let toastTimer = null

const form = reactive({ name: '', username: '', pin: '', role: 'cashier' })

function toast(msg, bad) {
  toastMsg.value = msg
  toastBad.value = !!bad
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toastMsg.value = ''), 2400)
}

async function load() {
  error.value = ''
  try {
    list.value = await api.listStaff()
  } catch (e) {
    error.value = '加载员工失败：' + (e.message || '未知错误')
  }
}

function openCreate() {
  Object.assign(form, { name: '', username: '', pin: '', role: 'cashier' })
  formError.value = ''
  editing.value = { id: 0 }
}

function openEdit(s) {
  Object.assign(form, { name: s.name, username: s.username, pin: '', role: s.role })
  formError.value = ''
  editing.value = s
}

async function save() {
  saving.value = true
  formError.value = ''
  const body = {
    name: form.name,
    username: form.username,
    pin: form.pin,
    role: form.role
  }
  try {
    if (editing.value.id) {
      await api.updateStaff(editing.value.id, body)
    } else {
      await api.createStaff(body)
    }
    editing.value = null
    toast('已保存')
    await load()
  } catch (e) {
    formError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function toggle(s) {
  try {
    await api.setStaffActive(s.id, !s.active)
    await load()
  } catch (e) {
    toast('操作失败：' + (e.message || ''), true)
  }
}

async function remove(s) {
  try {
    await api.deleteStaff(s.id)
    toast('已删除')
    await load()
  } catch (e) {
    toast('删除失败：' + (e.message || ''), true)
  }
}

onMounted(load)
</script>
