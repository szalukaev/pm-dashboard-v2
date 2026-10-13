<template>
  <!-- Весь блок принимает задачу, которую тянут из спринта обратно в бэклог -->
  <div
    class="sprint-backlog"
    :class="{ 'is-drop-target': dragDepth > 0 }"
    @dragenter="onDragEnter"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <h3 class="section-title">
      {{ $t('sprint.backlog') }}
      <span class="backlog-count">{{ totalTasks }}</span>
    </h3>

    <!-- Применяется по Enter: по названию проекта, теме или номеру задачи -->
    <div v-if="backlog.length > 0" class="backlog-filter">
      <Search :size="14" class="filter-icon" />
      <input
        v-model="query"
        type="text"
        :placeholder="$t('sprint.backlog_filter.placeholder')"
        data-testid="backlog-filter"
        @keydown.enter.prevent="applyFilter"
      />
      <button
        v-if="query || applied"
        type="button"
        class="filter-clear"
        :title="$t('sprint.backlog_filter.clear')"
        @click="clearFilter"
      >
        <X :size="14" />
      </button>
    </div>

    <div v-if="backlog.length === 0" class="empty-hint">
      {{ $t('sprint.no_tasks') }}
    </div>
    <div v-else-if="visibleGroups.length === 0" class="empty-hint">
      {{ $t('sprint.backlog_filter.nothing') }}
    </div>

    <div class="backlog-groups">
      <div
        v-for="group in visibleGroups"
        :key="group.project_name"
        class="backlog-group"
      >
        <div
          class="group-header"
          @click="toggle(group.project_name)"
        >
          <ChevronDown
            :size="14"
            class="chevron"
            :class="{ collapsed: !isOpen(group.project_name) }"
          />
          <span class="group-name">{{ group.project_name || $t('sprint.no_project') }}</span>
          <span class="group-count">{{ group.task_count }}</span>
        </div>

        <div v-if="isOpen(group.project_name)" class="group-body">
          <KanbanCard
            v-for="task in pageOf(group)"
            :key="task.external_id"
            :card="mapToKanbanCard(task)"
            compact
            draggable="true"
            @dragstart="onDragStart($event, task)"
            @open-task="$emit('open-task', $event)"
          />
        </div>
        <AppPagination
          v-if="isOpen(group.project_name)"
          :total="group.tasks.length"
          :limit="pageSize"
          :offset="offsetOf(group)"
          @update:offset="offsets[group.project_name] = $event"
          @update:limit="setPageSize"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown, Search, X } from 'lucide-vue-next'
import KanbanCard from '../kanban/KanbanCard.vue'
import AppPagination from '../ui/AppPagination.vue'
import { useSettingsStore } from '../../stores/settings'
import type { BacklogGroup, SprintTask } from '../../stores/sprint'
import { SPRINT_DRAG_TYPE } from '../../stores/sprint'
import type { KanbanCard as KanbanCardType } from '../../stores/kanban'

const props = defineProps<{ backlog: BacklogGroup[] }>()
const emit = defineEmits(['open-task', 'unassign-task'])

// Бэклог принимает только задачу из спринта: у неё в данных перетаскивания
// есть номер спринта. dragenter и dragleave срабатывают на каждом вложенном
// элементе, поэтому подсветка считается, а не переключается.
const dragDepth = ref(0)

function fromSprint(e: DragEvent): boolean {
  return !!e.dataTransfer && Array.from(e.dataTransfer.types).includes(SPRINT_DRAG_TYPE)
}

function onDragEnter(e: DragEvent) {
  if (!fromSprint(e)) return
  e.preventDefault()
  dragDepth.value++
}

function onDragOver(e: DragEvent) {
  if (!fromSprint(e)) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onDragLeave() {
  if (dragDepth.value > 0) dragDepth.value--
}

function onDrop(e: DragEvent) {
  dragDepth.value = 0
  if (!fromSprint(e)) return
  e.preventDefault()
  const issueId = parseInt(e.dataTransfer?.getData('text/plain') || '', 10)
  const sprintId = parseInt(e.dataTransfer?.getData(SPRINT_DRAG_TYPE) || '', 10)
  if (issueId && sprintId) emit('unassign-task', { sprintId, issueId })
}

const openGroups = ref<Set<string>>(new Set())

// Каждый проект показывает свои задачи постранично; бэклог уже загружен
// целиком (фильтр работает по всему бэклогу), поэтому страницы режутся здесь.
const settingsStore = useSettingsStore()
settingsStore.ensureLoaded()
const PAGE_TABLE = 'backlog'
const pageSize = computed(() => settingsStore.pageSize(PAGE_TABLE))
const offsets = ref<Record<string, number>>({})

// Вместо страницы, которой больше нет (задачи ушли из бэклога), показывается последняя
function offsetOf(group: BacklogGroup): number {
  const last = group.tasks.length > 0 ? Math.floor((group.tasks.length - 1) / pageSize.value) * pageSize.value : 0
  return Math.min(offsets.value[group.project_name] || 0, last)
}

function pageOf(group: BacklogGroup): SprintTask[] {
  const start = offsetOf(group)
  return group.tasks.slice(start, start + pageSize.value)
}

async function setPageSize(size: number) {
  await settingsStore.setPageSize(PAGE_TABLE, size)
  offsets.value = {}
}

// query — то, что печатают; applied — то, по чему отобран список
const query = ref('')
const applied = ref('')

// Проект с подходящим названием остаётся целиком; иначе — только его подходящие задачи
const visibleGroups = computed<BacklogGroup[]>(() => {
  const q = applied.value
  if (!q) return props.backlog
  const number = q.replace(/^#/, '')
  return props.backlog.flatMap(group => {
    if (group.project_name.toLowerCase().includes(q)) return [group]
    const tasks = group.tasks.filter(t =>
      t.subject.toLowerCase().includes(q) || String(t.external_id).includes(number)
    )
    return tasks.length ? [{ ...group, tasks, task_count: tasks.length }] : []
  })
})

const totalTasks = computed(() =>
  visibleGroups.value.reduce((sum, g) => sum + g.task_count, 0)
)

function applyFilter() {
  applied.value = query.value.trim().toLowerCase()
  offsets.value = {}
  // Найденное показывается сразу, без раскрытия каждого проекта вручную
  if (applied.value) openGroups.value = new Set(visibleGroups.value.map(g => g.project_name))
}

function clearFilter() {
  query.value = ''
  applied.value = ''
  offsets.value = {}
}

function toggle(name: string) {
  if (openGroups.value.has(name)) {
    openGroups.value.delete(name)
  } else {
    openGroups.value.add(name)
  }
}

function isOpen(name: string): boolean {
  return openGroups.value.has(name)
}

function onDragStart(e: DragEvent, task: SprintTask) {
  if (e.dataTransfer) {
    e.dataTransfer.setData('text/plain', String(task.external_id))
    e.dataTransfer.effectAllowed = 'move'
  }
}

function mapToKanbanCard(task: SprintTask): KanbanCardType {
  return {
    external_id: task.external_id,
    subject: task.subject,
    project_name: task.project_name,
    status_name: task.status_name,
    status_id: 0,
    priority_name: task.priority_name,
    priority_id: task.priority_id,
    assigned_to_name: task.assigned_to_name,
    assigned_to_id: null,
    category_name: '',
    due_date: task.due_date,
    estimated_hours: task.estimated_hours,
    is_overdue: task.is_overdue,
  }
}
</script>

<style scoped>
.sprint-backlog {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 16px;
}

.sprint-backlog.is-drop-target {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-bg);
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.backlog-count {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
  background: var(--tag-bg);
  padding: 2px 8px;
  border-radius: 9999px;
}

.backlog-filter {
  position: relative;
  display: flex;
  align-items: center;
  max-width: 420px;
  margin-bottom: 12px;
}

.backlog-filter input {
  width: 100%;
  padding: 8px 32px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.backlog-filter input:focus-visible {
  border-color: var(--accent);
}

.filter-icon {
  position: absolute;
  left: 10px;
  color: var(--text-faint);
  pointer-events: none;
}

.filter-clear {
  position: absolute;
  right: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 9999px;
  color: var(--text-muted);
}

.filter-clear:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.empty-hint {
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
  padding: 24px;
}

.backlog-groups {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.backlog-group {
  border: 1px solid var(--border-light);
  border-radius: 8px;
  overflow: hidden;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--surface-2);
  cursor: pointer;
  user-select: none;
}

.group-header:hover {
  background: var(--bg-hover);
}

.chevron {
  color: var(--text-faint);
  transition: transform 0.2s;
}

.chevron.collapsed {
  transform: rotate(-90deg);
}

.group-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.group-count {
  font-size: 11px;
  color: var(--text-muted);
  margin-left: auto;
}

.group-body {
  padding: 8px;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 6px;
}
</style>
