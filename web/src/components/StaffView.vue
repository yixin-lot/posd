<template>
  <section class="orders">
    <header class="orders-head">
      <h2>{{ tr('stfx.title') }}</h2>
      <span class="settings-hint">{{ tr('stfx.hint') }}</span>
      <span class="grow"></span>
      <button class="primary" @click="openCreate">{{ tr('stfx.new') }}</button>
    </header>

    <p v-if="error" class="orders-error">{{ error }}</p>

    <table class="orders-table">
      <thead>
        <tr>
          <th>{{ tr('stfx.name') }}</th>
          <th style="width: 160px">{{ tr('stfx.account') }}</th>
          <th style="width: 100px">{{ tr('stfx.role') }}</th>
          <th style="width: 90px">{{ tr('stfx.status') }}</th>
          <th style="width: 210px"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in list" :key="s.id" :class="{ dim: !s.active }">
          <td>{{ s.name }}</td>
          <td class="mono">{{ s.username }}</td>
          <td>{{ s.role === 'manager' ? tr('stfx.manager') : tr('stfx.cashier') }}</td>
          <td>
            <span class="chip" :class="s.active ? 'chip-ok' : 'chip-idle'">{{ s.active ? tr('stfx.active') : tr('stfx.inactive') }}</span>
          </td>
          <td class="ops">
            <template v-if="s.username === 'admin'">
              <button @click="openEdit(s)">{{ tr('stfx.changePin') }}</button>
              <span class="chip chip-idle">{{ tr('stfx.builtinManager') }}</span>
            </template>
            <template v-else>
              <button @click="openEdit(s)">{{ tr('stfx.edit') }}</button>
              <button @click="toggle(s)">{{ s.active ? tr('stfx.disable') : tr('stfx.enable') }}</button>
              <button @click="remove(s)">{{ tr('stfx.delete') }}</button>
            </template>
          </td>
        </tr>
        <tr v-if="!list.length">
          <td colspan="5" class="empty-row">{{ tr('stfx.empty') }}</td>
        </tr>
      </tbody>
    </table>

    <div v-if="editing" class="mask" @click.self="editing = null">
      <div class="modal product-modal">
        <h3>{{ editing.id ? tr('stfx.editTitle') : tr('stfx.newTitle') }}</h3>
        <div class="product-form">
          <label>{{ tr('stfx.name') }}* <input v-model="form.name" :placeholder="tr('stfx.namePh')" /></label>
          <label>{{ tr('stfx.account') }}* <input v-model="form.username" :placeholder="tr('stfx.accountPh')" :disabled="form.username === 'admin'" /></label>
          <label>PIN{{ editing.id ? tr('stfx.pinOptional') : '*' }}
            <input v-model="form.pin" type="password" :placeholder="editing.id ? tr('stfx.pinKeep') : tr('stfx.pinMin')" />
          </label>
          <label>{{ tr('stfx.role') }}
            <select v-model="form.role" :disabled="form.username === 'admin'">
              <option value="cashier">{{ tr('stfx.cashier') }}</option>
              <option value="manager">{{ tr('stfx.manager') }}</option>
            </select>
          </label>
        </div>
        <div class="modal-actions">
          <button @click="editing = null">{{ tr('stfx.cancel') }}</button>
          <button class="primary" :disabled="saving" @click="save">{{ saving ? tr('stfx.saving') : tr('stfx.save') }}</button>
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
import { t as tr } from '../i18n.js'

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
    error.value = tr('stfx.loadFailed', { msg: e.message || '' })
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
    toast(tr('stfx.saved'))
    await load()
  } catch (e) {
    formError.value = e.message || tr('stfx.saveFailedShort')
  } finally {
    saving.value = false
  }
}

async function toggle(s) {
  try {
    await api.setStaffActive(s.id, !s.active)
    await load()
  } catch (e) {
    toast(tr('stfx.opFailed', { msg: e.message || '' }), true)
  }
}

async function remove(s) {
  try {
    await api.deleteStaff(s.id)
    toast(tr('stfx.deleted'))
    await load()
  } catch (e) {
    toast(tr('stfx.deleteFailed', { msg: e.message || '' }), true)
  }
}

onMounted(load)
</script>
