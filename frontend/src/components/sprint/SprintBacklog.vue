<template>
  <div class="sprint-backlog">
    <h3 class="section-title">
      {{ $t('sprint.backlog') }}
      <span class="backlog-count">{{ totalTasks }}</span>
    </h3>

    <!-- Applied on Enter: by project name, task subject or task number -->
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
          <span class="group-name">{{ group.project_name }}</span>
          <span class="group-count">{{ group.task_count }}</span>
        </div>

        <div v-if="isOpen(group.project_name)" class="group-body">
          <KanbanCard
            v-for="task in group.tasks"
            :key="task.external_id"
            :card="mapToKanbanCard(task)"
            compact
            draggable="true"
            @dragstart="onDragStart($event, task)"
            @open-task="$emit('open-task', $event)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown, Search, X } from 'lucide-vue-next'
import KanbanCard from '../kanban/KanbanCard.vue'
import type { BacklogGroup, SprintTask } from '../../stores/sprint'
import type { KanbanCard as KanbanCardType } from '../../stores/kanban'

const props = defineProps<{ backlog: BacklogGroup[] }>()
defineEmits(['open-task'])

const openGroups = ref<Set<string>>(new Set())

// query is what is being typed, applied is what the list is filtered by
const query = ref('')
const applied = ref('')

// A project whose name matches stays whole; otherwise only its matching tasks
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
  // What was found is shown at once, without opening every project by hand
  if (applied.value) openGroups.value = new Set(visibleGroups.value.map(g => g.project_name))
}

function clearFilter() {
  query.value = ''
  applied.value = ''
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

.backlog-filter input:focus {
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
