<template>
  <div class="retention" data-testid="data-retention">
    <h4 class="adm-title">{{ $t('settings.retention.title') }}</h4>
    <p class="adm-hint">{{ $t('settings.retention.hint') }}</p>

    <p class="stored" data-testid="retention-stats">
      <template v-if="stats && stats.issue_rows + stats.daily_rows > 0">
        {{ $t('settings.retention.stored', {
          rows: (stats.issue_rows + stats.daily_rows).toLocaleString(),
          days: stats.days,
          from: formatDay(stats.first_date),
          to: formatDay(stats.last_date),
        }) }}
      </template>
      <template v-else>{{ $t('settings.retention.stored_none') }}</template>
    </p>

    <div class="retention-row">
      <label class="field">
        <span class="field-label">{{ $t('settings.retention.days') }}</span>
        <input
          v-model="daysInput"
          type="text"
          inputmode="numeric"
          class="days-input"
          data-testid="retention-days"
          @change="saveDays"
          @keyup.enter="($event.target as HTMLInputElement).blur()"
        />
      </label>

      <label class="check">
        <input type="checkbox" :checked="enabled" data-testid="retention-enabled" @change="saveEnabled(($event.target as HTMLInputElement).checked)" />
        {{ $t('settings.retention.auto', { time: cleanupTime }) }}
      </label>
    </div>

    <div class="retention-actions">
      <AppButton variant="ghost" :loading="busy" test-id="retention-cleanup" @click="cleanup">
        {{ $t('settings.retention.cleanup_now') }}
      </AppButton>
      <AppButton variant="danger" :loading="busy" test-id="retention-purge" @click="purge">
        {{ $t('settings.retention.purge') }}
      </AppButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useSwal } from '../../composables/useSwal'
import { useWebSocket } from '../../composables/useWebSocket'
import AppButton from '../ui/AppButton.vue'

interface Stats {
  daily_rows: number
  issue_rows: number
  first_date: string | null
  last_date: string | null
  days: number
}

const { t, locale } = useI18n()
const { toast, confirm } = useSwal()

const days = ref(90)
const daysInput = ref('90')
const limits = ref({ min: 7, max: 365 })
const enabled = ref(true)
const cleanupTime = ref('02:00')
const stats = ref<Stats | null>(null)
const busy = ref(false)

async function load() {
  try {
    const { data } = await axios.get('/api/admin/data-retention')
    days.value = data.retention_days
    daysInput.value = String(data.retention_days)
    limits.value = { min: data.min, max: data.max }
    enabled.value = data.cleanup_enabled
    cleanupTime.value = data.cleanup_time
    stats.value = data.stats
  } catch {}
}

async function saveDays() {
  const value = Number(daysInput.value.trim())
  const { min, max } = limits.value
  if (!Number.isInteger(value) || value < min || value > max) {
    daysInput.value = String(days.value)
    toast(t('settings.retention.days_invalid', { min, max }), 'error')
    return
  }
  if (value === days.value) return
  try {
    await axios.put('/api/admin/data-retention', { retention_days: value })
    days.value = value
    toast(t('settings.retention.saved'), 'success')
  } catch {
    daysInput.value = String(days.value)
    toast(t('settings.retention.save_error'), 'error')
  }
}

async function saveEnabled(value: boolean) {
  try {
    await axios.put('/api/admin/data-retention', { cleanup_enabled: value })
    enabled.value = value
    toast(t('settings.retention.saved'), 'success')
  } catch {
    toast(t('settings.retention.save_error'), 'error')
    load()
  }
}

async function run(url: string) {
  busy.value = true
  try {
    const { data } = await axios.post(url)
    toast(t('settings.retention.deleted', { count: (data.deleted || 0).toLocaleString() }), 'success')
  } catch (e: any) {
    const busyNow = e?.response?.data?.error === 'CLEANUP_ALREADY_RUNNING'
    toast(t(busyNow ? 'settings.retention.already_running' : 'settings.retention.cleanup_error'), busyNow ? 'info' : 'error')
  } finally {
    busy.value = false
    load()
  }
}

async function cleanup() {
  const answer = await confirm(t('settings.retention.cleanup_confirm_title'), t('settings.retention.cleanup_confirm_text', { days: days.value }))
  if (answer.isConfirmed) run('/api/admin/snapshots/cleanup')
}

async function purge() {
  const answer = await confirm(t('settings.retention.purge_confirm_title'), t('settings.retention.purge_confirm_text'))
  if (answer.isConfirmed) run('/api/admin/snapshots/purge')
}

function formatDay(value: string | null): string {
  if (!value) return '—'
  return new Date(value + 'T00:00:00').toLocaleDateString(locale.value === 'en' ? 'en-GB' : 'ru-RU')
}

// The figures follow the sync (it writes the snapshot of the day) and the
// cleanup, whoever started them
const { on, off } = useWebSocket()

onMounted(() => {
  load()
  on('collection-complete', load)
  on('workers-changed', load)
})

onUnmounted(() => {
  off('collection-complete', load)
  off('workers-changed', load)
})
</script>

<style scoped>
.retention {
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid var(--hairline);
}

.stored {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.retention-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 24px;
  margin-bottom: 16px;
}

.field {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label,
.check {
  font-size: 13px;
  color: var(--text);
}

.days-input {
  width: 64px;
  padding: 6px 8px;
  font-size: 13px;
  text-align: center;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.days-input:focus {
  border-color: var(--accent);
}

.check {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.retention-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
