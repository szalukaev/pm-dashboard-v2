<template>
  <div class="task-accordion">
    <!-- Блок в рамке с компактной шапкой на каждый проект или исполнителя -->
    <template v-if="sortedGroups.length">
      <div v-for="group in sortedGroups" :key="group.name" class="assignee-group">
        <div
          class="assignee-header"
          @click="toggle(group.name)"
          :data-testid="'accordion-' + group.name"
        >
          <ChevronDown :size="18" class="chevron" :class="{ collapsed: !isOpen(group.name) }" />
          <Folder v-if="groupBy === 'project'" :size="18" class="person" />
          <CircleDot v-else-if="groupBy === 'status' && isMobile" :size="18" class="person" />
          <span v-else-if="groupBy === 'status'" class="status-dot" :style="{ background: statusColor(group.name) }"></span>
          <span
            v-else-if="group.name !== NO_ASSIGNEE"
            class="avatar"
            :style="{ background: avatarColor(group.name) }"
          ><img v-if="avatarSrc(null, group.name)" :src="avatarSrc(null, group.name)" alt="" /><template v-else>{{ initials(group.name) }}</template></span>
          <User v-else :size="18" class="person" />
          <span class="assignee-name">{{ groupTitle(group.name) }}</span>
          <span class="count-badge">{{ group.task_count }}</span>
          <span class="group-metric">
            {{ $t('tasks.accordions.estimate') }}: <b class="estimate">{{ formatTotal(group.estimate_total) }}</b>
          </span>
          <span class="group-metric">
            {{ $t('tasks.accordions.fact') }}: <b class="fact" :class="{ over: !isMobile && group.fact_total > group.estimate_total && group.estimate_total > 0 }">{{ formatTotal(group.fact_total) }}</b>
          </span>
          <!-- Что в группе требует внимания; только на компьютере, у телефона своя шапка -->
          <span v-if="!isMobile" class="group-flags">
            <span v-if="group.overdue" class="flag overdue">{{ $t('tasks.attention.group_overdue', { n: group.overdue }) }}</span>
            <span v-if="group.overrun" class="flag overrun">{{ $t('tasks.attention.group_overrun', { n: group.overrun }) }}</span>
            <span v-if="group.unassigned && groupBy !== 'assignee'" class="flag">{{ $t('tasks.attention.group_unassigned', { n: group.unassigned }) }}</span>
            <!-- Загрузка исполнителя: факт против оценки, перерасход оранжевым.
                 Считаются только задачи с оценкой; по клику — список задач с перерасходом -->
            <span
              v-if="groupBy === 'assignee' && group.estimate_total"
              class="load-wrap"
              @click.stop="toggleOverrunList(group.name)"
            >
              <span class="load">
                <i class="used" :style="{ width: loadUsedPct(group) + '%' }"></i>
                <i class="extra" :style="{ width: loadOverPct(group) + '%' }"></i>
              </span>
              <span class="load-tip" role="tooltip">
                <span>{{ $t('tasks.attention.load_estimate') }}: <b>{{ formatTotal(group.estimate_total) }}</b></span>
                <span>{{ $t('tasks.attention.load_fact') }}: <b>{{ formatTotal(group.estimated_fact_total) }}</b></span>
                <span>{{ $t('tasks.attention.load_overrun_pct') }}: <b>{{ overrunPct(group) }}%</b></span>
              </span>
              <span v-if="overrunList.group === group.name" class="overrun-list" @click.stop>
                <span class="overrun-title">{{ $t('tasks.attention.overrun_list_title') }}</span>
                <span v-if="overrunList.loading" class="overrun-empty">{{ $t('common.loading') }}</span>
                <span v-else-if="!overrunList.tasks.length" class="overrun-empty">{{ $t('tasks.attention.overrun_list_empty') }}</span>
                <button
                  v-for="task in overrunList.tasks"
                  v-else
                  :key="task.external_id"
                  type="button"
                  class="overrun-item"
                  @click="openOverrunTask(task)"
                >
                  <span class="overrun-subject">#{{ task.external_id }} {{ task.subject }}</span>
                  <span class="overrun-hours">{{ formatTotal(task.estimated_hours || 0) }} → <b>{{ formatTotal(task.spent_hours) }}</b> (+{{ taskOverrunPct(task) }}%)</span>
                </button>
              </span>
            </span>
          </span>
        </div>
        <template v-if="isOpen(group.name)">
          <p v-if="group.loading" class="group-loading">{{ $t('common.loading') }}</p>
          <TaskTable
            v-else
            :tasks="group.tasks"
            flat
            @open-task="$emit('open-task', $event)"
            @sort-change="$emit('sort-change', $event)"
          />
          <AppPagination
            :total="group.task_count"
            :limit="store.pageSize"
            :offset="group.offset"
            @update:offset="store.setGroupPage(group.name, $event)"
            @update:limit="store.setPageSize"
          />
        </template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Folder, User, CircleDot } from 'lucide-vue-next'
import { statusColor } from '../../utils/statusColor'
import { initials, avatarColor } from '../../utils/avatar'
import TaskTable from './TaskTable.vue'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { useMemberAvatars } from '../../composables/useMemberAvatars'
import AppPagination from '../ui/AppPagination.vue'
import { useTasksStore, type TaskGroup, type Task } from '../../stores/tasks'

const props = defineProps<{ groups: TaskGroup[]; groupBy: string }>()
const emit = defineEmits(['open-task', 'sort-change'])

const { t } = useI18n()

// Имена групп, которые сервер даёт задачам без исполнителя и без статуса
const NO_ASSIGNEE = 'Без исполнителя'
const NO_STATUS = 'Без статуса'

function groupTitle(name: string): string {
  if (name === NO_ASSIGNEE) return t('tasks.accordions.no_assignee')
  if (name === NO_STATUS) return t('tasks.accordions.no_status')
  return name
}

// Статусы сервер отдаёт в рабочем порядке (открытые, в тестировании,
// закрытые) — его не меняем. Остальные группы: сначала те, где больше
// задач с проблемами, затем по алфавиту; «Без исполнителя» — первой
const { isMobile } = useMobileTasks()
const { avatarSrc } = useMemberAvatars()

const sortedGroups = computed(() =>
  props.groupBy === 'status' ? props.groups : [...props.groups].sort((a, b) => {
    // На телефоне прежний порядок: по алфавиту, «Без исполнителя» — последней
    if (isMobile.value) {
      if (a.name === NO_ASSIGNEE) return 1
      if (b.name === NO_ASSIGNEE) return -1
      return a.name.localeCompare(b.name, 'ru')
    }
    if (a.name === NO_ASSIGNEE) return -1
    if (b.name === NO_ASSIGNEE) return 1
    return problems(b) - problems(a) || a.name.localeCompare(b.name, 'ru')
  })
)

function problems(g: TaskGroup): number {
  return g.overdue + g.overrun + g.unassigned
}

// Полоса загрузки: доля факта до оценки и перерасход сверх неё. Факт берётся
// только по задачам с оценкой — срочные задачи без оценки перерасходом не считаются
function loadUsedPct(g: TaskGroup): number {
  const max = Math.max(g.estimate_total, g.estimated_fact_total)
  return max ? Math.min(g.estimated_fact_total, g.estimate_total) / max * 100 : 0
}

function loadOverPct(g: TaskGroup): number {
  const max = Math.max(g.estimate_total, g.estimated_fact_total)
  return max && g.estimated_fact_total > g.estimate_total ? (g.estimated_fact_total - g.estimate_total) / max * 100 : 0
}

function overrunPct(g: TaskGroup): number {
  if (!g.estimate_total || g.estimated_fact_total <= g.estimate_total) return 0
  return Math.round((g.estimated_fact_total - g.estimate_total) / g.estimate_total * 100)
}

function taskOverrunPct(task: Task): number {
  const est = task.estimated_hours || 0
  return est ? Math.round((task.spent_hours - est) / est * 100) : 0
}

// Список задач с перерасходом под шкалой; открыт не больше чем у одной группы
const overrunList = reactive<{ group: string; loading: boolean; tasks: Task[] }>({ group: '', loading: false, tasks: [] })
let overrunRequest = 0

async function toggleOverrunList(name: string) {
  if (overrunList.group === name) {
    overrunList.group = ''
    return
  }
  const requestId = ++overrunRequest
  Object.assign(overrunList, { group: name, loading: true, tasks: [] })
  try {
    const list = await store.fetchOverrunTasks(name)
    if (requestId === overrunRequest) overrunList.tasks = list
  } catch {
    if (requestId === overrunRequest) overrunList.tasks = []
  } finally {
    if (requestId === overrunRequest) overrunList.loading = false
  }
}

function openOverrunTask(task: Task) {
  overrunList.group = ''
  emit('open-task', task.external_id)
}

function closeOverrunList(e: MouseEvent) {
  if (overrunList.group && !(e.target as HTMLElement).closest?.('.load-wrap')) overrunList.group = ''
}
onMounted(() => document.addEventListener('click', closeOverrunList))
onBeforeUnmount(() => document.removeEventListener('click', closeOverrunList))

// По умолчанию группы свёрнуты; раскрытые хранятся в сторе и сохраняются
// вместе с фильтрами, поэтому переживают закрытие браузера.
const store = useTasksStore()

function toggle(name: string) {
  store.toggleGroup(name)
}

function isOpen(name: string): boolean {
  return store.isGroupExpanded(name)
}

function formatTotal(h: number): string {
  return `${(h || 0).toFixed(1)}${t('tasks.units.hours_short')}`
}
</script>

<style scoped>
.task-accordion {
  display: flex;
  flex-direction: column;
}

.chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform 0.2s;
}

.chevron.collapsed {
  transform: rotate(-90deg);
}

/* Группы */
.assignee-group {
  margin-bottom: 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  /* Без обрезки: подсказка и список перерасхода выходят за рамку группы */
}

.assignee-header {
  border-radius: 7px 7px 0 0;
  display: flex;
  /* Заголовок не прокручивается вместе с таблицей: на узком экране правый
     блок (метки и шкала) переносится строкой ниже, а не уходит за край */
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  padding: 10px 14px;
  background: var(--bg-card);
  cursor: pointer;
  user-select: none;
  transition: background 0.1s;
}

.assignee-header:only-child {
  border-radius: 7px;
}

.assignee-header:hover {
  background: var(--bg-hover);
}

.person {
  color: var(--accent);
}

.assignee-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 9999px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

/* Списки телефона (карточки и сроки) внутри группы — с отступом от рамки,
   как у шапки группы */
.assignee-group :deep(.mobile-cards),
.assignee-group :deep(.mobile-deadlines) {
  padding: 10px 14px;
}

.group-loading {
  padding: 12px 14px;
  font-size: 13px;
  color: var(--text-muted);
}

/* Итоги */
.group-metric {
  white-space: nowrap;
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--text-muted);
}

.group-metric b {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.group-metric .estimate {
  color: var(--cyan);
}

.group-metric .fact {
  color: var(--success);
}

.group-metric .fact.over {
  color: var(--orange);
}

.status-dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.avatar {
  display: inline-grid;
  place-items: center;
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  color: #fff;
  font-size: 9px;
  font-weight: 600;
}

/* Фото пользователя вместо инициалов */
.avatar img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

/* Метки проблем и загрузка — у правого края заголовка */
.group-flags {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  min-width: 0;
}

.flag {
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--surface-3);
  color: var(--text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.flag.overdue {
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  color: var(--danger);
}

.flag.overrun {
  background: color-mix(in srgb, var(--orange) 15%, transparent);
  color: var(--orange);
}

.load {
  display: flex;
  width: 110px;
  height: 6px;
  border-radius: 3px;
  background: var(--surface-3);
  overflow: hidden;
}

.load-wrap {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 6px 0;
  cursor: pointer;
}

.load-tip {
  display: none;
  position: absolute;
  top: 100%;
  right: 0;
  z-index: 20;
  flex-direction: column;
  gap: 2px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-card);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  font-size: 12px;
  white-space: nowrap;
  color: var(--text);
}

.load-wrap:hover .load-tip {
  display: flex;
}

/* Пока открыт список, подсказка не мешает */
.load-wrap:has(.overrun-list) .load-tip {
  display: none;
}

.overrun-list {
  position: absolute;
  top: 100%;
  right: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  width: 380px;
  max-height: 320px;
  overflow-y: auto;
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-card);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
  cursor: default;
  font-size: 12px;
}

.overrun-title {
  padding: 4px 6px;
  font-weight: 600;
  color: var(--text-muted);
}

.overrun-empty {
  padding: 6px;
  color: var(--text-muted);
}

.overrun-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.overrun-item:hover {
  background: var(--surface-3);
}

.overrun-subject {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overrun-hours {
  color: var(--text-muted);
}

.overrun-hours b {
  color: var(--orange);
}

.load .used {
  background: var(--accent);
}

.load .extra {
  background: var(--orange);
}
</style>
