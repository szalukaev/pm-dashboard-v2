<template>
  <div class="table-card" :class="{ flat }">
    <div class="table-scroll">
      <table class="task-table" data-testid="task-table">
        <thead>
          <tr>
            <th
              v-for="col in shownColumns"
              :key="col.key"
              :class="{ sortable: col.sortable, sorted: sortBy === col.key }"
              @click="col.sortable && toggleSort(col.key)"
              :data-testid="'col-' + col.key"
            >
              {{ $t(col.label) }}
              <span v-if="col.sortable && sortBy === col.key" class="sort-icon">{{ sortDir === 'asc' ? '▲' : '▼' }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="task in tasks"
            :key="task.external_id"
            :class="{ overdue: isOverdue(task), 'flash-update': isFlashing(task.external_id) }"
            :data-testid="'task-row-' + task.external_id"
          >
            <!-- The cells follow the columns the user chose and their order -->
            <template v-for="col in shownColumns" :key="col.key">
            <td v-if="col.key === 'external_id'" class="center">
              <a :href="taskLink(task.external_id)" target="_blank" class="issue-link" :data-testid="'task-link-' + task.external_id">
                #{{ task.external_id }}
              </a>
            </td>

            <td v-else-if="col.key === 'subject'" class="subject" @click="$emit('open-task', task.external_id)">{{ task.subject }}</td>

            <td v-else-if="col.key === 'priority_name'" class="center">
              <select
                :disabled="!auth.canWrite"
                class="cell-inputpriority"
                :style="{ color: priorityColor(task.priority_name) }"
                :value="task.priority_name"
                @change="saveInlineEdit(task, 'priority_name', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="p in withCurrent(priorityNames, task.priority_name)" :key="p" :value="p">{{ p }}</option>
              </select>
            </td>

            <td v-else-if="col.key === 'assigned_to'">
              <select
                :disabled="!auth.canWrite"
                class="cell-inputassignee"
                :value="task.assigned_to_name || ''"
                @change="saveInlineEdit(task, 'assigned_to_name', ($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ $t('tasks.table.unassigned') }}</option>
                <option v-for="m in withCurrent(memberNames, task.assigned_to_name)" :key="m" :value="m">{{ m }}</option>
              </select>
            </td>

            <td v-else-if="col.key === 'estimate'" class="center">
              <input
                :disabled="!auth.canWrite"
                class="cell-inputestimate"
                type="text"
                inputmode="decimal"
                :value="task.estimated_hours ?? ''"
                placeholder="—"
                @change="saveInlineEdit(task, 'estimated_hours', ($event.target as HTMLInputElement).value.trim().replace(',', '.'))"
                @keyup.enter="($event.target as HTMLInputElement).blur()"
              />
            </td>

            <td v-else-if="col.key === 'fact'" class="center num">{{ formatHours(task.spent_hours) }}</td>

            <td v-else-if="col.key === 'status_name'">
              <select
                :disabled="!auth.canWrite"
                class="cell-inputstatus-select"
                :style="badgeStyle(statusColor(task.status_name))"
                :value="task.status_name"
                @change="saveInlineEdit(task, 'status_name', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="s in withCurrent(statusNames, task.status_name)" :key="s" :value="s">{{ s }}</option>
              </select>
            </td>

            <td v-else-if="col.key === 'bug_fix_hours'" class="center num" :class="task.bug_fix_hours > 0 ? 'bug' : 'faint'">{{ formatHours(task.bug_fix_hours) }}</td>
            <td v-else-if="col.key === 'bug_fix_pct'" class="center num" :class="task.bug_fix_hours > 0 ? 'bug' : 'faint'">{{ formatBugFixPct(task) }}</td>

            <td v-else-if="col.key === 'start_date'" class="center">
              <input
                :disabled="!auth.canWrite"
                class="cell-inputdate"
                type="date"
                :value="(task.start_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'start_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td v-else-if="col.key === 'due_date'" class="center">
              <input
                :disabled="!auth.canWrite"
                class="cell-inputdate"
                :class="{ overdue: isOverdue(task) }"
                type="date"
                :value="(task.due_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'due_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td v-else-if="col.key === 'category_name'">
              <select
                :disabled="!auth.canWrite"
                class="cell-inputcategory"
                :value="task.category_name || ''"
                @focus="tasksStore.fetchProjectCategories(task.project_id)"
                @mousedown="tasksStore.fetchProjectCategories(task.project_id)"
                @change="saveInlineEdit(task, 'category_name', ($event.target as HTMLSelectElement).value)"
              >
                <option value="">—</option>
                <option v-for="c in projectCategories(task)" :key="c" :value="c">{{ c }}</option>
              </select>
            </td>
            </template>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import type { Task } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'
import { useSwal } from '../../composables/useSwal'
import { useTasksStore } from '../../stores/tasks'
import { isFlashing } from '../../composables/useFlash'
import { TASK_COLUMNS, TASKS_TABLE } from './taskColumns'

const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const { showChange, toast } = useSwal()
const auth = useAuthStore()
const { t } = useI18n()
const { settings } = storeToRefs(settingsStore)

// flat: no own frame (the table sits inside a framed group)
defineProps<{ tasks: Task[]; flat?: boolean }>()

// Team members for the assignee dropdown — loaded once for all tables
// (every accordion group renders its own TaskTable).
onMounted(() => tasksStore.fetchMembers())
const emit = defineEmits(['open-task', 'sort-change', 'inline-edit'])

const fieldLabels: Record<string, string> = {
  status_name: 'tasks.fields.status',
  priority_name: 'tasks.fields.priority',
  assigned_to_name: 'tasks.fields.assignee',
  category_name: 'tasks.fields.category',
  start_date: 'tasks.fields.start_date',
  due_date: 'tasks.fields.deadline',
  estimated_hours: 'tasks.fields.estimate',
}

async function saveInlineEdit(task: Task, field: string, value: string) {
  const oldValue = (task as any)[field]
  const oldNormalized = oldValue === null || oldValue === undefined ? '' : String(oldValue).slice(0, field.endsWith('_date') ? 10 : undefined)
  if (value === oldNormalized) return
  const savedValue = value === '' ? null : value
  // Optimistic update — change field in place without reloading table
  ;(task as any)[field] = field === 'estimated_hours' && savedValue !== null ? Number(savedValue) : savedValue
  try {
    const result = await tasksStore.updateTask(task.external_id, { [field]: savedValue }, true)
    if (result && result.redmine_ok === false) {
      ;(task as any)[field] = oldValue
      toast(t('tasks.messages.redmine_sync_error') + ': ' + (result.redmine_error || ''), 'error')
      return
    }
    showChange(oldNormalized || '—', value || '—', `#${task.external_id} — ${fieldLabels[field] ? t(fieldLabels[field]) : field}`)
  } catch {
    ;(task as any)[field] = oldValue
    toast(t('tasks.messages.save_error'), 'error')
  }
}

const { statuses, priorities, members, projectCategories: projectCategoriesMap, filters } = storeToRefs(tasksStore)

// The sorting is one for every table of tasks and lives with the filters:
// a table is rebuilt when its data is reloaded, and a state of its own
// would be lost between two clicks.
const sortBy = computed(() => filters.value.sort_by)
const sortDir = computed(() => filters.value.sort_dir)

const statusNames = computed(() => statuses.value.map(s => s.name))
const priorityNames = computed(() => priorities.value.map(p => p.name))
// Closed is whatever the status groups in the settings say, not a status name
const closedStatusIds = computed(() => new Set(statuses.value.filter(s => s.group === 'closed').map(s => s.id)))

const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  if (selectedTeam.length === 0) return members.value.map(m => m.name).sort()
  return members.value.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

// The current value must stay selectable even if it is not in the list.
function withCurrent(list: string[], current?: string | null): string[] {
  return current && !list.includes(current) ? [current, ...list] : list
}

// The columns the user chose to see, in their order
const shownColumns = computed(() =>
  settingsStore.tableColumns(TASKS_TABLE, TASK_COLUMNS.map(c => c.key)).map(key => TASK_COLUMNS.find(c => c.key === key)!)
)

function toggleSort(key: string) {
  const dir = sortBy.value === key && sortDir.value === 'asc' ? 'desc' : 'asc'
  emit('sort-change', { sortBy: key, sortDir: dir })
}

function isOverdue(task: Task): boolean {
  if (!task.due_date) return false
  return task.due_date.slice(0, 10) < new Date().toISOString().slice(0, 10) && !closedStatusIds.value.has(task.status_id)
}

// Colors as in the previous version of the dashboard
const priorityColors: Record<string, string> = {
  immediate: '#ff4444',
  urgent: '#ff8800',
  high: '#ffaa00',
  normal: '#4488ff',
  low: '#888888',
}

function priorityColor(name: string): string {
  return priorityColors[(name || '').toLowerCase()] || '#888888'
}

const statusColors: Record<string, string> = {
  'new': '#5e6ad2',
  'in progress': '#ffd93d',
  'review': '#a855f7',
  'feedback': '#f97316',
  'bugs': '#ff6b6b',
  'testing': '#06b6d4',
  'closed': '#6bcb77',
  'tested': '#6bcb77',
  'resolved': '#6bcb77',
}

function statusColor(name: string): string {
  const n = (name || '').toLowerCase()
  if (statusColors[n]) return statusColors[n]
  if (n.includes('test')) return statusColors.testing
  if (n.includes('bug')) return statusColors.bugs
  return '#888888'
}

function badgeStyle(color: string) {
  return { color, background: color + '22' }
}

function taskLink(id: number): string {
  const base = settings.value.data_source_url || settings.value.redmine_url || ''
  return base ? base.replace(/\/$/, '') + '/issues/' + id : '#'
}

function projectCategories(task: Task): string[] {
  // All categories of the task's project; the current value stays selectable
  // even while the list is loading or if it was removed in Redmine.
  const projectCats = new Set<string>(projectCategoriesMap.value[task.project_id] || [])
  if (task.category_name) projectCats.add(task.category_name)
  return [...projectCats].sort()
}

// Share of bug fix hours in the fact, computed on the fly.
function formatBugFixPct(task: Task): string {
  if (!task.spent_hours || !task.bug_fix_hours) return '—'
  return (task.bug_fix_hours / task.spent_hours * 100).toFixed(1) + '%'
}

function formatHours(h: number): string {
  if (!h) return '—'
  return Number(h).toFixed(2)
}
</script>

<style scoped>
.table-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
  margin-bottom: 24px;
}

.table-card.flat {
  border: none;
  border-top: 1px solid var(--hairline);
  border-radius: 0;
  margin-bottom: 0;
}

.table-scroll {
  overflow-x: auto;
}

.task-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  table-layout: auto;
}

th {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky-header);
  padding: 10px 14px;
  background: var(--surface-2);
  border-bottom: 1px solid var(--hairline);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
  text-align: center;
  text-transform: uppercase;
  white-space: nowrap;
  user-select: none;
}

th.sortable {
  cursor: pointer;
}

th.sortable:hover,
th.sorted {
  color: var(--text-bright);
}

.sort-icon {
  margin-left: 4px;
  font-size: 9px;
}

td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
  vertical-align: middle;
}

tbody tr:last-child td {
  border-bottom: none;
}

tbody tr:hover td {
  background: var(--bg-hover);
}

tr.overdue td:first-child {
  box-shadow: inset 3px 0 0 var(--critical);
}

.center {
  text-align: center;
}

.num {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.faint {
  color: var(--text-faint);
}

.bug {
  color: var(--danger);
}

.subject {
  min-width: 180px;
  max-width: 350px;
  color: var(--accent);
  white-space: normal;
  word-break: break-word;
  cursor: pointer;
}

.subject:hover {
  text-decoration: underline;
}

.issue-link {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}

.issue-link:hover {
  text-decoration: underline;
}

/* Inline editors look like plain text until hovered */
.cell-input {
  padding: 4px 6px;
  font-size: 12px;
  font-family: inherit;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--text-bright);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.cell-input:hover:not(:disabled) {
  border-color: var(--hairline);
}

/* A read-only user sees the values as plain text */
.cell-input:disabled {
  cursor: default;
  opacity: 1;
}

.cell-input:focus {
  outline: none;
  border-color: var(--accent);
  background: var(--surface-2);
}

.cell-input option {
  background: var(--surface-1);
  color: var(--text);
}

.priority {
  width: 100px;
  font-size: 11px;
  font-weight: 600;
}

.assignee {
  width: 160px;
}

.estimate {
  width: 60px;
  text-align: center;
  cursor: text;
}

.estimate::placeholder {
  color: var(--text-faint);
}

.status-select {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
}

.date {
  width: 130px;
}

.date.overdue {
  color: var(--critical);
}

.category {
  width: 140px;
  font-size: 11px;
  color: var(--text-muted);
}

@media (max-width: 768px) {
  th,
  td {
    padding: 8px;
    font-size: 11px;
  }
}
</style>
