<template>
  <div class="settings-sync">
    <h3 class="section-title">{{ $t('settings.sync.title') }}</h3>
    <p class="section-hint">{{ $t('settings.sync.hint') }}</p>

    <div class="sync-actions">
      <AppButton variant="primary" :loading="syncing" data-testid="sync-run" @click="runSync">
        <RefreshCw :size="14" />
        {{ $t('settings.sync.run') }}
      </AppButton>

      <label class="interval">
        <span class="interval-label">{{ $t('settings.sync.interval') }}</span>
        <input
          v-model="intervalInput"
          type="text"
          inputmode="numeric"
          class="interval-input"
          data-testid="sync-interval"
          @change="saveInterval"
          @keyup.enter="($event.target as HTMLInputElement).blur()"
        />
        <span class="interval-label">{{ $t('settings.sync.minutes') }}</span>
      </label>
    </div>

    <table class="sync-table">
      <thead>
        <tr>
          <th>{{ $t('settings.sync.col_time') }}</th>
          <th>{{ $t('settings.sync.col_duration') }}</th>
          <th>{{ $t('settings.sync.col_issues') }}</th>
          <th>{{ $t('settings.sync.col_status') }}</th>
          <th>{{ $t('settings.sync.col_error') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in logs" :key="log.id">
          <td>{{ formatDateTime(log.started_at) }}</td>
          <td>{{ formatDuration(log.duration_ms) }}</td>
          <td>{{ log.issues_collected }}</td>
          <td>
            <span class="status-badge" :class="log.status">{{ statusLabel(log.status) }}</span>
          </td>
          <td class="error-cell" :title="log.error_text || undefined">
            {{ log.error_text ? truncate(log.error_text, 60) : '—' }}
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="logs.length === 0" class="empty-hint">
      {{ $t('settings.sync.empty') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { RefreshCw } from 'lucide-vue-next'
import { useSwal } from '../../composables/useSwal'
import AppButton from '../ui/AppButton.vue'

interface SyncLog {
  id: number
  started_at: string
  finished_at: string | null
  duration_ms: number | null
  issues_collected: number
  status: string
  error_text: string | null
}

const { t, te, locale } = useI18n()
const { toast } = useSwal()

const logs = ref<SyncLog[]>([])
const syncing = ref(false)

// Interval between periodic syncs, minutes
const interval = ref(5)
const intervalInput = ref('5')
const intervalLimits = ref({ min: 1, max: 1440 })

async function loadSettings() {
  try {
    const { data } = await axios.get('/api/admin/sync-settings')
    interval.value = data.interval_minutes
    intervalInput.value = String(data.interval_minutes)
    intervalLimits.value = { min: data.min, max: data.max }
  } catch {}
}

async function saveInterval() {
  const value = Number(intervalInput.value.trim())
  const { min, max } = intervalLimits.value
  if (!Number.isInteger(value) || value < min || value > max) {
    intervalInput.value = String(interval.value)
    toast(t('settings.sync.interval_invalid', { min, max }), 'error')
    return
  }
  if (value === interval.value) return
  try {
    await axios.put('/api/admin/sync-settings', { interval_minutes: value })
    interval.value = value
    toast(t('settings.sync.interval_saved'), 'success')
  } catch {
    intervalInput.value = String(interval.value)
    toast(t('settings.sync.interval_error'), 'error')
  }
}

function statusLabel(status: string): string {
  const key = `settings.sync.status_${status}`
  return te(key) ? t(key) : status
}

async function loadLogs() {
  try {
    const { data } = await axios.get('/api/admin/sync-log')
    logs.value = data.logs || []
  } catch {}
}

async function runSync() {
  syncing.value = true
  try {
    await axios.post('/api/admin/sync/run')
    toast(t('settings.sync.started'), 'success')
    // The sync runs in the background: show the new entry, then its outcome
    loadLogs()
    setTimeout(loadLogs, 5000)
  } catch (e: any) {
    const code = e?.response?.data?.error
    const key = code === 'SYNC_ALREADY_RUNNING' ? 'already_running'
      : code === 'DATA_SOURCE_NOT_CONFIGURED' ? 'not_configured'
      : 'run_error'
    toast(t(`settings.sync.${key}`), code === 'SYNC_ALREADY_RUNNING' ? 'info' : 'error')
    loadLogs()
  } finally {
    syncing.value = false
  }
}

function formatDateTime(s: string): string {
  if (!s) return '—'
  return new Date(s).toLocaleString(locale.value === 'en' ? 'en-GB' : 'ru-RU', {
    day: '2-digit', month: '2-digit', year: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function formatDuration(ms: number | null): string {
  if (!ms) return '—'
  if (ms < 1000) return `${ms} ${t('settings.sync.ms')}`
  return `${(ms / 1000).toFixed(1)} ${t('settings.sync.sec')}`
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '…' : s
}

onMounted(() => {
  loadLogs()
  loadSettings()
})
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.section-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 20px;
}

.sync-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px 24px;
  margin-bottom: 20px;
}

.interval {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.interval-label {
  font-size: 12px;
  color: var(--text-muted);
}

.interval-input {
  width: 64px;
  padding: 6px 10px;
  font-size: 13px;
  text-align: center;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text-bright);
}

.interval-input:focus {
  border-color: var(--accent);
}

.sync-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  overflow: hidden;
}

.sync-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}

.sync-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
}

.sync-table tr:hover td {
  background: var(--bg-hover);
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 10px;
  font-weight: 500;
  border-radius: 9999px;
}

.status-badge.success {
  background: var(--success)22;
  color: var(--success);
}

.status-badge.running {
  background: var(--warning)22;
  color: var(--warning);
}

.status-badge.error {
  background: var(--danger)22;
  color: var(--danger);
}

.error-cell {
  max-width: 200px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  color: var(--text-faint);
}

.empty-hint {
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
  padding: 24px;
}
</style>
