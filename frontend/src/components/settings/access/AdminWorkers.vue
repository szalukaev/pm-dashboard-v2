<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('workers.title') }}</h4>
      <span v-if="serverTime" class="server-time" data-testid="workers-server-time">
        {{ $t('workers.server_time', { time: serverTime, zone: timezone }) }}
      </span>
    </div>
    <p class="adm-hint">{{ $t('workers.hint') }}</p>

    <div class="adm-table-wrap">
      <table class="adm-table" data-testid="workers-table">
        <thead>
          <tr>
            <th>{{ $t('workers.col_worker') }}</th>
            <th>{{ $t('workers.col_status') }}</th>
            <th>{{ $t('workers.col_last_start') }}</th>
            <th>{{ $t('workers.col_duration') }}</th>
            <th>{{ $t('workers.col_processed') }}</th>
            <th>{{ $t('workers.col_actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in workers" :key="w.key" :data-testid="'worker-' + w.key">
            <td>
              <div class="worker-name">{{ $t('workers.names.' + w.key) }}</div>
              <div class="worker-schedule">{{ scheduleText(w) }}</div>
            </td>
            <td>
              <span class="state" :class="w.state" :data-testid="'worker-state-' + w.key">{{ $t('workers.states.' + w.state) }}</span>
              <div v-if="w.error && w.state === 'error'" class="worker-error" :title="errorText(w)">{{ errorText(w) }}</div>
            </td>
            <td class="num">{{ formatDateTime(w.last_start) }}</td>
            <td class="num">{{ formatDuration(w.duration_ms) }}</td>
            <td class="num">{{ processedText(w) }}</td>
            <td>
              <div class="actions">
                <AppButton v-if="w.running && w.key === 'sync'" variant="danger" :test-id="'worker-stop-' + w.key" @click="act(w, 'stop')">
                  {{ $t('workers.stop') }}
                </AppButton>
                <template v-else-if="!w.running">
                  <AppButton variant="primary" :test-id="'worker-run-' + w.key" @click="act(w, 'run')">
                    {{ $t('workers.run_now') }}
                  </AppButton>
                  <AppButton v-if="w.key === 'sync'" variant="ghost" test-id="worker-run-full" @click="act(w, 'run', { full: true })">
                    {{ $t('workers.run_full') }}
                  </AppButton>
                </template>
                <AppButton variant="ghost" :test-id="'worker-toggle-' + w.key" @click="act(w, w.enabled ? 'disable' : 'enable')">
                  {{ $t(w.enabled ? 'workers.disable' : 'workers.enable') }}
                </AppButton>
                <AppButton variant="ghost" :test-id="'worker-schedule-' + w.key" @click="openSchedule(w)">
                  {{ $t('workers.schedule') }}
                </AppButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Schedule of a worker -->
    <AppModal v-model="scheduleOpen" :title="scheduleTitle" width="420px">
      <form class="schedule-form" @submit.prevent="saveSchedule">
        <template v-if="scheduleKey === 'sync'">
          <label class="schedule-field">
            <span>{{ $t('workers.interval') }}</span>
            <input v-model="schedule.interval" type="text" inputmode="numeric" class="adm-input" data-testid="schedule-interval" />
          </label>
          <label class="schedule-field">
            <span>{{ $t('workers.full_sync_time') }}</span>
            <input v-model="schedule.fullTime" type="time" class="adm-input" data-testid="schedule-full-time" />
          </label>
          <p class="adm-hint">{{ $t('workers.full_sync_hint') }}</p>
        </template>
        <template v-else>
          <label class="schedule-field">
            <span>{{ $t('workers.cleanup_time') }}</span>
            <input v-model="schedule.time" type="time" class="adm-input" data-testid="schedule-time" />
          </label>
          <p class="adm-hint">{{ $t('workers.cleanup_hint') }}</p>
        </template>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="scheduleOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" test-id="schedule-save" @click="saveSchedule">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useSwal } from '../../../composables/useSwal'
import { useWebSocket } from '../../../composables/useWebSocket'
import AppButton from '../../ui/AppButton.vue'
import AppModal from '../../ui/AppModal.vue'

interface Worker {
  key: 'sync' | 'cleanup'
  // working | stopped | error | starting
  state: string
  enabled: boolean
  running: boolean
  last_start: string | null
  duration_ms: number | null
  processed: number | null
  error: string
  // "partial": error holds "<failed>/<total>" projects of a finished sync
  error_kind: string
  schedule: Record<string, any>
}

const { t, te, locale } = useI18n()
const { toast } = useSwal()

const workers = ref<Worker[]>([])
const serverTime = ref('')
const timezone = ref('')

async function load() {
  try {
    const { data } = await axios.get('/api/admin/workers')
    workers.value = data.workers || []
    timezone.value = data.timezone || ''
    // The clock of the server, not of this computer: schedules follow it
    serverTime.value = String(data.server_time || '').slice(11, 16)
  } catch {
    toast(t('workers.load_error'), 'error')
  }
}

function errorMessage(e: any, fallback: string): string {
  const key = `workers.errors.${e?.response?.data?.error}`
  return te(key) ? t(key) : t(fallback)
}

async function act(w: Worker, action: 'run' | 'stop' | 'enable' | 'disable', body?: Record<string, unknown>) {
  try {
    await axios.post(`/api/admin/workers/${w.key}/${action}`, body || {})
    if (action === 'run') toast(t('workers.started'), 'success')
  } catch (e) {
    toast(errorMessage(e, 'workers.action_error'), 'error')
  } finally {
    load()
  }
}

// ─── Schedule ───

const scheduleOpen = ref(false)
const scheduleKey = ref<Worker['key']>('sync')
const schedule = reactive({ interval: '5', fullTime: '03:00', time: '02:00' })
const scheduleTitle = computed(() => t('workers.schedule_title', { name: t('workers.names.' + scheduleKey.value) }))

function openSchedule(w: Worker) {
  scheduleKey.value = w.key
  schedule.interval = String(w.schedule.interval_minutes ?? 5)
  schedule.fullTime = w.schedule.full_sync_time || '03:00'
  schedule.time = w.schedule.time || '02:00'
  scheduleOpen.value = true
}

async function saveSchedule() {
  const body = scheduleKey.value === 'sync'
    ? { interval_minutes: Number(schedule.interval), full_sync_time: schedule.fullTime }
    : { time: schedule.time }
  try {
    await axios.put(`/api/admin/workers/${scheduleKey.value}/schedule`, body)
    scheduleOpen.value = false
    toast(t('workers.schedule_saved'), 'success')
  } catch (e) {
    toast(errorMessage(e, 'workers.schedule_error'), 'error')
  } finally {
    load()
  }
}

// ─── Texts ───

function scheduleText(w: Worker): string {
  if (w.key === 'sync') {
    return t('workers.schedule_sync', { minutes: w.schedule.interval_minutes, time: w.schedule.full_sync_time })
  }
  return t('workers.schedule_cleanup', { time: w.schedule.time, days: w.schedule.retention_days })
}

function processedText(w: Worker): string {
  if (w.processed === null || w.processed === undefined) return '—'
  return t('workers.processed.' + w.key, { count: w.processed.toLocaleString() })
}

function errorText(w: Worker): string {
  if (w.error_kind === 'partial') {
    const [failed, total] = w.error.split('/')
    return t('workers.partial', { failed, total })
  }
  return w.error
}

function formatDateTime(value: string | null): string {
  if (!value) return '—'
  return new Date(value).toLocaleString(locale.value === 'en' ? 'en-GB' : 'ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function formatDuration(ms: number | null): string {
  if (ms === null || ms === undefined) return '—'
  const seconds = Math.round(ms / 1000)
  if (seconds < 1) return t('workers.duration_ms', { ms })
  const minutes = Math.floor(seconds / 60)
  return minutes > 0
    ? t('workers.duration_min', { min: minutes, sec: seconds % 60 })
    : t('workers.duration_sec', { sec: seconds })
}

// The table follows the workers without a reload
const { on, off } = useWebSocket()

onMounted(() => {
  load()
  on('workers-changed', load)
  on('sync-status', load)
})

onUnmounted(() => {
  off('workers-changed', load)
  off('sync-status', load)
})
</script>

<style scoped>
.server-time {
  font-size: 12px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.worker-name {
  font-weight: 600;
  color: var(--text-bright);
}

.worker-schedule {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-muted);
}

.num {
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.state {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 9999px;
  white-space: nowrap;
}

.state.working {
  color: var(--success);
  background: color-mix(in srgb, var(--success) 14%, transparent);
}

.state.stopped {
  color: var(--text-muted);
  background: var(--tag-bg);
}

.state.error {
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 14%, transparent);
}

.state.starting {
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 14%, transparent);
}

.worker-error {
  max-width: 260px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--danger);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.schedule-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.schedule-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  font-size: 13px;
  color: var(--text);
}

.schedule-field .adm-input {
  width: 120px;
}
</style>
