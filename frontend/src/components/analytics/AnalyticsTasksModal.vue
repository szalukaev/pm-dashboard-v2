<template>
  <AppModal :model-value="query !== null" :title="query ? query.title : ''" width="1100px" expandable @close="$emit('close')">
    <div v-if="loading && tasks.length === 0" class="list-hint">{{ $t('common.loading') }}</div>
    <div v-else-if="failed" class="list-hint">{{ $t('common.error') }}</div>
    <div v-else-if="tasks.length === 0" class="list-hint">{{ $t('common.empty') }}</div>

    <template v-else>
      <div class="list-wrapper">
        <table class="list-table" data-testid="analytics-tasks">
          <thead>
            <tr>
              <th>{{ $t('tasks.table.number') }}</th>
              <th>{{ $t('tasks.table.name') }}</th>
              <th>{{ $t('tasks.table.status') }}</th>
              <th>{{ $t('tasks.table.priority') }}</th>
              <th class="num">{{ $t('tasks.table.estimate') }}</th>
              <th class="num">{{ $t('tasks.table.fact') }}</th>
              <th class="num">{{ $t('tasks.table.bug_fix') }}</th>
              <th v-if="showAssignee">{{ $t('tasks.table.responsible') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="group in groups" :key="group.key">
              <!-- The tasks go by project; a project may continue on the next page -->
              <tr class="group-row">
                <td :colspan="showAssignee ? 8 : 7">{{ group.name || '—' }}</td>
              </tr>
              <tr v-for="task in group.tasks" :key="task.external_id" :data-testid="'analytics-task-' + task.external_id">
                <td class="id-cell">
                  <a :href="taskLink(task.external_id)" target="_blank" class="task-id">#{{ task.external_id }}</a>
                </td>
                <td class="subject-cell">
                  <button type="button" class="task-subject" :title="task.subject" @click="$emit('open-task', task.external_id)">
                    {{ task.subject }}
                  </button>
                </td>
                <td :style="{ color: statusColor(task.status_name) }">{{ task.status_name }}</td>
                <td>{{ task.priority_name }}</td>
                <td class="num">{{ hours(task.estimated_hours) }}</td>
                <td class="num">{{ hours(task.spent_hours) }}</td>
                <td class="num">{{ hours(task.bug_fix_hours) }}</td>
                <td v-if="showAssignee">{{ task.assigned_to_name || '—' }}</td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <AppPagination :total="total" :limit="PAGE" :offset="offset" :sizes="[PAGE]" @update:offset="load" />
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import axios from 'axios'
import { useSettingsStore } from '../../stores/settings'
import { useAnalyticsStore, type TaskListQuery } from '../../stores/analytics'
import { statusColor } from '../../utils/statusColor'
import AppModal from '../ui/AppModal.vue'
import AppPagination from '../ui/AppPagination.vue'

interface ListTask {
  external_id: number
  project_id: number
  project_name: string
  subject: string
  status_name: string
  priority_name: string
  assigned_to_name: string
  estimated_hours: number | null
  spent_hours: number
  bug_fix_hours: number
}

const props = defineProps<{ query: TaskListQuery | null }>()
defineEmits<{
  (e: 'close'): void
  (e: 'open-task', id: number): void
}>()

const PAGE = 25

const settingsStore = useSettingsStore()
const tasks = ref<ListTask[]>([])
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const failed = ref(false)

// One employee's tasks need no column with their name
const showAssignee = computed(() => props.query?.assignee === undefined)

const groups = computed(() => {
  const result: { key: string; name: string; tasks: ListTask[] }[] = []
  for (const task of tasks.value) {
    const key = task.project_id + ' ' + task.project_name
    const last = result[result.length - 1]
    if (last && last.key === key) last.tasks.push(task)
    else result.push({ key, name: task.project_name, tasks: [task] })
  }
  return result
})

async function load(from: number) {
  const query = props.query
  if (!query) return
  loading.value = true
  failed.value = false
  try {
    // Тот же период, что у цифры, по которой открыт список
    const params: Record<string, string | number> = { ...useAnalyticsStore().periodParams(), metric: query.metric, offset: from }
    if (query.assignee !== undefined) params.assignee = query.assignee
    const { data } = await axios.get('/api/analytics/tasks', { params })
    // The answer to a figure that is not shown any more is of no use
    if (props.query !== query) return
    tasks.value = data.tasks || []
    total.value = data.total || 0
    offset.value = data.offset || 0
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

watch(() => props.query, query => {
  tasks.value = []
  total.value = 0
  offset.value = 0
  if (query) load(0)
})

function hours(value: number | null): string {
  return value === null || value === undefined || value === 0 ? '—' : String(Math.round(value * 100) / 100)
}

function taskLink(externalId: number): string {
  const base = settingsStore.settings.data_source_url || ''
  return base ? base.replace(/\/$/, '') + '/issues/' + externalId : '#'
}
</script>

<style scoped>
.list-hint {
  padding: 32px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}

.list-wrapper {
  overflow-x: auto;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  margin-bottom: 12px;
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
  white-space: nowrap;
}

.list-table .num {
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.group-row td {
  background: var(--surface-2);
  font-weight: 600;
  color: var(--text-bright);
}

.task-id {
  color: var(--accent);
  text-decoration: none;
}

.task-id:hover {
  text-decoration: underline;
}

.subject-cell {
  width: 100%;
  max-width: 0;
}

.task-subject {
  display: block;
  width: 100%;
  padding: 0;
  border: none;
  background: none;
  font-size: 13px;
  color: var(--text);
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.task-subject:hover {
  color: var(--accent);
}
</style>
