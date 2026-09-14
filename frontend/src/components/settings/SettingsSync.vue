<template>
  <div class="settings-sync">
    <h3 class="section-title">Синхронизация</h3>
    <p class="section-hint">Журнал запусков воркера синхронизации данных из системы-источника.</p>

    <div class="sync-actions">
      <AppButton variant="primary" :loading="syncing" @click="runSync">
        <RefreshCw :size="14" />
        Запустить синхронизацию сейчас
      </AppButton>
    </div>

    <table class="sync-table">
      <thead>
        <tr>
          <th>Дата/время</th>
          <th>Длительность</th>
          <th>Задач собрано</th>
          <th>Статус</th>
          <th>Ошибка</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in logs" :key="log.id">
          <td>{{ formatDateTime(log.started_at) }}</td>
          <td>{{ formatDuration(log.duration_ms) }}</td>
          <td>{{ log.issues_collected }}</td>
          <td>
            <span class="status-badge" :class="log.status">{{ log.status }}</span>
          </td>
          <td class="error-cell" :title="log.error_text || undefined">
            {{ log.error_text ? truncate(log.error_text, 60) : '—' }}
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="logs.length === 0" class="empty-hint">
      Пока не было запусков синхронизации
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { RefreshCw } from 'lucide-vue-next'
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

const logs = ref<SyncLog[]>([])
const syncing = ref(false)

async function loadLogs() {
  try {
    const { data } = await axios.get('/api/admin/sync-log')
    logs.value = data.logs || []
  } catch {}
}

async function runSync() {
  syncing.value = true
  try {
    await axios.post('/api/admin/sync-now')
    // Reload after a short delay
    setTimeout(loadLogs, 3000)
  } catch {}
  finally { syncing.value = false }
}

function formatDateTime(s: string): string {
  if (!s) return '—'
  return new Date(s).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function formatDuration(ms: number | null): string {
  if (!ms) return '—'
  if (ms < 1000) return ms + ' мс'
  return (ms / 1000).toFixed(1) + ' сек'
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '…' : s
}

onMounted(loadLogs)
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
  margin-bottom: 20px;
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
