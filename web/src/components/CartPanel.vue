<template>
  <section class="panel cart-panel">
    <div class="panel-head">
      <h2>{{ tr('cart.title') }}</h2>
      <span class="hint">{{ tr('cart.items', { n: items.length }) }}</span>
    </div>

    <div class="cart-items" v-if="items.length">
      <div class="cart-row" v-for="it in items" :key="it.id">
        <div class="row-top">
          <span class="row-name">{{ it.name }}</span>
          <button class="repricing-ico" :title="tr('appx.reprice')" @click="$emit('reprice', it)">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 20h4l10-10-4-4L4 16z" fill="none" stroke="currentColor" stroke-width="1.6" />
            </svg>
          </button>
          <span class="row-amount">¥{{ (it.price * it.qty).toFixed(0) }}</span>
        </div>
        <div class="row-bottom">
          <span class="row-unit">
            {{ money(it.price) }} × {{ it.qty }}
            <em v-if="it.price !== it.listPrice" class="reprice-tag">{{ tr('cart.original') }} {{ money(it.listPrice) }}</em>
          </span>
          <span class="qty">
            <button @click="$emit('dec', it)">−</button>
            <span>{{ it.qty }}</span>
            <button class="plus" @click="$emit('inc', it)">+</button>
          </span>
        </div>
        <button class="row-del" :title="tr('appx.remove')" @click="$emit('remove', it)">×</button>
      </div>
    </div>
    <div v-else class="empty">
      <svg class="empty-icon" viewBox="0 0 48 48" width="44" height="44" aria-hidden="true">
        <path d="M10 14h28l-3 18H13z" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linejoin="round"/>
        <circle cx="17" cy="36" r="2.6" fill="currentColor"/>
        <circle cx="31" cy="36" r="2.6" fill="currentColor"/>
        <path d="M20 20l4-5 4 5" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <div class="empty-title">{{ tr('cart.emptyTitle') }}</div>
      <div class="empty-sub">{{ tr('cart.emptySub') }}</div>
    </div>

    <!-- v03 P2：购物车底部四键一行（挂单 / 取单 / 折扣 / 清空） -->
    <div class="cart-foot">
      <button :disabled="!items.length" @click="$emit('hold')">{{ tr('cart.hold') }} <span class="kbd">F4</span></button>
      <button :disabled="!heldCount" @click="$emit('pickup')">{{ tr('cart.pickup') }} <span class="kbd">F5</span></button>
      <button :disabled="!items.length" @click="$emit('discount')">{{ tr('cart.discount') }}</button>
      <button :disabled="!items.length" @click="$emit('clear')">×</button>
    </div>
  </section>
</template>

<script setup>
defineProps({
  items: { type: Array, required: true },
  heldCount: { type: Number, default: 0 }
})
defineEmits(['inc', 'dec', 'remove', 'clear', 'reprice', 'hold', 'pickup', 'discount'])

// 设计稿单价是整数显示（¥13 × 2）；有角分才补两位
const fmt = (n) => (Number.isInteger(n) ? String(n) : Number(n).toFixed(2))
import { t as tr, money } from '../i18n.js'
</script>
