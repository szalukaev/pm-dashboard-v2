<template>
  <div
    class="kanban-card"
    :class="{ 'is-overdue': card.is_overdue, 'is-dragging': isDragging, 'is-compact': compact, 'flash-update': isFlashing(card.external_id) }"
    :style="{ borderLeftColor: priorityColor }"
    draggable="true"
    @dragstart="onDragStart"
    @dragend="onDragEnd"
    @click="$emit('open-task', card.external_id)"
    :data-testid="'kanban-card-' + card.external_id"
  >
    <div class="card-header">
      <a
        :href="issueLink"
        target="_blank"
        class="card-id"
        @click.stop
      >
        #{{ card.external_id }}
      </a>
      <span v-if="card.is_overdue" class="overdue-icon" :title="$t('kanban.card.overdue')">
        <AlertCircle :size="14" />
      </span>
    </div>

    <div class="card-subject" :title="card.subject">
      {{ subject }}
    </div>

    <div class="card-badges">
      <span class="badge priority-badge" :style="{ background: priorityColor + '22', color: priorityColor }">
        {{ card.priority_name }}
      </span>
      <span class="badge status-badge">
        {{ card.status_name }}
      </span>
      <span v-if="card.estimated_hours" class="badge estimate-badge">
        {{ card.estimated_hours }}ч
      </span>
      <span v-if="card.due_date" class="badge date-badge" :class="{ 'overdue-date': card.is_overdue }">
        {{ formatDate(card.due_date) }}
      </span>
    </div>

    <div class="card-footer">
      <span class="card-category" v-if="card.category_name">{{ card.category_name }}</span>
      <span class="card-assignee">{{ card.assigned_to_name || '—' }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { AlertCircle } from 'lucide-vue-next'
import type { KanbanCard } from '../../stores/kanban'
import { useSettingsStore } from '../../stores/settings'
import { isFlashing } from '../../composables/useFlash'

const settingsStore = useSettingsStore()
const { settings } = storeToRefs(settingsStore)

// compact: a tile as wide as its subject, several in a row (sprints, backlog)
const props = defineProps<{ card: KanbanCard; compact?: boolean }>()

const COMPACT_SUBJECT_MAX = 150

const subject = computed(() => {
  const text = props.card.subject
  if (!props.compact || text.length <= COMPACT_SUBJECT_MAX) return text
  return text.slice(0, COMPACT_SUBJECT_MAX).trimEnd() + '…'
})
defineEmits(['open-task'])

const issueLink = computed(() => {
  const base = settings.value.data_source_url || ''
  if (base) {
    return base.replace(/\/$/, '') + '/issues/' + props.card.external_id
  }
  return '#'
})

const isDragging = ref(false)

const priorityColor = computed(() => {
  const style = getComputedStyle(document.documentElement)
  const colors: Record<number, string> = {
    1: style.getPropertyValue('--priority-low').trim(),
    2: style.getPropertyValue('--priority-low').trim(),
    3: style.getPropertyValue('--priority-normal').trim(),
    4: style.getPropertyValue('--priority-high').trim(),
    5: style.getPropertyValue('--priority-urgent').trim(),
    6: style.getPropertyValue('--priority-immediate').trim(),
  }
  return colors[props.card.priority_id] || style.getPropertyValue('--priority-low').trim()
})

function onDragStart(e: DragEvent) {
  isDragging.value = true
  if (e.dataTransfer) {
    e.dataTransfer.setData('text/plain', String(props.card.external_id))
    e.dataTransfer.effectAllowed = 'move'
  }
}

function onDragEnd() {
  isDragging.value = false
}

function formatDate(d: string): string {
  const date = new Date(d)
  return date.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' })
}
</script>

<style scoped>
.kanban-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-left: 3px solid var(--hairline);
  border-radius: 8px;
  padding: 12px;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;
}

.kanban-card:hover {
  border-color: var(--hairline-strong);
  transform: translateY(-1px);
}

.kanban-card.is-dragging {
  opacity: 0.5;
}

.kanban-card.is-overdue {
  border-left-color: var(--critical);
}

.kanban-card.is-compact {
  width: fit-content;
  max-width: 100%;
  padding: 8px 10px;
}

.kanban-card.is-compact .card-header {
  margin-bottom: 2px;
}

.kanban-card.is-compact .card-subject {
  display: block;
  white-space: nowrap;
  text-overflow: ellipsis;
  margin-bottom: 6px;
}

.kanban-card.is-compact .card-badges {
  margin-bottom: 4px;
}

.kanban-card.is-compact .card-footer {
  gap: 12px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.card-id {
  font-size: 11px;
  font-weight: 500;
  color: var(--accent);
  text-decoration: none;
}

.card-id:hover {
  text-decoration: underline;
}

.overdue-icon {
  color: var(--critical);
}

.card-subject {
  font-size: 13px;
  color: var(--text);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 8px;
}

.card-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 8px;
}

.badge {
  display: inline-block;
  padding: 2px 6px;
  font-size: 10px;
  font-weight: 500;
  border-radius: 9999px;
  white-space: nowrap;
}

.status-badge {
  background: var(--tag-bg);
  color: var(--text-muted);
}

.estimate-badge {
  background: var(--accent-bg);
  color: var(--accent);
}

.date-badge {
  background: var(--tag-bg);
  color: var(--text-faint);
}

.overdue-date {
  color: var(--critical);
  background: var(--critical)22;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  color: var(--text-faint);
}

.card-category {
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-assignee {
  text-align: right;
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
