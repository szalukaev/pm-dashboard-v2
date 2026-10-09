<template>
  <div
    class="kanban-column"
    :class="{ 'drag-over': isDragOver }"
    @dragover.prevent="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
    :data-testid="'kanban-column-' + column.id"
  >
    <div class="column-header" draggable="true" @dragstart="onColumnDragStart">
      <GripVertical :size="14" class="drag-handle" />
      <span class="column-name">{{ column.id === UNASSIGNED ? $t('kanban.unassigned') : column.name }}</span>
      <span class="column-count">{{ column.tasks.length }}</span>
    </div>

    <div class="column-body">
      <KanbanCard
        v-for="card in column.tasks"
        :key="card.external_id"
        :card="card"
        @open-task="$emit('open-task', $event)"
      />

      <div v-if="column.tasks.length === 0" class="column-empty">
        —
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { GripVertical } from 'lucide-vue-next'
import KanbanCard from './KanbanCard.vue'
import { UNASSIGNED, type KanbanColumn as ColumnType } from '../../stores/kanban'

const props = defineProps<{ column: ColumnType }>()
defineEmits(['open-task', 'move-card'])

// Data type of a dragged column header (a dragged card uses text/plain)
const COLUMN_DRAG = 'application/x-kanban-column'

const isDragOver = ref(false)

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragOver.value = true
}

function onDragLeave(e: DragEvent) {
  // Only trigger if actually leaving the column
  const related = e.relatedTarget as HTMLElement
  if (!related || !(e.currentTarget as HTMLElement).contains(related)) {
    isDragOver.value = false
  }
}

function onDrop(e: DragEvent) {
  isDragOver.value = false

  // A column header dropped here: this column gives up its place to it
  const fromId = e.dataTransfer?.getData(COLUMN_DRAG)
  if (fromId) {
    e.currentTarget?.dispatchEvent(new CustomEvent('kanban-column-drop', {
      detail: { fromId, toId: props.column.id },
      bubbles: true,
    }))
    return
  }

  const issueId = parseInt(e.dataTransfer?.getData('text/plain') || '0')
  if (issueId) {
    // The parent handles the actual move
    e.currentTarget?.dispatchEvent(new CustomEvent('kanban-drop', {
      detail: { issueId, columnId: props.column.id },
      bubbles: true,
    }))
  }
}

function onColumnDragStart(e: DragEvent) {
  e.dataTransfer?.setData(COLUMN_DRAG, props.column.id)
}
</script>

<style scoped>
.kanban-column {
  min-width: 300px;
  max-width: 340px;
  background: var(--bg-card);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 200px);
}

.kanban-column.drag-over {
  border-color: var(--accent);
  background: var(--accent-bg);
}

.column-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border-light);
  cursor: grab;
  user-select: none;
}

.drag-handle {
  color: var(--text-faintest);
  cursor: grab;
}

.column-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  flex: 1;
}

.column-count {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
  background: var(--tag-bg);
  padding: 2px 8px;
  border-radius: 9999px;
}

.column-body {
  padding: 8px;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.column-empty {
  text-align: center;
  color: var(--text-faintest);
  font-size: 12px;
  padding: 20px 0;
}
</style>
