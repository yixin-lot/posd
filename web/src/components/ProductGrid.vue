<template>
  <section class="panel">
    <div class="panel-head">
      <div class="search-wrap">
        <svg class="search-ico" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" stroke-width="1.8" />
          <path d="M16 16l4.5 4.5" fill="none" stroke="currentColor" stroke-width="1.8" />
        </svg>
        <input
          ref="input"
          class="search"
          :value="keyword"
          :placeholder="tr('grid.searchPlaceholder')"
          @input="$emit('update:keyword', $event.target.value)"
          @keydown="onKey"
        />
        <span class="search-kbd">{{ tr('grid.close') }}</span>
      </div>
    </div>
    <div class="cats">
      <button
        v-for="c in categories"
        :key="c"
        class="cat"
        :class="{ active: c === active }"
        @click="$emit('update:active', c)"
      >
        {{ c }}
      </button>
    </div>
    <div class="grid">
      <button
        v-for="p in products"
        :key="p.id"
        class="card"
        :class="{ sel: p.id === selId, off: p.active === false }"
        @click="pick(p)"
      >
        <span class="thumb" :style="thumbStyle(p)">
          <svg viewBox="0 0 24 24" aria-hidden="true" v-html="iconPath(p)"></svg>
        </span>
        <span class="meta">
          <span class="name">
            {{ p.name }}<i v-if="p.hot" class="hot">{{ tr('grid.hot') }}</i><i v-if="p.kind === 'bundle'" class="bundle">套餐</i>
          </span>
          <span class="price">¥{{ Number(p.price).toFixed(0) }}</span>
        </span>
        <span v-if="p.active === false" class="off-tag">{{ tr('grid.off') }}</span>
      </button>
      <div v-if="!products.length" class="empty">{{ tr('grid.noMatch') }}</div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { t as tr } from '../i18n.js'

const props = defineProps({
  products: { type: Array, required: true },
  categories: { type: Array, required: true },
  active: { type: String, required: true },
  keyword: { type: String, default: '' },
  // 全量商品（未过滤）：回车时在全部商品里做条码精确匹配加购。
  all: { type: Array, default: () => [] }
})
const emit = defineEmits(['add', 'update:active', 'update:keyword'])

const input = ref(null)
const selId = ref(null)
function focus() {
  if (input.value) input.value.focus()
}
defineExpose({ focus })

// 回车先做条码精确匹配（命中直接加购并清空），否则维持名称/类目/条码过滤结果。
onMounted(focus)

function pick(p) {
  if (p.active === false) return
  selId.value = p.id
  emit('add', p)
}

// v03 P2：每张卡左侧 40px 圆角图标块，色板按类目分配（同一类目恒定，便于扫视）
const PALETTE = [
  { bg: 'rgba(13,148,136,.18)', fg: '#5EEAD4' },
  { bg: 'rgba(245,158,11,.16)', fg: '#FBBF24' },
  { bg: 'rgba(139,92,246,.18)', fg: '#C4B5FD' },
  { bg: 'rgba(59,130,246,.18)', fg: '#93C5FD' },
  { bg: 'rgba(244,114,182,.16)', fg: '#F9A8D4' },
  { bg: 'rgba(234,179,8,.14)', fg: '#FCD34D' },
  { bg: 'rgba(34,197,94,.16)', fg: '#86EFAC' },
  { bg: 'rgba(148,163,184,.16)', fg: '#CBD5E1' }
]
function colorOf(p) {
  // 设计稿每张卡一个色块：优先按商品名取色，没名字才回落到类目
  const key = String(p.name || '') || String(p.category || '')
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) % 997
  return PALETTE[h % PALETTE.length]
}
function thumbStyle(p) {
  const c = p.active === false ? { bg: 'rgba(148,163,184,.10)', fg: '#64748B' } : colorOf(p)
  return { background: c.bg, color: c.fg }
}

// 线条图标：按类目给一个能认出来的杯子/柠檬/壶/袋，避免每格都一样
const ICONS = {
  杯: '<path d="M7 8h10l-1 9a3 3 0 0 1-3 3h-2a3 3 0 0 1-3-3z" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M11.5 4.5h4l-1 3" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  柠: '<circle cx="11" cy="14" r="6" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M14 8c2-2 5-1 5 2" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  壶: '<path d="M8 9h9v7a3 3 0 0 1-3 3H11a3 3 0 0 1-3-3z" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M17 11h2a2 2 0 0 1 0 4h-2" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M9 6h6" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  咖: '<path d="M6 9h11v5a5 5 0 0 1-5 5h-1a5 5 0 0 1-5-5z" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M17 11h2a2 2 0 0 1 0 4h-2" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  甜: '<path d="M6 17h12L12 6z" fill="none" stroke="currentColor" stroke-width="1.6"/><circle cx="12" cy="6" r="2" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  袋: '<path d="M7 8h10l1 12H6z" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M9 8V6a3 3 0 0 1 6 0v2" fill="none" stroke="currentColor" stroke-width="1.6"/>',
  停: '<path d="M7 8h10l1 12H6z" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M9 8V6a3 3 0 0 1 6 0v2" fill="none" stroke="currentColor" stroke-width="1.6"/>'
}
function iconPath(p) {
  if (p.active === false) return ICONS.停
  const cat = String(p.category || '')
  const name = String(p.name || '')
  if (/咖啡|美式|拿铁|摩卡|意式/.test(cat + name)) return ICONS.咖
  if (/茶|乌龙|龙井|普洱/.test(name) && !/奶/.test(name)) return ICONS.壶
  if (/果/.test(cat)) return ICONS.柠
  if (/甜品|蛋糕|挞|酥|布丁/.test(cat + name)) return ICONS.甜
  if (/小食|零食/.test(cat)) return ICONS.袋
  return ICONS.杯
}

function onKey(e) {
  if (e.key === 'Escape') {
    emit('update:keyword', '')
    focus()
    return
  }
  if (e.key !== 'Enter') return
  // 直接读 DOM value，避免 v-model 异步批处理导致同帧输入+回车时读到旧值。
  const kw = (input.value && input.value.value || '').trim()
  if (!kw) return
  const pool = props.all.length ? props.all : props.products
  const hit = pool.find((p) => p.barcode && p.barcode === kw)
  if (hit) {
    emit('add', hit)
    emit('update:keyword', '')
    focus()
  }
  // 非精确条码：保持现有过滤结果，由用户点卡片加购。
}
</script>
