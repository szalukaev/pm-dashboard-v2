<template>
  <div class="task-accordion">
    <!-- By project: a heading above each project's table -->
    <template v-if="groupBy === 'project'">
      <section v-for="group in sortedGroups" :key="group.name" class="project-group">
        <h2
          class="project-title"
          @click="toggle(group.name)"
          :data-testid="'accordion-' + group.name"
        >
          <ChevronDown :size="18" class="chevron" :class="{ collapsed: !isOpen(group.name) }" />
          {{ group.name }}
          <span class="project-count">({{ group.task_count }})</span>
          <span class="group-metric">
            Оценка: <b class="estimate">{{ formatTotal(group.estimate_total) }}</b>
          </span>
          <span class="group-metric">
            Факт: <b class="fact">{{ formatTotal(group.fact_total) }}</b>
          </span>
        </h2>
        <TaskTable v-if="isOpen(group.name)" :tasks="group.tasks" @open-task="$emit('open-task', $event)" />
      </section>
    </template>

    <!-- By assignee: framed blocks with a compact header -->
    <template v-else>
      <div v-for="group in sortedGroups" :key="group.name" class="assignee-group">
        <div
          class="assignee-header"
          @click="toggle(group.name)"
          :data-testid="'accordion-' + group.name"
        >
          <ChevronDown :size="18" class="chevron" :class="{ collapsed: !isOpen(group.name) }" />
          <User :size="18" class="person" />
          <span class="assignee-name">{{ group.name }}</span>
          <span class="count-badge">{{ group.task_count }}</span>
          <span class="group-metric">
            Оценка: <b class="estimate">{{ formatTotal(group.estimate_total) }}</b>
          </span>
          <span class="group-metric">
            Факт: <b class="fact">{{ formatTotal(group.fact_total) }}</b>
          </span>
        </div>
        <TaskTable v-if="isOpen(group.name)" :tasks="group.tasks" flat @open-task="$emit('open-task', $event)" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDown, User } from 'lucide-vue-next'
import TaskTable from './TaskTable.vue'
import { useTasksStore, type TaskGroup } from '../../stores/tasks'

const props = defineProps<{ groups: TaskGroup[]; groupBy: string }>()
defineEmits(['open-task'])

const NO_ASSIGNEE = 'Без исполнителя'

const sortedGroups = computed(() =>
  [...props.groups].sort((a, b) => {
    if (a.name === NO_ASSIGNEE) return 1
    if (b.name === NO_ASSIGNEE) return -1
    return a.name.localeCompare(b.name, 'ru')
  })
)

// Groups are collapsed by default; expanded ones are kept in the store and
// saved with the filters, so they survive closing the browser.
const store = useTasksStore()

function toggle(name: string) {
  store.toggleGroup(name)
}

function isOpen(name: string): boolean {
  return store.isGroupExpanded(name)
}

function formatTotal(h: number): string {
  return `${(h || 0).toFixed(1)}ч`
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

/* By project */
.project-group {
  margin-top: 16px;
}

.project-group:first-child {
  margin-top: 0;
}

.project-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
  cursor: pointer;
  user-select: none;
}

.project-count {
  font-size: 14px;
  font-weight: 400;
  color: var(--text-faint);
}

/* By assignee */
.assignee-group {
  margin-bottom: 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}

.assignee-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--bg-card);
  cursor: pointer;
  user-select: none;
  transition: background 0.1s;
}

.assignee-header:hover {
  background: var(--bg-hover);
}

.person {
  color: var(--accent);
}

.assignee-name {
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
  border-radius: 4px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

/* Totals */
.group-metric {
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--text-muted);
}

.project-title .group-metric:first-of-type {
  margin-left: 8px;
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
</style>
