<template>
  <!-- 独立模态模式 -->
  <div v-if="open && !embedded" class="tpl-mask">
    <div class="tpl-modal">
      <div class="tpl-head">
        <div>
          <h2 class="tpl-title">选择你的业态</h2>
          <p class="tpl-sub">模板会导入一套示例商品，之后可随时在「商品」页修改。</p>
        </div>
        <button class="tpl-close" title="关闭" @click="emit('update:open', false); emit('close')">×</button>
      </div>

      <div v-if="error" class="tpl-error">{{ error }}</div>
      <div v-else-if="!templates.length" class="tpl-loading">加载模板中…</div>

      <div v-else class="tpl-grid">
        <div
          v-for="t in templates"
          :key="t.id"
          class="tpl-card"
          :class="{ active: t.id === industry }"
        >
          <div class="tpl-card-head">
            <span class="tpl-name">{{ t.name }}</span>
            <span v-if="t.id === industry" class="tpl-tag">当前业态</span>
          </div>
          <p class="tpl-summary">{{ t.summary }}</p>
          <div class="tpl-cats">
            <span v-for="c in t.categories" :key="c" class="tpl-cat">{{ c }}</span>
          </div>
          <div class="tpl-foot">
            <span class="tpl-count">{{ t.productCount }} 个示例商品</span>
            <button
              class="tpl-apply"
              :disabled="!isManager || applying === t.id || t.id === industry"
              @click="apply(t)"
            >
              {{ applying === t.id ? '应用中…' : t.id === industry ? '已应用' : '应用此模板' }}
            </button>
          </div>
        </div>
      </div>

      <p v-if="!isManager" class="tpl-perm">仅店长可应用业态模板</p>
      <p v-if="doneMsg" class="tpl-done">{{ doneMsg }}</p>
    </div>
  </div>

  <!-- 内嵌模式（向导 Step1 复用） -->
  <div v-if="open && embedded" class="tpl-body">
    <div v-if="error" class="tpl-error">{{ error }}</div>
    <div v-else-if="!templates.length" class="tpl-loading">加载模板中…</div>

    <div v-else class="tpl-grid">
      <div
        v-for="t in templates"
        :key="t.id"
        class="tpl-card"
        :class="{ active: t.id === industry }"
      >
        <div class="tpl-card-head">
          <span class="tpl-name">{{ t.name }}</span>
          <span v-if="t.id === industry" class="tpl-tag">当前业态</span>
        </div>
        <p class="tpl-summary">{{ t.summary }}</p>
        <div class="tpl-cats">
          <span v-for="c in t.categories" :key="c" class="tpl-cat">{{ c }}</span>
        </div>
        <div class="tpl-foot">
          <span class="tpl-count">{{ t.productCount }} 个示例商品</span>
          <button
            class="tpl-apply"
            :disabled="!isManager || applying === t.id || t.id === industry"
            @click="apply(t)"
          >
            {{ applying === t.id ? '应用中…' : t.id === industry ? '已应用' : '应用此模板' }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="!isManager" class="tpl-perm">仅店长可应用业态模板</p>
    <p v-if="doneMsg" class="tpl-done">{{ doneMsg }}</p>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { request } from '../lib/http.js'

// 应用走 POST /catalog/templates/{id}/apply（仅店长，服务端顺带写 industry + setup_done）。
// embedded=true 供 P2 向导 Step1 内嵌复用（不渲染模态外壳，应用后 emit applied）。
const props = defineProps({
  role: { type: String, default: 'cashier' },
  open: { type: Boolean, default: false },
  embedded: { type: Boolean, default: false }
})
const emit = defineEmits(['close', 'applied', 'update:open'])
const isManager = computed(() => props.role === 'manager')

const templates = ref([])
const industry = ref('')
const applying = ref('')
const error = ref('')
const doneMsg = ref('')

async function load() {
  try {
    const [tpls, settings] = await Promise.all([
      request('/pos/v1/catalog/templates'),
      request('/pos/v1/settings')
    ])
    templates.value = tpls || []
    industry.value = (settings && settings.industry) || ''
    error.value = ''
  } catch (e) {
    error.value = '加载失败：' + (e.message || '未知错误')
  }
}

async function apply(t) {
  if (applying.value || !isManager.value) return
  applying.value = t.id
  doneMsg.value = ''
  error.value = ''
  try {
    const res = await request('/pos/v1/catalog/templates/' + t.id + '/apply', { method: 'POST' })
    industry.value = t.id
    doneMsg.value =
      '已导入 ' + res.imported + ' 个示例商品' +
      (res.replacedSeeds ? '，并重置了内置示例商品' : '，未影响已有商品')
    emit('applied', t.id)
  } catch (e) {
    error.value = '应用失败：' + (e.message || '未知错误')
  } finally {
    applying.value = ''
  }
}

watch(
  () => props.open,
  (v) => {
    if (v) load()
  },
  { immediate: true }
)
</script>

<style scoped>
.tpl-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  background: rgba(2, 6, 17, 0.72);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 9vh 24px 24px;
}

.tpl-modal {
  width: min(920px, 100%);
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 20px 22px;
  max-height: 82vh;
  overflow: auto;
}

.tpl-body {
  width: 100%;
}

.tpl-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.tpl-title {
  margin: 0;
  font-size: 18px;
  color: var(--text);
}

.tpl-sub {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--muted);
}

.tpl-close {
  background: transparent;
  border-color: transparent;
  font-size: 20px;
  line-height: 1;
  padding: 2px 8px;
  color: var(--muted);
}

.tpl-close:hover {
  color: var(--text);
}

.tpl-error {
  color: var(--danger);
  font-size: 13px;
  padding: 10px 0;
}

.tpl-loading {
  color: var(--muted);
  font-size: 13px;
  padding: 20px 0;
  text-align: center;
}

.tpl-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.tpl-card {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  background: var(--panel-2);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tpl-card.active {
  border-color: var(--brand);
  box-shadow: 0 0 0 1px var(--brand);
}

.tpl-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tpl-name {
  font-weight: 700;
  font-size: 15px;
  color: var(--text);
}

.tpl-tag {
  flex-shrink: 0;
  border: 1px solid var(--brand);
  color: var(--brand);
  border-radius: 4px;
  padding: 0 6px;
  font-size: 11px;
}

.tpl-summary {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
  min-height: 54px;
}

.tpl-cats {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tpl-cat {
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1px 8px;
  font-size: 11px;
  color: var(--text);
}

.tpl-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
}

.tpl-count {
  font-size: 12px;
  color: var(--muted);
}

.tpl-apply {
  padding: 6px 12px;
  font-size: 12px;
  border-radius: 6px;
  border-color: var(--brand);
  color: var(--brand);
}

.tpl-apply:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tpl-perm {
  margin: 14px 0 0;
  font-size: 12px;
  color: var(--money);
}

.tpl-done {
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--ok);
}

@media (max-width: 760px) {
  .tpl-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
