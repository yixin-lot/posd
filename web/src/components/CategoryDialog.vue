<template>
  <div v-if="open" class="mask" @click.self="$emit('close')">
    <div class="modal category-modal">
      <h3>类目管理</h3>
      <p class="muted" style="margin: 0">改名会同步更新所有商品，不用逐个改。</p>

      <div class="cat-add">
        <input v-model="newName" maxlength="20" placeholder="新类目名" @keyup.enter="create" />
        <button class="primary" :disabled="!newName.trim()" @click="create">新建</button>
      </div>

      <table class="orders-table" style="margin-top: 10px">
        <thead>
          <tr>
            <th>类目</th>
            <th style="width: 90px" class="num">商品数</th>
            <th style="width: 150px"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in list" :key="c.id">
            <td>
              <input v-if="editing === c.id" v-model="editName" class="inline-input" @keyup.enter="save(c)" />
              <template v-else>{{ c.name }}</template>
            </td>
            <td class="num">{{ c.count }}</td>
            <td class="ops">
              <template v-if="editing === c.id">
                <button @click="save(c)">保存</button>
                <button @click="editing = 0">取消</button>
              </template>
              <template v-else>
                <button @click="startEdit(c)">改名</button>
                <button :disabled="c.count > 0" @click="remove(c)">删除</button>
              </template>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="3" class="empty-row">还没有类目</td></tr>
        </tbody>
      </table>

      <div class="modal-actions">
        <button @click="$emit('close')">关闭</button>
      </div>
      <p v-if="error" class="orders-error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { api } from '../api/index.js'

const props = defineProps({ open: { type: Boolean, default: false } })
const emit = defineEmits(['close', 'changed'])

const list = ref([])
const newName = ref('')
const editing = ref(0)
const editName = ref('')
const error = ref('')

async function load() {
  error.value = ''
  try {
    list.value = await api.listCategories()
  } catch (e) {
    error.value = '读取类目失败：' + (e.message || '未知错误')
  }
}

async function create() {
  if (!newName.value.trim()) return
  try {
    await api.createCategory({ name: newName.value.trim() })
    newName.value = ''
    await load()
    emit('changed')
  } catch (e) {
    error.value = '新建失败：' + (e.message || '未知错误')
  }
}

function startEdit(c) {
  editing.value = c.id
  editName.value = c.name
}

async function save(c) {
  try {
    await api.updateCategory(c.id, { name: editName.value.trim(), sort: c.sort || 0 })
    editing.value = 0
    await load()
    emit('changed')
  } catch (e) {
    error.value = '改名失败：' + (e.message || '未知错误')
  }
}

async function remove(c) {
  try {
    await api.deleteCategory(c.id)
    await load()
    emit('changed')
  } catch (e) {
    error.value = '删除失败：' + (e.message || '未知错误')
  }
}

watch(() => props.open, (v) => v && load())
onMounted(() => props.open && load())
</script>

<style scoped>
.category-modal { width: min(560px, 100%); }
.cat-add { display: flex; gap: 8px; margin-top: 12px; }
.cat-add input { flex: 1; }
.inline-input { width: 100%; }
</style>
