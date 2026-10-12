<template>
  <!-- The whole block takes a dragged task: the header and a collapsed sprint too -->
  <div
    class="sprint-widget"
    :class="{ 'is-closed': sprint.status === 'closed', 'is-overdue': sprint.is_overdue, 'is-drop-target': dragDepth > 0 }"
    :data-testid="'sprint-' + sprint.id"
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
      <!-- Закрыт позже даты окончания -->
      <span v-if="sprint.closed_late" class="late-mark" :title="$t('sprint.closed_late_hint')">{{ $t('sprint.state.overdue') }}</span>
      <!-- Вид спринта; у чужого общего — кто его создал -->
      <span class="sprint-kind" :title="$t(sprint.is_shared ? 'sprint.kind.shared_hint' : 'sprint.kind.personal_hint')">
        <Users v-if="sprint.is_shared" :size="13" />
        <Lock v-else :size="13" />
        {{ $t(sprint.is_shared ? 'sprint.kind.shared' : 'sprint.kind.personal') }}
        <template v-if="sprint.is_shared && !sprint.is_own && sprint.owner_name"> · {{ sprint.owner_name }}</template>
      </span>
      <!-- Открытых / в тестировании / всего -->
      <span class="task-count" :title="$t('sprint.counts_hint')" data-testid="sprint-counts">
        <span class="count-open">{{ sprint.open_count }}</span> /
        <span class="count-testing">{{ sprint.testing_count }}</span> /
        {{ sprint.task_count }}
      </span>
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

      <!-- Закрытый спринт можно открыть заново или удалить -->
      <div v-else class="header-actions" @click.stop>
        <button v-if="auth.canSprint('close')" class="action-btn" @click="$emit('reopen', sprint.id)" :title="$t('sprint.reopen')" data-testid="sprint-reopen">
          <RotateCcw :size="16" />
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
          :class="'group-' + task.status_group"
          @dragstart="onTaskDragStart"
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
import { ChevronDown, Flag, CheckCircle2, Pencil, RefreshCw, RotateCcw, Trash2, Users, Lock } from 'lucide-vue-next'
import SprintProgressBar from './SprintProgressBar.vue'
import SprintStateBadge from './SprintStateBadge.vue'
import KanbanCard from '../kanban/KanbanCard.vue'
import type { Sprint, SprintTask } from '../../stores/sprint'
import { SPRINT_DRAG_TYPE } from '../../stores/sprint'
import type { KanbanCard as KanbanCardType } from '../../stores/kanban'
import { useAuthStore } from '../../stores/auth'

const props = defineProps<{
  sprint: Sprint
}>()

const emit = defineEmits(['complete', 'reopen', 'edit', 'refresh', 'delete', 'open-task', 'assign-task'])
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
  const fromSprintId = parseInt(e.dataTransfer?.getData(SPRINT_DRAG_TYPE) || '', 10) || null
  // Задачу бросили в тот же спринт, из которого взяли
  if (!issueId || fromSprintId === props.sprint.id) return
  expanded.value = true
  emit('assign-task', { sprintId: props.sprint.id, issueId, fromSprintId })
}

// Плитка сама кладёт в перетаскивание номер задачи; здесь добавляется спринт,
// из которого её тянут, — по нему бэклог и другие спринты узнают источник
function onTaskDragStart(e: DragEvent) {
  e.dataTransfer?.setData(SPRINT_DRAG_TYPE, String(props.sprint.id))
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
  opacity: 0.8;
}

/* Просроченный спринт заметен в списке: цветная рамка и полоса слева */
.sprint-widget.is-overdue {
  border-color: var(--warning);
  box-shadow: inset 4px 0 0 var(--warning);
}

.late-mark {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 9999px;
  white-space: nowrap;
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 14%, transparent);
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
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.count-open { color: var(--accent); }
.count-testing { color: var(--teal); }

.sprint-kind {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
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

/* Сделанное отступает на второй план: закрытые задачи приглушены */
.tasks-grid :deep(.group-closed) {
  opacity: 0.55;
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
