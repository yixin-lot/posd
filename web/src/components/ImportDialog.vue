<template>
  <div v-if="open" class="idlg-mask" @click.self="close">
    <div class="idlg">
      <div class="idlg-head">
        <h3 class="idlg-title">{{ tr('imp.title') }}</h3>
        <button class="idlg-x" @click="close" :aria-label="tr('imp.close')">×</button>
      </div>

      <p class="idlg-tip">
        {{ tr('imp.columns') }} <code>Name,Barcode,Category,Price,Cost,Hot</code> {{ tr('imp.columnsHint') }}
        <button class="idlg-link" @click="downloadTemplate">{{ tr('imp.template') }}</button>
      </p>

      <div class="idlg-file">
        <input id="csv-input" ref="fileInput" type="file" accept=".csv,text/csv" @change="onFile" />
        <label for="csv-input" class="idlg-file-btn">{{ tr('imp.choose') }}</label>
        <span v-if="fileName" class="idlg-file-name">{{ fileName }}</span>
      </div>

      <div v-if="previewRows.length" class="idlg-preview">
        <p class="idlg-preview-title">{{ tr('imp.preview', { n: previewRows.length }) }}</p>
        <table class="idlg-table">
          <thead>
            <tr>
              <th>{{ tr('imp.row') }}</th><th>{{ tr('imp.name') }}</th><th>{{ tr('imp.barcode') }}</th><th>{{ tr('imp.category') }}</th><th>{{ tr('imp.price') }}</th><th>{{ tr('imp.cost') }}</th><th>{{ tr('imp.hot') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, i) in previewRows" :key="i">
              <td>{{ row.line }}</td>
              <td>{{ row.name }}</td>
              <td>{{ row.barcode }}</td>
              <td>{{ row.category }}</td>
              <td>{{ row.price }}</td>
              <td>{{ row.cost }}</td>
              <td>{{ row.hot ? tr('imp.yes') : tr('imp.no') }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="totalRows > previewRows.length" class="idlg-more">
          {{ tr('imp.moreRows', { n: totalRows - previewRows.length }) }}
        </p>
      </div>

      <div v-if="result" class="idlg-result">
        <p class="idlg-result-line">
          {{ tr('imp.done') }} <b>{{ result.imported }}</b> {{ tr('imp.okRows') }} <b class="bad">{{ result.failed }}</b> {{ tr('imp.failRows') }}
        </p>
        <ul v-if="result.errors.length" class="idlg-errs">
          <li v-for="(e, i) in result.errors.slice(0, 10)" :key="i">
            {{ tr('imp.line', { n: e.line }) }} {{ e.message }}
          </li>
          <li v-if="result.errors.length > 10">{{ tr('imp.allErrors', { n: result.errors.length }) }}</li>
        </ul>
      </div>

      <div class="idlg-foot">
        <button class="idlg-btn" @click="close">{{ tr('imp.close') }}</button>
        <button class="idlg-btn primary" :disabled="!csvText || importing" @click="doImport">
          {{ importing ? tr('imp.importing') : tr('imp.start') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { api } from '../api/index.js'
import { t as tr } from '../i18n.js'

const props = defineProps({ open: { type: Boolean, default: false } })
const emit = defineEmits(['close', 'done'])

const fileInput = ref(null)
const fileName = ref('')
const csvText = ref('')
const previewRows = ref([])
const totalRows = ref(0)
const importing = ref(false)
const result = ref(null)

watch(() => props.open, (v) => {
  if (v) {
    fileName.value = ''
    csvText.value = ''
    previewRows.value = []
    totalRows.value = 0
    result.value = null
  }
})

function downloadTemplate() {
  const csv = '\ufeff名称,条码,类目,售价,成本,热销\n示例商品,690000000001,示例类目,10,5,否\n'
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = tr('imp.templateFile')
  a.click()
  URL.revokeObjectURL(a.href)
}

function onFile(e) {
  const file = e.target.files && e.target.files[0]
  if (!file) return
  fileName.value = file.name
  const reader = new FileReader()
  reader.onload = () => {
    csvText.value = String(reader.result || '')
    parsePreview(csvText.value)
  }
  reader.readAsText(file, 'utf-8')
}

function parsePreview(text) {
  const lines = text.split(/\r?\n/).filter((l) => l.trim() !== '')
  totalRows.value = Math.max(0, lines.length - 1)
  previewRows.value = []
  for (let i = 1; i < lines.length && previewRows.value.length < 20; i++) {
    const c = splitCSV(lines[i])
    previewRows.value.push({
      line: i + 1,
      name: c[0] || '',
      barcode: c[1] || '',
      category: c[2] || '',
      price: c[3] || '',
      cost: c[4] || '',
      hot: c[5] === '是'
    })
  }
}

// 极简 CSV 行拆分（含引号字段时展开）。
function splitCSV(line) {
  const out = []
  let cur = ''
  let inQ = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (inQ) {
      if (ch === '"' && line[i + 1] === '"') {
        cur += '"'
        i++
      } else if (ch === '"') {
        inQ = false
      } else {
        cur += ch
      }
    } else if (ch === '"') {
      inQ = true
    } else if (ch === ',') {
      out.push(cur)
      cur = ''
    } else {
      cur += ch
    }
  }
  out.push(cur)
  return out
}

async function doImport() {
  if (!csvText.value) return
  importing.value = true
  result.value = null
  try {
    result.value = await api.importProducts(csvText.value)
    emit('done')
  } catch (err) {
    result.value = { imported: 0, failed: 1, errors: [{ line: 0, message: err.message || tr('imp.failed') }] }
  } finally {
    importing.value = false
  }
}

function close() {
  emit('close')
}
</script>

<style scoped>
.idlg-mask {
  position: fixed;
  inset: 0;
  background: rgba(10, 16, 28, 0.66);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 60;
}

.idlg {
  width: min(640px, calc(100vw - 40px));
  max-height: calc(100vh - 60px);
  overflow: auto;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 18px 20px;
  color: var(--text);
}

.idlg-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.idlg-title {
  margin: 0;
  font-size: 15px;
}

.idlg-x {
  border: none;
  background: none;
  color: var(--muted);
  font-size: 18px;
  cursor: pointer;
}

.idlg-tip {
  margin: 10px 0;
  font-size: 12px;
  color: var(--muted);
}

.idlg-tip code {
  background: var(--panel-2);
  padding: 1px 5px;
  border-radius: 4px;
  color: var(--text);
}

.idlg-link {
  border: none;
  background: none;
  color: var(--brand);
  cursor: pointer;
  font-size: 12px;
  padding: 0;
}

.idlg-file {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0;
}

.idlg-file input[type='file'] {
  display: none;
}

.idlg-file-btn {
  padding: 7px 14px;
  font-size: 13px;
  border: 1px dashed var(--line);
  border-radius: 8px;
  color: var(--text);
  background: var(--panel-2);
  cursor: pointer;
}

.idlg-file-name {
  font-size: 12px;
  color: var(--muted);
}

.idlg-preview {
  margin-top: 12px;
}

.idlg-preview-title {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--muted);
}

.idlg-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.idlg-table th,
.idlg-table td {
  border: 1px solid var(--line);
  padding: 5px 8px;
  text-align: left;
}

.idlg-table th {
  background: var(--panel-2);
  color: var(--muted);
  font-weight: 500;
}

.idlg-more {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.idlg-result {
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--panel-2);
}

.idlg-result-line {
  margin: 0;
  font-size: 13px;
}

.idlg-result-line .bad {
  color: #b45309;
}

.idlg-errs {
  margin: 8px 0 0;
  padding-left: 18px;
  font-size: 12px;
  color: #b45309;
}

.idlg-foot {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}

.idlg-btn {
  padding: 8px 16px;
  font-size: 13px;
  border-radius: 8px;
  border: 1px solid var(--line);
  color: var(--text);
  background: var(--panel-2);
  cursor: pointer;
}

.idlg-btn.primary {
  border-color: var(--brand);
  color: var(--brand);
}

.idlg-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
