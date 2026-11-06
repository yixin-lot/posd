<template>
  <section class="panel">
    <div class="panel-head">
      <h2>商品</h2>
      <input
        ref="input"
        class="search"
        :value="keyword"
        placeholder="扫条码 / 拼音 / 名称（F1）"
        @input="$emit('update:keyword', $event.target.value)"
        @keydown="onKey"
      />
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
      <button v-for="p in products" :key="p.id" class="card" @click="$emit('add', p)">
        <span class="name">{{ p.name }}</span>
        <span class="hot" v-if="p.hot">热销</span>
        <span class="bundle" v-if="p.kind === 'bundle'">套餐</span>
        <span class="price">¥{{ p.price.toFixed(2) }}</span>
      </button>
      <div v-if="!products.length" class="empty">没有匹配的商品</div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'

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
function focus() {
  if (input.value) input.value.focus()
}
defineExpose({ focus })

// 回车先做条码精确匹配（命中直接加购并清空），否则维持名称/类目/条码过滤结果。
onMounted(focus)

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
