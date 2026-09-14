<template>
  <div class="sprint-backlog">
    <h3 class="section-title">
      {{ $t('sprint.backlog') }}
      <span class="backlog-count">{{ totalTasks }}</span>
    </h3>

    <div v-if="backlog.length === 0" class="empty-hint">
      {{ $t('sprint.no_tasks') }}
    </div>

    <div class="backlog-groups">
      <div
        v-for="group in backlog"
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
import { ChevronDown } from 'lucide-vue-next'
import KanbanCard from '../kanban/KanbanCard.vue'
import type { BacklogGroup, SprintTask } from '../../stores/sprint'
import type { KanbanCard as KanbanCardType } from '../../stores/kanban'

const props = defineProps<{ backlog: BacklogGroup[] }>()
defineEmits(['open-task'])

const openGroups = ref<Set<string>>(new Set())

const totalTasks = computed(() =>
  props.backlog.reduce((sum, g) => sum + g.task_count, 0)
)

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
  flex-direction: column;
  gap: 4px;
}
</style>
