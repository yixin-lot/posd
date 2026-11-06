<template>
  <nav class="nav64" aria-label="主导航">
    <button
      v-for="t in items"
      :key="t.view"
      type="button"
      class="it"
      :class="{ on: view === t.view }"
      :title="t.menu"
      @click="go(t.view)"
    >
      <svg viewBox="0 0 24 24">
        <path v-for="(d, i) in t.icon" :key="i" :d="d" />
      </svg>
      <span>{{ tr('nav.' + t.view) }}</span>
    </button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'
import { navItems } from '../nav.js'

// 导航清单来自 nav.js（与收银台 ☰ 抽屉同一份），这里只按角色过滤，不再硬编码。
const props = defineProps({
  view: { type: String, default: 'cashier' },
  role: { type: String, default: 'cashier' }
})
const emit = defineEmits(['update:view'])
const items = computed(() => navItems(props.role))

function go(view) {
  if (view !== props.view) emit('update:view', view)
}
import { t as tr } from '../i18n.js'
</script>
