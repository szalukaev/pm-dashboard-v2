<template>
  <div class="table-card" :class="{ flat }">
    <div class="table-scroll">
      <table class="task-table" data-testid="task-table">
        <thead>
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              :class="{ sortable: col.sortable, sorted: sortBy === col.key, center: col.center }"
              @click="col.sortable && toggleSort(col.key)"
              :data-testid="'col-' + col.key"
            >
              {{ col.label }}
              <span v-if="col.sortable && sortBy === col.key" class="sort-icon">{{ sortDir === 'asc' ? '▲' : '▼' }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="task in tasks"
            :key="task.external_id"
            :class="{ overdue: isOverdue(task) }"
            :data-testid="'task-row-' + task.external_id"
          >
            <td class="subject" @click="$emit('open-task', task.external_id)">{{ task.subject }}</td>

            <td class="center">
              <a :href="taskLink(task.external_id)" target="_blank" class="issue-link" :data-testid="'task-link-' + task.external_id">
                #{{ task.external_id }}
              </a>
            </td>

            <td class="center">
              <select
                class="cell-input priority"
                :style="{ color: priorityColor(task.priority_name) }"
                :value="task.priority_name"
                @change="saveInlineEdit(task, 'priority_name', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="p in withCurrent(priorityNames, task.priority_name)" :key="p" :value="p">{{ p }}</option>
              </select>
            </td>

            <td>
              <select
                class="cell-input assignee"
                :value="task.assigned_to_name || ''"
                @change="saveInlineEdit(task, 'assigned_to_name', ($event.target as HTMLSelectElement).value)"
              >
                <option value="">Не назначен</option>
                <option v-for="m in withCurrent(memberNames, task.assigned_to_name)" :key="m" :value="m">{{ m }}</option>
              </select>
            </td>

            <td class="center">
              <input
                class="cell-input estimate"
                type="text"
                inputmode="decimal"
                :value="task.estimated_hours ?? ''"
                placeholder="—"
                @change="saveInlineEdit(task, 'estimated_hours', ($event.target as HTMLInputElement).value.trim().replace(',', '.'))"
                @keyup.enter="($event.target as HTMLInputElement).blur()"
              />
            </td>

            <td class="center num">{{ formatHours(task.spent_hours) }}</td>

            <td>
              <select
                class="cell-input status-select"
                :style="badgeStyle(statusColor(task.status_name))"
                :value="task.status_name"
                @change="saveInlineEdit(task, 'status_name', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="s in withCurrent(statusNames, task.status_name)" :key="s" :value="s">{{ s }}</option>
              </select>
            </td>

            <td class="center num" :class="task.bug_fix_hours > 0 ? 'bug' : 'faint'">{{ formatHours(task.bug_fix_hours) }}</td>
            <td class="center num" :class="task.bug_fix_hours > 0 ? 'bug' : 'faint'">{{ formatBugFixPct(task) }}</td>

            <td class="center">
              <input
                class="cell-input date"
                type="date"
                :value="(task.start_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'start_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td class="center">
              <input
                class="cell-input date"
                :class="{ overdue: isOverdue(task) }"
                type="date"
                :value="(task.due_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'due_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td>
              <select
                class="cell-input category"
                :value="task.category_name || ''"
                @focus="tasksStore.fetchProjectCategories(task.project_id)"
                @mousedown="tasksStore.fetchProjectCategories(task.project_id)"
                @change="saveInlineEdit(task, 'category_name', ($event.target as HTMLSelectElement).value)"
              >
                <option value="">—</option>
                <option v-for="c in projectCategories(task)" :key="c" :value="c">{{ c }}</option>
              </select>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import type { Task } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'
import { useSwal } from '../../composables/useSwal'
import { useTasksStore } from '../../stores/tasks'

const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const { showChange, toast } = useSwal()
const { settings } = storeToRefs(settingsStore)

// flat: no own frame (the table sits inside a framed group)
defineProps<{ tasks: Task[]; flat?: boolean }>()

// Team members for the assignee dropdown — loaded once for all tables
// (every accordion group renders its own TaskTable).
onMounted(() => tasksStore.fetchMembers())
const emit = defineEmits(['open-task', 'sort-change', 'inline-edit'])

const fieldLabels: Record<string, string> = {
  status_name: 'Статус',
  priority_name: 'Приоритет',
  assigned_to_name: 'Исполнитель',
  category_name: 'Категория',
  start_date: 'Дата начала',
  due_date: 'Дедлайн',
  estimated_hours: 'Оценка',
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
      toast('Ошибка синхронизации с Redmine: ' + (result.redmine_error || ''), 'error')
      return
    }
    showChange(oldNormalized || '—', value || '—', `#${task.external_id} — ${fieldLabels[field] || field}`)
  } catch {
    ;(task as any)[field] = oldValue
    toast('Ошибка сохранения', 'error')
  }
}

const sortBy = ref('')
const sortDir = ref('asc')

const { statuses, priorities, members, projectCategories: projectCategoriesMap } = storeToRefs(tasksStore)

const statusNames = computed(() => statuses.value.map(s => s.name))
const priorityNames = computed(() => priorities.value.map(p => p.name))

const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  if (selectedTeam.length === 0) return members.value.map(m => m.name).sort()
  return members.value.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

// The current value must stay selectable even if it is not in the list.
function withCurrent(list: string[], current?: string | null): string[] {
  return current && !list.includes(current) ? [current, ...list] : list
}

const columns = [
  { key: 'subject', label: 'Наименование', sortable: true },
  { key: 'external_id', label: 'Redmine', sortable: true, center: true },
  { key: 'priority_name', label: 'Приоритет', sortable: true, center: true },
  { key: 'assigned_to', label: 'Ответственный', sortable: true },
  { key: 'estimate', label: 'Оценка, ч', sortable: true, center: true },
  { key: 'fact', label: 'Факт, ч', sortable: true, center: true },
  { key: 'status_name', label: 'Статус', sortable: true },
  { key: 'bug_fix_hours', label: 'Bug fix, ч', sortable: true, center: true },
  { key: 'bug_fix_pct', label: 'Bug fix, %', sortable: true, center: true },
  { key: 'start_date', label: 'Дата старта', sortable: true, center: true },
  { key: 'due_date', label: 'Дата окончания', sortable: true, center: true },
  { key: 'category_name', label: 'Категория', sortable: true },
]

function toggleSort(key: string) {
  if (sortBy.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = key
    sortDir.value = 'asc'
  }
  emit('sort-change', { sortBy: sortBy.value, sortDir: sortDir.value })
}

function isOverdue(task: Task): boolean {
  if (!task.due_date) return false
  return task.due_date.slice(0, 10) < new Date().toISOString().slice(0, 10) &&
    !['closed', 'rejected', 'resolved', 'tested'].includes(task.status_name.toLowerCase())
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
  text-align: left;
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

.cell-input:hover {
  border-color: var(--hairline);
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
  color-scheme: dark;
}

[data-mode="light"] .date {
  color-scheme: light;
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
