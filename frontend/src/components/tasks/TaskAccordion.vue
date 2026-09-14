<template>
  <div class="task-accordion">
    <div
      v-for="group in groups"
      :key="group.name"
      class="accordion-group"
    >
      <div
        class="accordion-header"
        @click="toggle(group.name)"
        :data-testid="'accordion-' + group.name"
      >
        <ChevronDown
          :size="16"
          class="chevron"
          :class="{ collapsed: !isOpen(group.name) }"
        />
        <span class="group-name">{{ group.name }}</span>
        <span class="group-count">{{ group.task_count }}</span>
        <span class="group-metric">
          <span class="metric-label">{{ $t('tasks.accordions.estimate') }}:</span>
          {{ formatHours(group.estimate_total) }}
        </span>
        <span class="group-metric">
          <span class="metric-label">{{ $t('tasks.accordions.fact') }}:</span>
          {{ formatHours(group.fact_total) }}
        </span>
      </div>

      <div v-if="isOpen(group.name)" class="accordion-body">
        <TaskTable :tasks="group.tasks" @open-task="$emit('open-task', $event)" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import TaskTable from './TaskTable.vue'
import type { TaskGroup } from '../../stores/tasks'

defineProps<{ groups: TaskGroup[] }>()
defineEmits(['open-task'])

const openGroups = ref<Set<string>>(new Set())

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

function formatHours(h: number): string {
  if (!h) return '0:00 ч.'
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  return `${hours}:${mins.toString().padStart(2, '0')} ч.`
}
</script>

<style scoped>
.task-accordion {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.accordion-group {
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
}

.accordion-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--surface-1);
  cursor: pointer;
  transition: background 0.15s;
  user-select: none;
}

.accordion-header:hover {
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

.group-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.2px;
}

.group-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 20px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
  background: var(--tag-bg);
  color: var(--text-muted);
}

.group-metric {
  font-size: 12px;
  color: var(--text-faint);
  margin-left: auto;
}

.group-metric + .group-metric {
  margin-left: 0;
}

.metric-label {
  color: var(--text-faintest);
}

.accordion-body {
  border-top: 1px solid var(--border-light);
}
</style>
