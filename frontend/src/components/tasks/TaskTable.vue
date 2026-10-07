<template>
  <div class="task-table-wrapper">
    <table class="task-table" data-testid="task-table">
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            class="th-cell"
            :class="{ sortable: col.sortable, sorted: sortBy === col.key, 'center-cell': ['estimate','fact','status_name','bug_fix_hours','bug_fix_pct','start_date','due_date'].includes(col.key) }"
            @click="col.sortable && toggleSort(col.key)"
            :data-testid="'col-' + col.key"
          >
            <span>{{ $t(col.label) }}</span>
            <span v-if="col.sortable && sortBy === col.key" class="sort-icon">
              {{ sortDir === 'asc' ? '↑' : '↓' }}
            </span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="task in tasks"
          :key="task.external_id"
          class="task-row"
          :class="{ 'overdue-row': isOverdue(task) }"
          :data-testid="'task-row-' + task.external_id"
        >
          <td class="td-cell name-cell">
            <a
              :href="taskLink(task.external_id)"
              target="_blank"
              class="task-link"
              @click.stop
              :data-testid="'task-link-' + task.external_id"
            >
              {{ task.external_id }}
            </a>
            <span class="task-subject" :title="task.subject" @click.stop="$emit('open-task', task.external_id)">{{ task.subject }}</span>
          </td>
          <td class="td-cell priority-cell" @dblclick.stop="startInlineEdit(task, 'priority_name')">
            <template v-if="isEditing(task.external_id, 'priority_name')">
              <select
                :value="task.priority_name"
                @change="saveInlineEdit(task, 'priority_name', ($event.target as HTMLSelectElement).value)"
                @blur="cancelInlineEdit"
                class="inline-select"
                autofocus
              >
                <option v-for="p in priorityNames" :key="p" :value="p">{{ p }}</option>
              </select>
            </template>
            <template v-else>
              <span class="priority-dot" :style="{ background: priorityColor(task.priority_id) }"></span>
              {{ task.priority_name }}
            </template>
          </td>
          <td class="td-cell" @dblclick.stop="startInlineEdit(task, 'assigned_to_name')">
            <template v-if="isEditing(task.external_id, 'assigned_to_name')">
              <select
                :value="task.assigned_to_name"
                @change="saveInlineEdit(task, 'assigned_to_name', ($event.target as HTMLSelectElement).value)"
                @blur="cancelInlineEdit"
                class="inline-select"
                autofocus
              >
                <option value="">Без ответственного</option>
                <option v-for="m in memberNames" :key="m" :value="m">{{ m }}</option>
              </select>
            </template>
            <template v-else>{{ task.assigned_to_name || 'Без ответственного' }}</template>
          </td>
          <td class="td-cell num-cell center-cell" @dblclick.stop="startInlineEdit(task, 'estimated_hours')">
            <template v-if="isEditing(task.external_id, 'estimated_hours')">
              <input
                type="number"
                :value="task.estimated_hours"
                @blur="saveInlineEdit(task, 'estimated_hours', ($event.target as HTMLInputElement).value)"
                @keyup.enter="saveInlineEdit(task, 'estimated_hours', ($event.target as HTMLInputElement).value)"
                @keyup.escape="cancelInlineEdit"
                class="inline-input num-input"
                autofocus
              />
            </template>
            <template v-else>{{ formatEstimate(task.estimated_hours) }}</template>
          </td>
          <td class="td-cell num-cell center-cell">{{ formatHours(task.spent_hours) }}</td>
          <td class="td-cell center-cell" @dblclick.stop="startInlineEdit(task, 'status_name')">
            <template v-if="isEditing(task.external_id, 'status_name')">
              <select
                :value="task.status_name"
                @change="saveInlineEdit(task, 'status_name', ($event.target as HTMLSelectElement).value)"
                @blur="cancelInlineEdit"
                class="inline-select"
                autofocus
              >
                <option v-for="s in statusNames" :key="s" :value="s">{{ s }}</option>
              </select>
            </template>
            <template v-else>
              <span class="status-badge" :class="statusClass(task.status_name)">{{ task.status_name }}</span>
            </template>
          </td>
          <td class="td-cell num-cell center-cell">{{ formatHours(task.bug_fix_hours) }}</td>
          <td class="td-cell num-cell center-cell">{{ formatBugFixPct(task) }}</td>
          <td class="td-cell date-cell center-cell" @dblclick.stop="startInlineEdit(task, 'start_date')">
            <template v-if="isEditing(task.external_id, 'start_date')">
              <input
                type="date"
                :value="task.start_date"
                @blur="saveInlineEdit(task, 'start_date', ($event.target as HTMLInputElement).value)"
                @keyup.escape="cancelInlineEdit"
                class="inline-input"
                autofocus
              />
            </template>
            <template v-else>{{ formatDate(task.start_date) }}</template>
          </td>
          <td class="td-cell date-cell center-cell" :class="{ 'overdue-date': isOverdue(task) }" @dblclick.stop="startInlineEdit(task, 'due_date')">
            <template v-if="isEditing(task.external_id, 'due_date')">
              <input
                type="date"
                :value="task.due_date"
                @blur="saveInlineEdit(task, 'due_date', ($event.target as HTMLInputElement).value)"
                @keyup.escape="cancelInlineEdit"
                class="inline-input"
                autofocus
              />
            </template>
            <template v-else>{{ formatDate(task.due_date) }}</template>
          </td>
          <td class="td-cell" @dblclick.stop="startInlineEdit(task, 'category_name')">
            <template v-if="isEditing(task.external_id, 'category_name')">
              <select
                :value="task.category_name"
                @change="saveInlineEdit(task, 'category_name', ($event.target as HTMLSelectElement).value)"
                @blur="cancelInlineEdit"
                class="inline-select"
                autofocus
              >
                <option value="">—</option>
                <option v-for="c in projectCategories(task)" :key="c" :value="c">{{ c }}</option>
              </select>
            </template>
            <template v-else>{{ task.category_name || '—' }}</template>
          </td>
        </tr>
      </tbody>
    </table>
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

defineProps<{ tasks: Task[] }>()

// Team members for the assignee dropdown — loaded once for all tables
// (every accordion group renders its own TaskTable).
onMounted(() => tasksStore.fetchMembers())
const emit = defineEmits(['open-task', 'sort-change', 'inline-edit'])

// Inline edit state
const editingCell = ref<{ taskId: number; field: string } | null>(null)

function startInlineEdit(task: Task, field: string) {
  if (field === 'category_name') tasksStore.fetchProjectCategories(task.project_id)
  editingCell.value = { taskId: task.external_id, field }
}

function isEditing(taskId: number, field: string): boolean {
  return editingCell.value?.taskId === taskId && editingCell.value?.field === field
}

function cancelInlineEdit() {
  editingCell.value = null
}

async function saveInlineEdit(task: Task, field: string, value: string) {
  editingCell.value = null
  const oldValue = (task as any)[field]
  if (value === oldValue || (value === '' && oldValue === null)) return
  const oldDisplay = String(oldValue || '—')
  const newDisplay = String(value || '—')
  // Optimistic update — change field in place without reloading table
  const savedValue = value || null
  ;(task as any)[field] = savedValue
  try {
    const result = await tasksStore.updateTask(task.external_id, { [field]: savedValue }, true)
    const fieldLabels: Record<string, string> = {
      status_name: 'Статус',
      priority_name: 'Приоритет',
      assigned_to_name: 'Исполнитель',
      category_name: 'Категория',
      start_date: 'Дата начала',
      due_date: 'Дедлайн',
      estimated_hours: 'Оценка',
    }
    showChange(oldDisplay, newDisplay, `#${task.external_id} — ${fieldLabels[field] || field}`)
    if (result && result.redmine_ok === false) {
      // Revert on Redmine error
      ;(task as any)[field] = oldValue
      toast('Ошибка синхронизации с Redmine: ' + (result.redmine_error || ''), 'error')
    }
  } catch {
    // Revert on error
    ;(task as any)[field] = oldValue
    toast('Ошибка сохранения', 'error')
  }
}

const sortBy = ref('')
const sortDir = ref('asc')

const { statuses, priorities, members, projectCategories: projectCategoriesMap } = storeToRefs(tasksStore)

const statusNames = computed(() => {
  return statuses.value.map((s: any) => s.name) // already sorted by external_id ASC from API
})

const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  if (selectedTeam.length === 0) return members.value.map(m => m.name).sort()
  return members.value.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

const columns = [
  { key: 'subject', label: 'tasks.table.name', sortable: true },
  { key: 'priority_name', label: 'tasks.table.priority', sortable: true },
  { key: 'assigned_to', label: 'tasks.table.assignee', sortable: true },
  { key: 'estimate', label: 'tasks.table.estimate', sortable: true },
  { key: 'fact', label: 'tasks.table.fact', sortable: true },
  { key: 'status_name', label: 'tasks.table.status', sortable: true },
  { key: 'bug_fix_hours', label: 'tasks.table.bug_fix', sortable: true },
  { key: 'bug_fix_pct', label: 'tasks.table.bug_fix_pct', sortable: true },
  { key: 'start_date', label: 'tasks.table.start_date', sortable: true },
  { key: 'due_date', label: 'tasks.table.end_date', sortable: true },
  { key: 'category_name', label: 'tasks.table.category', sortable: true },
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
  return task.due_date < new Date().toISOString().slice(0, 10) &&
    !['closed', 'rejected', 'resolved', 'tested'].includes(task.status_name.toLowerCase())
}

function priorityColor(id: number): string {
  // Highest priority (largest id) = bright red, lowest = gray
  if (id >= 6) return '#ff2222' // Immediate — bright red
  if (id >= 5) return '#ff4444' // Urgent — red
  if (id >= 4) return '#ff8800' // High — orange
  if (id >= 3) return '#ffaa00' // Normal — yellow
  return '#888888' // Low/lowest — gray
}

const priorityNames = computed(() => {
  return priorities.value.map((p: any) => p.name) // already sorted DESC from API
})

function statusClass(name: string): string {
  const n = name.toLowerCase()
  if (['closed', 'rejected', 'resolved', 'tested'].includes(n)) return 'status-closed'
  if (n.includes('test')) return 'status-testing'
  if (n.includes('bug')) return 'status-bug'
  return 'status-open'
}

function taskLink(id: number): string {
  const base = settings.value.data_source_url || settings.value.redmine_url || ''
  if (base) {
    return base.replace(/\/$/, '') + '/issues/' + id
  }
  return '#'
}

function formatDate(d?: string | null): string {
  if (!d) return '—'
  return d.slice(0, 10)
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

function formatEstimate(h: number | null): string {
  if (h === null || h === undefined) return '—'
  return h.toFixed(1)
}

function formatHours(h: number): string {
  if (!h) return '—'
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  return `${hours}:${mins.toString().padStart(2, '0')} ч.`
}
</script>

<style scoped>
.task-table-wrapper {
  overflow-x: auto;
}

.task-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.th-cell {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
  position: sticky;
  top: 0;
  z-index: var(--z-sticky-header);
  white-space: nowrap;
  user-select: none;
}

.th-cell.sortable {
  cursor: pointer;
}

.th-cell.sortable:hover {
  color: var(--text-bright);
}

.th-cell.sorted {
  color: var(--accent);
}

.sort-icon {
  margin-left: 4px;
  font-size: 10px;
}

.td-cell {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
  white-space: nowrap;
}

.center-cell {
  text-align: center;
}

.task-row {
  transition: background 0.15s;
}

.task-row:hover {
  background: var(--bg-hover);
}

.overdue-row {
  border-left: 3px solid var(--critical);
}

.name-cell {
  max-width: 300px;
  white-space: nowrap;
}

.task-link {
  color: var(--accent);
  font-weight: 500;
  font-size: 12px;
  text-decoration: none;
  display: inline-block;
  vertical-align: middle;
  margin-right: 8px;
}

.task-link:hover {
  text-decoration: underline;
}

.task-subject {
  display: inline-block;
  vertical-align: middle;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
  cursor: pointer;
}

.task-subject:hover {
  color: var(--accent);
  text-decoration: underline;
}

.priority-cell {
  white-space: nowrap;
  text-align: left;
}

.priority-dot {
  display: inline-block;
  vertical-align: middle;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}

.num-cell {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.date-cell {
  font-size: 12px;
  color: var(--text-faint);
}

.overdue-date {
  color: var(--critical);
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
}

.status-open {
  background: var(--accent-bg);
  color: var(--accent);
}

.status-testing {
  background: var(--teal)22;
  color: var(--teal);
}

.status-closed {
  background: var(--success)22;
  color: var(--success);
}

.status-bug {
  background: var(--danger)22;
  color: var(--danger);
}

.inline-select,
.inline-input {
  padding: 2px 6px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text);
  width: 100%;
  min-width: 60px;
}

.inline-input.num-input {
  width: 80px;
  text-align: right;
}

.inline-select:focus,
.inline-input:focus {
  outline: none;
  box-shadow: var(--focus-ring);
}

@media (max-width: 768px) {
  .th-cell,
  .td-cell {
    padding: 8px;
    font-size: 11px;
  }
}
</style>
