<template>
  <section class="panel">
    <div class="panel-head">
      <h2>购物车</h2>
      <span class="hint">{{ items.length }} 行</span>
      <span class="grow" style="flex: 1"></span>
      <button :disabled="!items.length" @click="$emit('clear')">清空</button>
    </div>

    <div class="cart-items" v-if="items.length">
      <div class="cart-row" v-for="it in items" :key="it.id">
        <div>
          <div>{{ it.name }}</div>
          <div class="sub">
            ¥{{ it.price.toFixed(2) }} × {{ it.qty }}
            <span v-if="it.price !== it.listPrice" class="reprice-tag">原价 ¥{{ it.listPrice.toFixed(2) }}</span>
          </div>
        </div>
        <div class="qty">
          <button @click="$emit('dec', it)">-</button>
          <span>{{ it.qty }}</span>
          <button @click="$emit('inc', it)">+</button>
          <button v-if="it.price !== it.listPrice" class="reprice-on" @click="$emit('reprice', it)">改</button>
          <button v-else @click="$emit('reprice', it)">改价</button>
          <button @click="$emit('remove', it)">×</button>
        </div>
      </div>
    </div>
    <div v-else class="empty">
      <svg class="empty-icon" viewBox="0 0 48 48" width="44" height="44" aria-hidden="true">
        <path d="M10 14h28l-3 18H13z" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linejoin="round"/>
        <circle cx="17" cy="36" r="2.6" fill="currentColor"/>
        <circle cx="31" cy="36" r="2.6" fill="currentColor"/>
        <path d="M20 20l4-5 4 5" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <div class="empty-title">购物车是空的</div>
      <div class="empty-sub">点左侧商品加购，或扫条码 / 搜名称（F1）</div>
    </div>
  </section>
</template>

<script setup>
defineProps({
  items: { type: Array, required: true }
})
defineEmits(['inc', 'dec', 'remove', 'clear', 'reprice'])
</script>
