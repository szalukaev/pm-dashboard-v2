<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('invoice_template.title') }}</h4>
    </div>
    <p class="adm-hint">{{ $t('invoice_template.hint') }}</p>

    <!-- Which template is in force -->
    <div class="current" data-testid="invoice-template-current">
      <FileText :size="18" class="current-icon" />
      <div class="current-text">
        <div class="current-name">
          {{ info?.custom ? (info.file_name || $t('invoice_template.custom')) : $t('invoice_template.built_in') }}
          <span class="kind" :class="{ custom: info?.custom }">
            {{ $t(info?.custom ? 'invoice_template.kind_custom' : 'invoice_template.kind_built_in') }}
          </span>
        </div>
        <div v-if="info?.custom && info.uploaded_at" class="current-meta">
          {{ $t('invoice_template.uploaded', { date: formatDateTime(info.uploaded_at), user: info.uploaded_by || '—' }) }}
        </div>
      </div>
    </div>

    <div class="actions">
      <input ref="fileInput" type="file" accept=".docx" class="file-input" data-testid="invoice-template-file" @change="onFile" />
      <AppButton variant="primary" :loading="uploading" test-id="invoice-template-upload" @click="fileInput?.click()">
        <Upload :size="14" /> {{ $t('invoice_template.upload') }}
      </AppButton>
      <AppButton variant="ghost" test-id="invoice-template-download" @click="download('file')">
        <Download :size="14" /> {{ $t('invoice_template.download') }}
      </AppButton>
      <AppButton variant="ghost" test-id="invoice-template-preview" @click="download('preview')">
        <Eye :size="14" /> {{ $t('invoice_template.preview') }}
      </AppButton>
      <AppButton v-if="info?.custom" variant="danger" test-id="invoice-template-reset" @click="reset">
        {{ $t('invoice_template.reset') }}
      </AppButton>
    </div>

    <!-- What a check of the template finds -->
    <div v-if="info?.report" class="report" data-testid="invoice-template-report">
      <div v-if="problems.length === 0" class="report-line ok">
        <CheckCircle2 :size="14" /> {{ $t('invoice_template.report_ok', { count: info.report.found.length }) }}
      </div>
      <template v-else>
        <div v-for="p in problems" :key="p" class="report-line warn">
          <AlertTriangle :size="14" /> {{ p }}
        </div>
      </template>
    </div>

    <!-- The lines of an invoice -->
    <h4 class="adm-title block-title">{{ $t('invoice_template.items_title') }}</h4>
    <div class="ways">
      <div class="way">
        <div class="way-title">{{ $t('invoice_template.way_table_title') }}</div>
        <p>{{ $t('invoice_template.way_table_text', { v: braces('items_table') }) }}</p>
      </div>
      <div class="way">
        <div class="way-title">{{ $t('invoice_template.way_row_title') }}</div>
        <p>{{ $t('invoice_template.way_row_text', { a: braces('item_name'), b: braces('item_amount') }) }}</p>
      </div>
    </div>

    <!-- Every variable -->
    <h4 class="adm-title block-title">{{ $t('invoice_template.variables_title') }}</h4>
    <p class="adm-hint">{{ $t('invoice_template.variables_hint') }}</p>
    <div class="adm-table-wrap">
      <table class="adm-table" data-testid="invoice-template-variables">
        <thead>
          <tr>
            <th>{{ $t('invoice_template.col_variable') }}</th>
            <th>{{ $t('invoice_template.col_meaning') }}</th>
            <th>{{ $t('invoice_template.col_example') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="group in groups" :key="group.key">
            <tr class="group-row">
              <td colspan="3">{{ $t('invoice_template.groups.' + group.key) }}</td>
            </tr>
            <tr v-for="v in group.variables" :key="v.name">
              <td>
                <button type="button" class="variable" :class="{ used: used.has(v.name) }" :title="$t('invoice_template.copy_hint')" @click="copy(v.name)">
                  {{ braces(v.name) }}
                </button>
              </td>
              <td>{{ $t('invoice_template.vars.' + v.name) }}</td>
              <td class="example">{{ v.name === 'items_table' ? $t('invoice_template.example_table') : v.example }}</td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { formatDateTime as formatAt } from '../../../utils/format'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { FileText, Upload, Download, Eye, CheckCircle2, AlertTriangle } from 'lucide-vue-next'
import AppButton from '../../ui/AppButton.vue'
import { useSwal } from '../../../composables/useSwal'
import { copyText } from '../../../utils/clipboard'
import { downloadFile } from '../../../utils/download'

interface Variable {
  name: string
  // invoice | totals | customer | contract | items
  group: string
  example: string
}

interface TemplateInfo {
  custom: boolean
  file_name?: string
  uploaded_at?: string
  uploaded_by?: string
  size: number
  max_size: number
  variables: Variable[]
  report?: { found: string[]; unknown: string[]; warnings: string[] }
}

const { t, te } = useI18n()
const { toast, confirm } = useSwal()

const info = ref<TemplateInfo | null>(null)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// A variable as it is written in a template
function braces(name: string): string {
  return '{{' + name + '}}'
}

async function load() {
  try {
    const { data } = await axios.get('/api/admin/invoice-template')
    info.value = data
  } catch {
    toast(t('invoice_template.load_error'), 'error')
  }
}

const groupOrder = ['invoice', 'totals', 'customer', 'contract', 'items']
const groups = computed(() =>
  groupOrder
    .map(key => ({ key, variables: (info.value?.variables || []).filter(v => v.group === key) }))
    .filter(g => g.variables.length > 0)
)

// The variables the template in force uses
const used = computed(() => new Set(info.value?.report?.found || []))

// What is wrong with the template, in words
const problems = computed(() => {
  const report = info.value?.report
  if (!report) return []
  const list = report.warnings.map(code => {
    const key = `invoice_template.warnings.${code}`
    return te(key) ? t(key, { v: braces('items_table') }) : code
  })
  if (report.unknown.length > 0) {
    list.push(t('invoice_template.unknown', { list: report.unknown.join(', ') }))
  }
  return list
})

function errorText(e: any, fallback: string): string {
  const key = `invoice_template.errors.${e?.response?.data?.error}`
  const maxMb = Math.round((info.value?.max_size || 0) / 1024 / 1024)
  return te(key) ? t(key, { size: maxMb }) : t(fallback)
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  // The same file may be chosen again after it was corrected
  input.value = ''
  if (!file) return

  const form = new FormData()
  form.append('file', file)
  uploading.value = true
  try {
    await axios.post('/api/admin/invoice-template', form)
    toast(t('invoice_template.uploaded_ok'), 'success')
  } catch (err) {
    toast(errorText(err, 'invoice_template.upload_error'), 'error')
  } finally {
    uploading.value = false
    load()
  }
}

async function download(what: 'file' | 'preview') {
  try {
    await downloadFile(`/api/admin/invoice-template/${what}`, what === 'file' ? 'invoice_template.docx' : 'invoice_preview.docx')
  } catch {
    toast(t('invoice_template.download_error'), 'error')
  }
}

async function reset() {
  const answer = await confirm(t('invoice_template.reset_title'), t('invoice_template.reset_text'))
  if (!answer.isConfirmed) return
  try {
    await axios.delete('/api/admin/invoice-template')
    toast(t('invoice_template.reset_ok'), 'success')
  } catch {
    toast(t('invoice_template.upload_error'), 'error')
  } finally {
    load()
  }
}

async function copy(name: string) {
  const copied = await copyText(braces(name))
  toast(copied ? t('invoice_template.copied', { v: braces(name) }) : t('invoice_template.copy_failed'), copied ? 'success' : 'error')
}

function formatDateTime(value: string): string {
  return formatAt(value)
}

onMounted(load)
</script>

<style scoped>
.current {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  margin-bottom: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.current-icon {
  flex-shrink: 0;
  color: var(--accent);
}

.current-name {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.current-meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-muted);
}

.kind {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 9999px;
  color: var(--text-muted);
  background: var(--tag-bg);
}

.kind.custom {
  color: var(--success);
  background: color-mix(in srgb, var(--success) 14%, transparent);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.file-input {
  display: none;
}

.report {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}

.report-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  line-height: 1.4;
}

.report-line svg {
  flex-shrink: 0;
  margin-top: 2px;
}

.report-line.ok {
  color: var(--success);
}

.report-line.warn {
  color: var(--warning);
}

.block-title {
  margin-top: 28px;
  margin-bottom: 10px;
}

.ways {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.way {
  padding: 12px 14px;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text);
}

.way-title {
  margin-bottom: 4px;
  font-weight: 600;
  color: var(--text-bright);
}

.group-row td {
  padding-top: 14px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  color: var(--text-muted);
  background: transparent;
}

.variable {
  padding: 2px 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--text);
  cursor: pointer;
  white-space: nowrap;
}

.variable:hover {
  border-color: var(--accent);
  color: var(--text-bright);
}

/* The template in force uses this variable */
.variable.used {
  border-color: color-mix(in srgb, var(--success) 50%, transparent);
}

.example {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
