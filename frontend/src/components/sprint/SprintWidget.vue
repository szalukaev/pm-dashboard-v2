<template>
  <!-- The whole block takes a dragged task: the header and a collapsed sprint too -->
  <div
    class="sprint-widget"
    :class="{ 'is-closed': sprint.status === 'closed', 'is-drop-target': dragDepth > 0 }"
    @dragenter="onDragEnter"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <div class="widget-header" @click="toggleExpanded">
      <ChevronDown :size="16" class="chevron" :class="{ collapsed: !expanded }" />
      <Flag :size="16" class="flag-icon" />
      <span class="sprint-name">{{ sprint.name }}</span>
      <SprintStateBadge :sprint="sprint" />
      <span class="task-count">{{ sprint.task_count }}</span>
      <span v-if="sprint.start_date || sprint.due_date" class="date-range">
        {{ formatDate(sprint.start_date) }} — {{ formatDate(sprint.due_date) }}
      </span>

      <div class="header-actions" v-if="sprint.status !== 'closed'" @click.stop>
        <!-- Each action is a right of its own -->
        <button v-if="auth.canSprint('close')" class="action-btn" @click="$emit('complete', sprint.id)" :title="$t('sprint.complete')">
          <CheckCircle2 :size="16" />
        </button>
        <button v-if="auth.canSprint('edit')" class="action-btn" @click="$emit('edit', sprint.id)" :title="$t('common.edit')">
          <Pencil :size="16" />
        </button>
        <button v-if="auth.canSprint('tasks')" class="action-btn" @click="$emit('refresh', sprint.id)" :title="$t('sprint.refresh_from_redmine')">
          <RefreshCw :size="16" />
        </button>
        <button v-if="auth.canSprint('delete')" class="action-btn danger-btn" @click="$emit('delete', sprint.id)" :title="$t('common.delete')">
          <Trash2 :size="16" />
        </button>
      </div>
    </div>

    <div v-if="expanded" class="widget-body">
      <!-- Progress bar -->
      <SprintProgressBar
        :progress="sprint.progress"
        :open-count="sprint.open_count"
        :testing-count="sprint.testing_count"
        :closed-count="sprint.closed_count"
        :total-count="sprint.task_count"
      />

      <!-- Description -->
      <p v-if="sprint.description" class="sprint-desc">{{ sprint.description }}</p>

      <!-- Tasks grid -->
      <div
        v-if="sprint.tasks.length > 0"
        class="tasks-grid"
      >
        <KanbanCard
          v-for="task in sprint.tasks"
          :key="task.external_id"
          :card="toKanbanCard(task)"
          compact
          @open-task="$emit('open-task', $event)"
        />
      </div>

      <div v-else class="empty-hint">
        {{ $t('sprint.drag_hint') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown, Flag, CheckCircle2, Pencil, RefreshCw, Trash2 } from 'lucide-vue-next'
import SprintProgressBar from './SprintProgressBar.vue'
import SprintStateBadge from './SprintStateBadge.vue'
import KanbanCard from '../kanban/KanbanCard.vue'
import type { Sprint, SprintTask } from '../../stores/sprint'
import type { KanbanCard as KanbanCardType } from '../../stores/kanban'
import { useAuthStore } from '../../stores/auth'

const props = defineProps<{
  sprint: Sprint
}>()

const emit = defineEmits(['complete', 'edit', 'refresh', 'delete', 'open-task', 'assign-task'])
const auth = useAuthStore()

const expanded = ref(props.sprint.status !== 'closed')

function toKanbanCard(task: SprintTask): KanbanCardType {
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

function toggleExpanded() {
  expanded.value = !expanded.value
}

// Tasks are taken by a sprint that is not closed
const acceptsTasks = () => props.sprint.status !== 'closed'

// dragenter and dragleave fire for every child element the pointer crosses,
// so the highlight is counted, not switched
const dragDepth = ref(0)

function onDragEnter(e: DragEvent) {
  if (!acceptsTasks()) return
  e.preventDefault()
  dragDepth.value++
}

function onDragOver(e: DragEvent) {
  if (!acceptsTasks()) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onDragLeave() {
  if (dragDepth.value > 0) dragDepth.value--
}

function onDrop(e: DragEvent) {
  dragDepth.value = 0
  if (!acceptsTasks()) return
  e.preventDefault()
  const issueId = parseInt(e.dataTransfer?.getData('text/plain') || '', 10)
  if (issueId) {
    expanded.value = true
    emit('assign-task', { sprintId: props.sprint.id, issueId })
  }
}

function formatDate(d: string | null): string {
  if (!d) return ''
  return new Date(d).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' })
}
</script>

<style scoped>
.sprint-widget {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
}

.sprint-widget.is-closed {
  opacity: 0.7;
}

.sprint-widget.is-drop-target {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-bg);
}

.widget-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  cursor: pointer;
  transition: background 0.15s;
  user-select: none;
}

.widget-header:hover {
  background: var(--bg-hover);
}

.chevron {
  color: var(--text-faint);
  transition: transform 0.2s;
  flex-shrink: 0;
}

.chevron.collapsed {
  transform: rotate(-90deg);
}

.flag-icon {
  color: var(--accent);
  flex-shrink: 0;
}

.sprint-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.task-count {
  font-size: 12px;
  color: var(--text-muted);
}

.date-range {
  font-size: 11px;
  color: var(--text-faint);
  margin-left: auto;
}

.header-actions {
  display: flex;
  gap: 4px;
  margin-left: auto;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.action-btn:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.danger-btn:hover {
  color: var(--danger);
}

.widget-body {
  padding: 16px;
  border-top: 1px solid var(--border-light);
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sprint-desc {
  font-size: 13px;
  color: var(--text-dim);
  line-height: 1.5;
}

.tasks-grid {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 6px;
  min-height: 60px;
}

.empty-hint {
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
  padding: 24px;
  border: 1px dashed var(--hairline);
  border-radius: 8px;
  min-height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
