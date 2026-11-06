<template>
  <div class="es">
    <svg class="es-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 9l1.5-5h15L21 9" />
      <path d="M3 9v10a1 1 0 0 0 1 1h16a1 1 0 0 0 1-1V9" />
      <path d="M3 9h18" />
      <path d="M9 13h6" />
    </svg>
    <h3 class="es-title">{{ title }}</h3>
    <p class="es-desc">{{ desc }}</p>
    <div v-if="actions.length" class="es-actions">
      <button
        v-for="(a, i) in actions"
        :key="i"
        class="es-btn"
        :class="{ primary: a.primary }"
        @click="a.onClick && a.onClick()"
      >
        {{ a.label }}
      </button>
    </div>
  </div>
</template>

<script setup>
// 用法：<EmptyState title="…" desc="…" :actions="[{label, primary, onClick}]" />
defineProps({
  title: { type: String, default: '' },
  desc: { type: String, default: '' },
  actions: { type: Array, default: () => [] }
})
</script>

<style scoped>
.es {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 34px 16px;
  text-align: center;
}

.es-icon {
  width: 34px;
  height: 34px;
  color: var(--muted);
  opacity: 0.8;
}

.es-title {
  margin: 4px 0 0;
  font-size: 15px;
  color: var(--text);
}

.es-desc {
  margin: 0;
  max-width: 360px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.6;
}

.es-actions {
  display: flex;
  gap: 10px;
  margin-top: 6px;
}

.es-btn {
  padding: 7px 14px;
  font-size: 13px;
  border-radius: 8px;
}

.es-btn.primary {
  /* 背景沿用全局 .primary（青绿实心）；文字必须用深色而非品牌青绿，
     否则青绿底 + 青绿字同色隐形（Bug：桌台按钮无文字） */
  background: var(--brand);
  border-color: var(--brand);
  color: #04211f;
}
</style>
