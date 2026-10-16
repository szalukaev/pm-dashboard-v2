<template>
  <AppModal :model-value="query !== null" :title="title" width="900px" :page="isMobile" @update:model-value="!$event && close()">
    <div v-if="loading" class="list-hint"><AppSpinner :size="24" /></div>
    <div v-else-if="failed" class="list-hint">{{ $t('common.error') }}</div>
    <div v-else-if="entries.length === 0" class="list-hint">{{ $t('common.empty') }}</div>

    <template v-else>
      <!-- Телефон: карточками вместо широкой таблицы -->
      <div v-if="isMobile" class="te-cards" data-testid="time-entries">
        <div v-for="e in entries" :key="e.id" class="te-card">
          <div class="te-card-top">
            <span class="te-date">{{ formatDate(e.spent_on) }}</span>
            <b class="te-hours" :class="{ bug: isBug(e) }">{{ formatHours(e.hours) }}</b>
          </div>
          <div class="te-user">{{ e.user }}</div>
          <div class="te-activity" :class="{ bug: isBug(e) }">{{ e.activity }}</div>
          <div v-if="e.comments" class="te-comment">{{ e.comments }}</div>
        </div>
      </div>
      <div v-else class="list-wrapper">
        <table class="list-table" data-testid="time-entries">
          <thead>
            <tr>
              <th>{{ $t('tasks.time_entries.date') }}</th>
              <th>{{ $t('tasks.time_entries.user') }}</th>
              <th>{{ $t('tasks.time_entries.activity') }}</th>
              <th>{{ $t('tasks.time_entries.comment') }}</th>
              <th class="num">{{ $t('tasks.time_entries.hours') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in entries" :key="e.id">
              <td class="nowrap">{{ formatDate(e.spent_on) }}</td>
              <td class="nowrap">{{ e.user }}</td>
              <td class="nowrap" :class="{ bug: isBug(e) }">{{ e.activity }}</td>
              <td class="comment-cell">{{ e.comments || '—' }}</td>
              <td class="num" :class="{ bug: isBug(e) }">{{ formatHours(e.hours) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="te-total" data-testid="time-entries-total">
        {{ $t('tasks.time_entries.total') }} <b>{{ formatHours(total) }}</b>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import AppModal from '../ui/AppModal.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { onSwipeBack } from '../../composables/useSwipeBack'
import { formatDate as formatCalendarDate } from '../../utils/format'

// Какие трудозатраты показать: все по задаче, только исполнителя, только на ошибки
export interface TimeEntriesQuery {
  taskId: number
  assignee: boolean
  bugs: boolean
}

interface Entry {
  id: number
  spent_on: string
  user: string
  activity: string
  comments: string
  hours: number
}

const props = defineProps<{ query: TimeEntriesQuery | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const i18n = useI18n()
const { isMobile } = useMobileTasks()

const entries = ref<Entry[]>([])
const loading = ref(false)
const failed = ref(false)

const title = computed(() => {
  const q = props.query
  if (!q) return ''
  const key = q.assignee ? (q.bugs ? 'title_assignee_bugs' : 'title_assignee') : (q.bugs ? 'title_bugs' : 'title_all')
  return i18n.t('tasks.time_entries.' + key)
})

const total = computed(() => entries.value.reduce((sum, e) => sum + e.hours, 0))

// Как в карточке задачи: баг-часы — деятельность «Fixing bugs»
function isBug(e: Entry): boolean {
  return e.activity.trim().toLowerCase() === 'fixing bugs'
}

async function load(query: TimeEntriesQuery) {
  loading.value = true
  failed.value = false
  entries.value = []
  try {
    const params: Record<string, string> = {}
    if (query.assignee) params.assignee = '1'
    if (query.bugs) params.bugs = '1'
    const { data } = await axios.get(`/api/tasks/${query.taskId}/time-entries`, { params })
    // Ответ по уже закрытому или сменённому списку не нужен
    if (props.query !== query) return
    entries.value = data.entries || []
  } catch {
    if (props.query === query) failed.value = true
  } finally {
    if (props.query === query) loading.value = false
  }
}

// Жест «назад» на телефоне закрывает сначала этот список, потом карточку
let releaseSwipe: (() => void) | null = null
watch(() => props.query, query => {
  releaseSwipe?.()
  releaseSwipe = null
  if (!query) return
  releaseSwipe = onSwipeBack(() => { close(); return true })
  load(query)
})
onBeforeUnmount(() => releaseSwipe?.())

function close() {
  emit('close')
}

function formatDate(day: string): string {
  return day ? formatCalendarDate(day) : '—'
}

function formatHours(h: number): string {
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  return `${hours}:${mins.toString().padStart(2, '0')} ${i18n.t('tasks.units.hours')}`
}
</script>

<style scoped>
.list-hint {
  display: flex;
  justify-content: center;
  padding: 32px;
  font-size: 13px;
  color: var(--text-muted);
}

.list-wrapper {
  overflow-x: auto;
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.list-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.list-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
  white-space: nowrap;
}

.list-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
  vertical-align: top;
}

.list-table tr:last-child td {
  border-bottom: none;
}

.list-table .num {
  text-align: right;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.nowrap {
  white-space: nowrap;
}

.comment-cell {
  min-width: 200px;
  overflow-wrap: anywhere;
}

.bug {
  color: var(--danger) !important;
}

.te-total {
  margin-top: 12px;
  text-align: right;
  font-size: 13px;
  color: var(--text-muted);
}

.te-total b {
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
}

/* Телефон */
.te-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.te-card {
  padding: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  min-width: 0;
}

.te-card-top {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}

.te-date {
  font-size: 12px;
  color: var(--text-muted);
}

.te-hours {
  font-size: 16px;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
}

.te-user {
  margin-top: 2px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.te-activity {
  font-size: 12px;
  color: var(--text-muted);
}

.te-comment {
  margin-top: 6px;
  font-size: 13px;
  color: var(--text-dim);
  overflow-wrap: anywhere;
}
</style>
