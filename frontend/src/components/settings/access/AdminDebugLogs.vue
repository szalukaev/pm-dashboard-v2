<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('debug_logs.title') }} <AppHint help :text="$t('debug_logs.help')" /></h4>
      <AppButton variant="ghost" :loading="loading" test-id="debug-refresh" @click="load">{{ $t('common.refresh') }}</AppButton>
    </div>
    <p class="adm-hint">{{ $t('debug_logs.hint') }}</p>

    <!-- Отбор: пользователь, даты, вид запроса, часть адреса -->
    <div class="debug-filters">
      <label class="debug-field">
        <span>{{ $t('debug_logs.user') }}</span>
        <select v-model="filter.user_id" class="adm-select" data-testid="debug-filter-user" @change="apply">
          <option value="">{{ $t('debug_logs.all_users') }}</option>
          <option v-for="u in users" :key="u.id" :value="String(u.id)">
            {{ u.username }}{{ u.debug_until ? ' · ' + $t('debug_logs.on') : '' }}
          </option>
        </select>
      </label>
      <label class="debug-field">
        <span>{{ $t('debug_logs.from') }}</span>
        <input v-model="filter.from" type="date" class="adm-input" data-testid="debug-filter-from" @change="apply" />
      </label>
      <label class="debug-field">
        <span>{{ $t('debug_logs.to') }}</span>
        <input v-model="filter.to" type="date" class="adm-input" data-testid="debug-filter-to" @change="apply" />
      </label>
      <label class="debug-field">
        <span>{{ $t('debug_logs.method') }}</span>
        <select v-model="filter.method" class="adm-select" data-testid="debug-filter-method" @change="apply">
          <option value="">{{ $t('debug_logs.all_methods') }}</option>
          <option v-for="m in METHODS" :key="m" :value="m">{{ $t('debug_logs.methods.' + m) }}</option>
        </select>
      </label>
      <label class="debug-field wide">
        <span>{{ $t('debug_logs.path') }}</span>
        <input v-model="filter.path" type="text" class="adm-input" placeholder="/api/tasks" data-testid="debug-filter-path" @change="apply" @keyup.enter="apply" />
      </label>
      <AppButton variant="danger" :disabled="!filter.user_id" :title="filter.user_id ? '' : $t('debug_logs.clear_need_user')" test-id="debug-clear" @click="clear">
        {{ $t('debug_logs.clear') }}
      </AppButton>
    </div>

    <div class="adm-table-wrap">
      <table class="adm-table adm-stack" data-testid="debug-table">
        <thead>
          <tr>
            <th>{{ $t('debug_logs.col_time') }}</th>
            <th>{{ $t('debug_logs.user') }}</th>
            <th>{{ $t('debug_logs.method') }}</th>
            <th>{{ $t('debug_logs.col_path') }}</th>
            <th class="num">{{ $t('debug_logs.col_status') }}</th>
            <th class="num">{{ $t('debug_logs.col_duration') }}</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id" class="debug-row" :data-testid="'debug-row-' + log.id" @click="open(log)">
            <td class="nowrap">{{ formatAt(log.occurred_at, { seconds: true }) }}</td>
            <td :data-label="$t('debug_logs.user')">{{ log.username || '#' + log.user_id }}</td>
            <td :data-label="$t('debug_logs.method')"><span class="method" :class="log.method.toLowerCase()">{{ log.method }}</span></td>
            <td :data-label="$t('debug_logs.col_path')" class="path">{{ log.path }}<span v-if="log.query" class="adm-muted">?{{ log.query }}</span></td>
            <td :data-label="$t('debug_logs.col_status')" class="num" :class="{ failed: log.status_code >= 400 }">{{ log.status_code }}</td>
            <td :data-label="$t('debug_logs.col_duration')" class="num">{{ log.duration_ms }}</td>
            <td data-label="IP" class="adm-muted">{{ log.ip_address || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-if="!loading && logs.length === 0" class="adm-hint empty" data-testid="debug-empty">{{ $t('debug_logs.empty') }}</p>

    <AppPagination :total="total" :limit="limit" :offset="offset" @update:offset="setOffset" @update:limit="setLimit" />

    <!-- Запись целиком: тела запроса и ответа -->
    <AppModal v-model="detailOpen" :title="detailTitle" width="820px" expandable>
      <template v-if="detail">
        <dl class="detail-meta">
          <dt>{{ $t('debug_logs.col_time') }}</dt><dd>{{ formatAt(detail.occurred_at, { seconds: true }) }}</dd>
          <dt>{{ $t('debug_logs.user') }}</dt><dd>{{ detail.username || '#' + detail.user_id }}</dd>
          <dt>{{ $t('debug_logs.col_status') }}</dt><dd>{{ detail.status_code }}</dd>
          <dt>{{ $t('debug_logs.col_duration') }}</dt><dd>{{ detail.duration_ms }}</dd>
          <dt v-if="detail.query">{{ $t('debug_logs.query') }}</dt><dd v-if="detail.query" class="mono">{{ detail.query }}</dd>
        </dl>
        <h4 class="detail-title">{{ $t('debug_logs.request_body') }}</h4>
        <pre class="json-block" data-testid="debug-request">{{ pretty(detail.request_body) }}</pre>
        <h4 class="detail-title">{{ $t('debug_logs.response_body') }}</h4>
        <pre class="json-block" data-testid="debug-response">{{ pretty(detail.response_body) }}</pre>
      </template>
      <p v-else class="adm-hint">{{ $t('common.loading') }}</p>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import AppButton from '../../ui/AppButton.vue'
import AppModal from '../../ui/AppModal.vue'
import AppPagination from '../../ui/AppPagination.vue'
import AppHint from '../../ui/AppHint.vue'
import { useSwal } from '../../../composables/useSwal'
import { formatDateTime as formatAt } from '../../../utils/format'

interface DebugLog {
  id: number
  occurred_at: string
  user_id: number
  username: string
  method: string
  path: string
  query: string
  status_code: number
  duration_ms: number
  ip_address: string
  request_body?: string
  response_body?: string
}

interface UserItem {
  id: number
  username: string
  debug_until: string | null
}

const METHODS = ['GET', 'POST', 'PUT', 'DELETE']

const { t, locale } = useI18n()
const { toast, confirm } = useSwal()

const users = ref<UserItem[]>([])
const logs = ref<DebugLog[]>([])
const total = ref(0)
const limit = ref(25)
const offset = ref(0)
const loading = ref(false)
const filter = reactive({ user_id: '', from: '', to: '', method: '', path: '' })

async function load() {
  loading.value = true
  try {
    // Пустые поля отбора в запрос не уходят
    const params: Record<string, string | number> = { limit: limit.value, offset: offset.value }
    for (const [key, value] of Object.entries(filter)) {
      if (value) params[key] = value
    }
    const { data } = await axios.get('/api/admin/debug-logs', { params })
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch {
    toast(t('debug_logs.load_error'), 'error')
  } finally {
    loading.value = false
  }
}

// Новый отбор начинается с первой страницы
function apply() {
  offset.value = 0
  load()
}

function setOffset(value: number) {
  offset.value = value
  load()
}

function setLimit(value: number) {
  limit.value = value
  apply()
}

const detailOpen = ref(false)
const detail = ref<DebugLog | null>(null)
const detailTitle = computed(() => (detail.value ? `${detail.value.method} ${detail.value.path}` : t('debug_logs.title')))

async function open(log: DebugLog) {
  detail.value = null
  detailOpen.value = true
  try {
    const { data } = await axios.get(`/api/admin/debug-logs/${log.id}`)
    detail.value = data.log
  } catch {
    detailOpen.value = false
    toast(t('debug_logs.load_error'), 'error')
  }
}

// JSON — с отступами; пометка «не сохранено» и пустое тело — как есть
function pretty(body?: string): string {
  if (!body) return t('debug_logs.no_body')
  // Тело, которое сервер не сохранил: «[тип, N bytes — not saved]»
  const skipped = /^\[(.+), (?:(\d+) bytes|size unknown) — not saved\]$/.exec(body)
  if (skipped) {
    return skipped[2]
      ? t('debug_logs.not_saved', { type: skipped[1], size: Number(skipped[2]).toLocaleString(locale.value) })
      : t('debug_logs.not_saved_unknown', { type: skipped[1] })
  }
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

async function clear() {
  const user = users.value.find(u => String(u.id) === filter.user_id)
  if (!user) return
  const answer = await confirm(t('debug_logs.clear_title', { name: user.username }), t('debug_logs.clear_text'))
  if (!answer.isConfirmed) return
  try {
    const { data } = await axios.delete('/api/admin/debug-logs', { params: { user_id: user.id } })
    toast(t('debug_logs.cleared', { count: data.deleted || 0 }), 'success')
    apply()
  } catch {
    toast(t('debug_logs.clear_error'), 'error')
  }
}

onMounted(async () => {
  try {
    const { data } = await axios.get('/api/admin/users')
    users.value = data.users || []
  } catch {}
  load()
})
</script>

<style scoped>
.debug-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 12px;
  margin-bottom: 16px;
}

.debug-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.debug-field.wide {
  flex: 1;
  min-width: 180px;
}

.debug-row {
  cursor: pointer;
}

.nowrap {
  white-space: nowrap;
}

.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.failed {
  color: var(--danger);
  font-weight: 600;
}

.path {
  max-width: 420px;
  word-break: break-all;
}

.method {
  display: inline-block;
  min-width: 52px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
  color: var(--text-muted);
  background: var(--surface-3);
  border-radius: 9999px;
}

.method.post,
.method.put { color: var(--accent); }
.method.delete { color: var(--danger); }

.empty {
  margin-top: 12px;
  text-align: center;
}

.detail-meta {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 16px;
  margin-bottom: 12px;
  font-size: 13px;
}

.detail-meta dt {
  color: var(--text-muted);
}

.detail-meta dd {
  color: var(--text);
  word-break: break-all;
}

.mono {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
}

.detail-title {
  margin: 14px 0 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.json-block {
  max-height: 320px;
  padding: 12px;
  overflow: auto;
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
}
</style>
