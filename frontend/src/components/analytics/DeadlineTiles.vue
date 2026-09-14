<template>
  <div class="deadlines-section">
    <h3 class="section-title">{{ $t('analytics.deadlines') }}</h3>

    <div v-if="deadlines.length === 0" class="empty-hint">
      {{ $t('common.empty') }}
    </div>

    <div class="deadlines-grid">
      <div
        v-for="group in deadlines"
        :key="group.name"
        class="deadline-column"
        :data-testid="'deadline-' + group.name"
      >
        <div class="column-header" :class="group.name">
          <span class="column-title">{{ $t('analytics.deadline_groups.' + group.name) }}</span>
          <span class="column-count">{{ group.tasks.length }}</span>
        </div>

        <div v-if="group.tasks.length === 0" class="column-empty">
          —
        </div>

        <div class="column-tasks">
          <div
            v-for="task in group.tasks"
            :key="task.external_id"
            class="task-tile"
            :class="{ 'high-priority': task.priority_id >= 5 }"
            @click="$emit('open-task', task.external_id)"
            :data-testid="'deadline-task-' + task.external_id"
          >
            <a
              :href="taskLink(task.external_id)"
              target="_blank"
              class="tile-id"
              @click.stop
            >
              #{{ task.external_id }}
            </a>
            <div class="tile-subject" :title="task.subject">{{ task.subject }}</div>
            <div class="tile-meta">
              <span class="tile-assignee">{{ task.assigned_to_name || '—' }}</span>
              <span class="tile-project">{{ task.project_name }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import type { DeadlineGroup } from '../../stores/analytics'
import { useSettingsStore } from '../../stores/settings'

const settingsStore = useSettingsStore()
const { settings } = storeToRefs(settingsStore)

defineProps<{ deadlines: DeadlineGroup[] }>()
defineEmits(['open-task'])

function taskLink(externalId: number): string {
  const base = settings.value.data_source_url || ''
  if (base) {
    return base.replace(/\/$/, '') + '/issues/' + externalId
  }
  return '#'
}
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
  letter-spacing: -0.2px;
}

.empty-hint {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  padding: 24px;
}

.deadlines-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.deadline-column {
  min-height: 200px;
}

.column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  margin-bottom: 8px;
}

.column-header.overdue {
  background: var(--danger)22;
}

.column-header.today {
  background: var(--warning)22;
}

.column-header.three_days {
  background: var(--accent-bg);
}

.column-header.week {
  background: var(--surface-2);
}

.column-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.column-header.overdue .column-title { color: var(--danger); }
.column-header.today .column-title { color: var(--warning); }
.column-header.three_days .column-title { color: var(--accent); }
.column-header.week .column-title { color: var(--text-muted); }

.column-count {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
}

.column-empty {
  text-align: center;
  color: var(--text-faintest);
  font-size: 12px;
  padding: 20px 0;
}

.column-tasks {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.task-tile {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.task-tile:hover {
  border-color: var(--hairline-strong);
  transform: translateY(-1px);
}

.task-tile.high-priority {
  border-left: 3px solid var(--danger);
}

.tile-id {
  font-size: 11px;
  font-weight: 500;
  color: var(--accent);
  text-decoration: none;
}

.tile-id:hover {
  text-decoration: underline;
}

.tile-subject {
  font-size: 13px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 4px;
}

.tile-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-faint);
}

@media (max-width: 900px) {
  .deadlines-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .deadlines-grid {
    grid-template-columns: 1fr;
  }
}
</style>
