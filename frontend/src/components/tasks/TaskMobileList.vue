<template>
  <!-- Карточки: одна задача — одна карточка во всю ширину -->
  <div v-if="mode === 'cards'" class="mobile-cards" data-testid="task-cards">
    <div
      v-for="task in tasks"
      :key="task.external_id"
      class="task-card"
      :class="{ 'flash-update': isFlashing(task.external_id) }"
      :data-testid="'task-card-' + task.external_id"
      @click="$emit('open-task', task.external_id)"
    >
      <div class="card-head">
        <span class="task-number">#{{ task.external_id }}</span>
        <span class="card-project">{{ task.project_name }}</span>
      </div>
      <div class="card-subject">{{ task.subject }}</div>
      <div class="card-foot">
        <span class="chip" :style="chipStyle(statusColor(task.status_name))">
          <span class="dot" :style="{ background: statusColor(task.status_name) }"></span>
          {{ task.status_name }}
        </span>
        <span v-if="task.priority_name" class="chip" :style="chipStyle(priorityColor({ id: task.priority_id, name: task.priority_name }))">
          {{ task.priority_name }}
        </span>
        <span class="card-right">
          <span v-if="task.due_date" class="card-due" :class="{ overdue: isOverdue(task) }">
            {{ formatDue(task.due_date) }}
          </span>
          <span
            class="avatar"
            :style="{ background: avatarColor(task.assigned_to_name) }"
            :title="task.assigned_to_name || $t('tasks.table.unassigned')"
          ><img v-if="avatarSrc(task.assigned_to_id, task.assigned_to_name)" :src="avatarSrc(task.assigned_to_id, task.assigned_to_name)" alt="" /><template v-else>{{ initials(task.assigned_to_name) }}</template></span>
        </span>
      </div>
    </div>
  </div>

  <!-- Лента сроков: задачи по срокам — просрочено, сегодня, на неделе, позже -->
  <div v-else class="mobile-deadlines" data-testid="task-deadlines">
    <section v-for="bucket in buckets" :key="bucket.key" class="bucket">
      <div class="bucket-title" :class="'bucket-' + bucket.key">
        <span>{{ $t('tasks.mobile.buckets.' + bucket.key) }}</span>
        <span>{{ bucket.tasks.length }}</span>
      </div>
      <div
        v-for="task in bucket.tasks"
        :key="task.external_id"
        class="deadline-item"
        :class="{ 'flash-update': isFlashing(task.external_id) }"
        :data-testid="'task-card-' + task.external_id"
        @click="$emit('open-task', task.external_id)"
      >
        <div class="deadline-date">
          <template v-if="task.due_date">
            <b>{{ dueDay(task.due_date) }}</b>
            <small>{{ dueMonth(task.due_date) }}</small>
          </template>
          <small v-else>—</small>
        </div>
        <div class="deadline-card" :style="{ borderLeftColor: statusColor(task.status_name) }">
          <div v-if="task.project_name" class="deadline-project">{{ task.project_name }}</div>
          <div class="deadline-subject">{{ task.subject }}</div>
          <div class="deadline-foot">
            <span class="task-number">#{{ task.external_id }}</span>
            <span class="chip" :style="chipStyle(statusColor(task.status_name))">{{ task.status_name }}</span>
            <span
              class="avatar small"
              :style="{ background: avatarColor(task.assigned_to_name) }"
              :title="task.assigned_to_name || $t('tasks.table.unassigned')"
            ><img v-if="avatarSrc(task.assigned_to_id, task.assigned_to_name)" :src="avatarSrc(task.assigned_to_id, task.assigned_to_name)" alt="" /><template v-else>{{ initials(task.assigned_to_name) }}</template></span>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useTasksStore, type Task } from '../../stores/tasks'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { isFlashing } from '../../composables/useFlash'
import { statusColor, priorityColor } from '../../utils/statusColor'
import { initials, avatarColor } from '../../utils/avatar'
import { useMemberAvatars } from '../../composables/useMemberAvatars'

const props = defineProps<{ tasks: Task[] }>()
defineEmits(['open-task'])
const { avatarSrc } = useMemberAvatars()

const { locale } = useI18n()
const { mode } = useMobileTasks()
const { statuses } = storeToRefs(useTasksStore())

// Закрытой задачу делают группы статусов из настроек, а не название статуса
const closedStatusIds = computed(() => new Set(statuses.value.filter(s => s.group === 'closed').map(s => s.id)))

// Сегодняшняя дата по местному времени в виде ГГГГ-ММ-ДД
function localDate(offsetDays = 0): string {
  const d = new Date()
  d.setDate(d.getDate() + offsetDays)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function isClosed(task: Task): boolean {
  return closedStatusIds.value.has(task.status_id)
}

function isOverdue(task: Task): boolean {
  return !!task.due_date && task.due_date.slice(0, 10) < localDate() && !isClosed(task)
}

type BucketKey = 'overdue' | 'today' | 'week' | 'later' | 'none' | 'closed'
const BUCKET_ORDER: BucketKey[] = ['overdue', 'today', 'week', 'later', 'none', 'closed']

function bucketOf(task: Task): BucketKey {
  if (isClosed(task)) return 'closed'
  if (!task.due_date) return 'none'
  const due = task.due_date.slice(0, 10)
  if (due < localDate()) return 'overdue'
  if (due === localDate()) return 'today'
  if (due <= localDate(7)) return 'week'
  return 'later'
}

// Группы ленты строятся по задачам, которые уже загружены (текущая страница)
const buckets = computed(() => {
  const byKey = new Map<BucketKey, Task[]>()
  for (const task of props.tasks) {
    const key = bucketOf(task)
    if (!byKey.has(key)) byKey.set(key, [])
    byKey.get(key)!.push(task)
  }
  return BUCKET_ORDER
    .filter(key => byKey.has(key))
    .map(key => ({
      key,
      tasks: byKey.get(key)!.sort((a, b) => (a.due_date || '9999').localeCompare(b.due_date || '9999')),
    }))
})

function parseDate(value: string): Date {
  const [y, m, d] = value.slice(0, 10).split('-').map(Number)
  return new Date(y, m - 1, d)
}

function formatDue(value: string): string {
  return parseDate(value).toLocaleDateString(locale.value, { day: 'numeric', month: 'short' })
}

function dueDay(value: string): number {
  return parseDate(value).getDate()
}

function dueMonth(value: string): string {
  return parseDate(value).toLocaleDateString(locale.value, { month: 'short' }).replace('.', '')
}

// Полупрозрачный фон и текст цвета статуса или приоритета
function chipStyle(color: string) {
  return { background: `color-mix(in srgb, ${color} 16%, transparent)`, color }
}
</script>

<style scoped>
.mobile-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.task-card {
  padding: 13px 14px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  cursor: pointer;
}

.task-card:active,
.deadline-card:active {
  border-color: var(--accent);
}

.card-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 5px;
  font-size: 12px;
}

.task-number {
  color: var(--accent);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.card-project {
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-subject {
  margin-bottom: 10px;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--text-bright);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.card-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-due {
  color: var(--text-muted);
}

.card-due.overdue {
  color: var(--danger);
  font-weight: 600;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.avatar {
  width: 24px;
  height: 24px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 50%;
  font-size: 10px;
  font-weight: 700;
  color: #fff;
}

/* Фото пользователя вместо инициалов */
.avatar img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

.avatar.small {
  width: 20px;
  height: 20px;
  font-size: 9px;
  margin-left: auto;
}

/* Лента сроков */
.bucket-title {
  display: flex;
  justify-content: space-between;
  margin: 16px 0 8px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.bucket:first-child .bucket-title {
  margin-top: 4px;
}

.bucket-overdue { color: var(--danger); }
.bucket-today { color: var(--warning); }
.bucket-week { color: var(--accent); }

.deadline-item {
  position: relative;
  display: grid;
  grid-template-columns: 44px 1fr;
  gap: 12px;
  padding-bottom: 12px;
  cursor: pointer;
}

/* Линия, связывающая даты в ленту */
.deadline-item::before {
  content: '';
  position: absolute;
  left: 21px;
  top: 32px;
  bottom: 0;
  width: 2px;
  background: var(--border);
}

.deadline-date {
  text-align: center;
  line-height: 1;
  padding-top: 4px;
}

.deadline-date b {
  display: block;
  font-size: 20px;
  color: var(--text-bright);
}

.deadline-date small {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--text-muted);
}

.deadline-card {
  min-width: 0;
  padding: 10px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-left: 3px solid;
  border-radius: 10px;
}

.deadline-project {
  margin-bottom: 2px;
  overflow: hidden;
  font-size: 11px;
  color: var(--text-muted);
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.deadline-subject {
  margin-bottom: 6px;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--text-bright);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.deadline-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11.5px;
}
</style>
