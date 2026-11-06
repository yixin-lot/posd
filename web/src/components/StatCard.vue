<template>
  <div class="stat-card">
    <span class="stat-label">{{ label }}</span>
    <span class="stat-value">{{ value }}</span>
    <span v-if="sub" class="stat-sub">{{ sub }}</span>
  </div>
</template>

<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  sub: { type: String, default: '' }
})
</script>
