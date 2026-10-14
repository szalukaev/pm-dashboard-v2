<template>
  <!-- Телефон: вместо таблицы карточки или лента сроков -->
  <TaskMobileList v-if="isMobile" :tasks="tasks" @open-task="$emit('open-task', $event)" />
  <div v-else class="table-card" :class="{ flat }">
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
            <!-- Ячейки идут по колонкам, выбранным пользователем, в их порядке -->
            <template v-for="col in shownColumns" :key="col.key">
            <td v-if="col.key === 'external_id'" class="center">
              <a :href="taskLink(task.external_id)" target="_blank" class="issue-link" :data-testid="'task-link-' + task.external_id">
                #{{ task.external_id }}
              </a>
            </td>

            <td v-else-if="col.key === 'project_name'" class="project">{{ task.project_name }}</td>

            <td v-else-if="col.key === 'subject'" class="subject" @click="$emit('open-task', task.external_id)">{{ task.subject }}</td>

            <td v-else-if="col.key === 'priority_name'" class="center">
              <select
                :disabled="!auth.canWrite"
                class="cell-input priority"
                :style="{ color: priorityColor({ name: task.priority_name }) }"
                :value="task.priority_name"
                @change="saveInlineEdit(task, 'priority_name', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="p in withCurrent(priorityNames, task.priority_name)" :key="p" :value="p">{{ p }}</option>
              </select>
            </td>

            <td v-else-if="col.key === 'assigned_to'">
              <select
                :disabled="!auth.canWrite"
                class="cell-input assignee"
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
                class="cell-input estimate"
                type="text"
                inputmode="decimal"
                :value="task.estimated_hours ?? ''"
                placeholder="—"
                @change="saveInlineEdit(task, 'estimated_hours', ($event.target as HTMLInputElement).value.trim().replace(',', '.'))"
                @keyup.enter="($event.target as HTMLInputElement).blur()"
              />
            </td>

            <td v-else-if="col.key === 'fact'" class="center num">{{ formatHours(task.spent_hours) }}</td>

            <td v-else-if="col.key === 'status_name'" class="center">
              <select
                :disabled="!auth.canWrite"
                class="cell-input status-select"
                :style="{ color: statusColor(task.status_name) }"
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
                class="cell-input date"
                type="date"
                :value="(task.start_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'start_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td v-else-if="col.key === 'due_date'" class="center">
              <input
                :disabled="!auth.canWrite"
                class="cell-input date"
                :class="{ overdue: isOverdue(task) }"
                type="date"
                :value="(task.due_date || '').slice(0, 10)"
                @change="saveInlineEdit(task, 'due_date', ($event.target as HTMLInputElement).value)"
              />
            </td>

            <td v-else-if="col.key === 'category_name'">
              <select
                :disabled="!auth.canWrite"
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
import { statusColor, priorityColor } from '../../utils/statusColor'
import { useMobileTasks } from '../../composables/useMobileTasks'
import TaskMobileList from './TaskMobileList.vue'

const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const { showChange, toast } = useSwal()
const auth = useAuthStore()
const { t } = useI18n()
const { settings } = storeToRefs(settingsStore)
const { isMobile } = useMobileTasks()

// flat: без своей рамки (таблица стоит внутри группы с рамкой)
defineProps<{ tasks: Task[]; flat?: boolean }>()

// Сотрудники для списка исполнителей — загружаются один раз на все таблицы
// (у каждой раскрывающейся группы свой TaskTable).
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
  // Поле меняется на месте сразу, без перезагрузки таблицы
  ;(task as any)[field] = field === 'estimated_hours' && savedValue !== null ? Number(savedValue) : savedValue
  try {
    const result = await tasksStore.updateTask(task.external_id, { [field]: savedValue }, true)
    if (result && result.redmine_ok === false) {
      ;(task as any)[field] = oldValue
      toast(t('tasks.messages.redmine_sync_error') + ': ' + (result.redmine_error || ''), 'error')
      return
    }
    showChange(oldNormalized || '—', value || '—', fieldLabels[field] ? t(fieldLabels[field]) : field, task.external_id)
  } catch {
    ;(task as any)[field] = oldValue
    toast(t('tasks.messages.save_error'), 'error')
  }
}

const { statuses, priorities, members, projectCategories: projectCategoriesMap, filters, useGrouping } = storeToRefs(tasksStore)

// Сортировка одна на все таблицы задач и хранится вместе с фильтрами:
// таблица строится заново при перезагрузке данных, и собственное состояние
// терялось бы между двумя кликами.
const sortBy = computed(() => filters.value.sort_by)
const sortDir = computed(() => filters.value.sort_dir)

const statusNames = computed(() => statuses.value.map(s => s.name))
const priorityNames = computed(() => priorities.value.map(p => p.name))
// Закрытой задачу делают группы статусов из настроек, а не название статуса
const closedStatusIds = computed(() => new Set(statuses.value.filter(s => s.group === 'closed').map(s => s.id)))

const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  if (selectedTeam.length === 0) return members.value.map(m => m.name).sort()
  return members.value.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

// Текущее значение должно оставаться в списке выбора, даже если его там нет.
function withCurrent(list: string[], current?: string | null): string[] {
  return current && !list.includes(current) ? [current, ...list] : list
}

// Колонки, которые пользователь выбрал, в их порядке
// В группировке по проектам колонка «Проект» лишняя: проект назван в заголовке группы.
// В режиме «Все» group_by хранит прошлую группировку, поэтому смотрим и на useGrouping.
const groupedByProject = computed(() => useGrouping.value && filters.value.group_by === 'project')
const shownColumns = computed(() =>
  settingsStore.tableColumns(TASKS_TABLE, TASK_COLUMNS.map(c => c.key))
    .filter(key => key !== 'project_name' || !groupedByProject.value)
    .map(key => TASK_COLUMNS.find(c => c.key === key)!)
)

function toggleSort(key: string) {
  const dir = sortBy.value === key && sortDir.value === 'asc' ? 'desc' : 'asc'
  emit('sort-change', { sortBy: key, sortDir: dir })
}

function isOverdue(task: Task): boolean {
  if (!task.due_date) return false
  return task.due_date.slice(0, 10) < new Date().toISOString().slice(0, 10) && !closedStatusIds.value.has(task.status_id)
}


function taskLink(id: number): string {
  const base = settings.value.data_source_url || settings.value.redmine_url || ''
  return base ? base.replace(/\/$/, '') + '/issues/' + id : '#'
}

function projectCategories(task: Task): string[] {
  // Все категории проекта задачи; текущее значение остаётся в списке, даже
  // пока список загружается или если категорию удалили в Redmine.
  const projectCats = new Set<string>(projectCategoriesMap.value[task.project_id] || [])
  if (task.category_name) projectCats.add(task.category_name)
  return [...projectCats].sort()
}

// Доля часов на исправление багов в фактических; считается на лету.
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

.project {
  max-width: 200px;
  color: var(--text-muted);
  white-space: normal;
  word-break: break-word;
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

/* Поля правки в строке выглядят обычным текстом, пока на них не навели */
.cell-input {
  padding: 4px 6px;
  font-size: 12px;
  font-family: inherit;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text-bright);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.cell-input:hover:not(:disabled) {
  border-color: var(--hairline);
}

/* Пользователь с правом только на чтение видит значения обычным текстом */
.cell-input:disabled {
  cursor: default;
  opacity: 1;
}

.cell-input:focus-visible {
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

/* Статус выглядит как приоритет: цветной текст без плашки */
.status-select {
  width: 120px;
  font-size: 11px;
  font-weight: 600;
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
