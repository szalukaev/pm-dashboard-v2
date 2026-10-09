<template>
  <div
    class="sync-status"
    :class="status"
    :title="statusText"
    data-testid="sync-status"
  >
    <div class="sync-dot"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useWebSocket } from '../../composables/useWebSocket'

// State of the background sync with the data source:
// green — the last sync succeeded, yellow — a sync is running,
// red — the last attempt failed, grey — there was no sync yet.
type SyncState = 'success' | 'running' | 'error' | 'unknown'

const { t, locale } = useI18n()
const { on, off } = useWebSocket()

const status = ref<SyncState>('unknown')
const at = ref<string | null>(null)

function apply(data: { status?: string; at?: string | null }) {
  status.value = ['success', 'running', 'error'].includes(data?.status || '') ? (data.status as SyncState) : 'unknown'
  at.value = data?.at || null
}

async function load() {
  try {
    const { data } = await axios.get('/api/sync/status')
    apply(data)
  } catch {}
}

const time = computed(() => {
  if (!at.value) return ''
  return new Date(at.value).toLocaleString(locale.value === 'en' ? 'en-GB' : 'ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })
})

const statusText = computed(() => t(`sync_status.${status.value}`, { time: time.value }))

// Events keep the indicator live; the poll covers a lost connection.
let poll: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  load()
  on('sync-status', apply)
  poll = setInterval(load, 60000)
})

onUnmounted(() => {
  off('sync-status', apply)
  if (poll) clearInterval(poll)
})
</script>

<style scoped>
.sync-status {
  display: flex;
  align-items: center;
  cursor: help;
}

.sync-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-faint);
  transition: background 0.3s;
}

.success .sync-dot {
  background: var(--success);
}

.running .sync-dot {
  background: var(--warning);
}

.error .sync-dot {
  background: var(--danger);
}
</style>
