<template>
  <div class="gantt">
    <div class="gantt-bar">
      <select
        class="gantt-filter"
        :value="project === null ? '' : project"
        data-testid="gantt-project"
        @change="onProject"
      >
        <option value="">{{ $t('analytics.gantt_all_projects') }}</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
      <span v-if="total > tasks.length" class="gantt-note">
        {{ $t('analytics.gantt_shown', { shown: tasks.length, total }) }}
      </span>
    </div>

    <div v-if="tasks.length === 0" class="empty-hint">{{ $t('common.empty') }}</div>

    <div v-else class="gantt-table" data-testid="gantt">
      <!-- The dates of the scale -->
      <div class="gantt-row gantt-head">
        <div class="gantt-label"></div>
        <div class="gantt-track">
          <span v-for="tick in ticks" :key="tick.day" class="gantt-tick" :style="{ left: tick.left + '%' }">{{ tick.label }}</span>
        </div>
      </div>

      <div v-for="task in bars" :key="task.external_id" class="gantt-row" :data-testid="'gantt-task-' + task.external_id">
        <button type="button" class="gantt-label gantt-name" :title="task.subject" @click="$emit('open-task', task.external_id)">
          <span class="gantt-id">#{{ task.external_id }}</span> {{ task.subject }}
        </button>
        <div class="gantt-track">
          <span class="gantt-today" :style="{ left: todayLeft + '%' }"></span>
          <span
            class="gantt-line"
            :class="{ overdue: task.overdue }"
            :style="{ left: task.left + '%', width: task.width + '%' }"
            :title="task.hint"
          ></span>
        </div>
      </div>

      <div class="gantt-legend">
        <span class="gantt-legend-item"><span class="gantt-legend-today"></span>{{ $t('analytics.gantt_today') }}</span>
        <span class="gantt-legend-item"><span class="gantt-legend-dot overdue"></span>{{ $t('analytics.stats.overdue') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { GanttTask, GanttProject } from '../../stores/analytics'

// tasks: the bars, nearest deadlines first; total: how many tasks there are
// besides the shown ones; project: the project chosen in the filter.
const props = defineProps<{
  tasks: GanttTask[]
  projects: GanttProject[]
  total: number
  project: number | null
}>()

const emit = defineEmits<{
  (e: 'open-task', id: number): void
  (e: 'update:project', id: number | null): void
}>()

// A long-running task must not squeeze the others into a sliver: the scale
// starts at most this many days before today
const MAX_DAYS_BACK = 90
const MIN_DAYS_AHEAD = 7
const TICKS = 6

// A date as the number of its day, so that dates can be subtracted
function dayOf(date: string): number {
  const [y, m, d] = date.split('-').map(Number)
  return Math.round(Date.UTC(y, m - 1, d) / 86400000)
}

function dayLabel(day: number): string {
  const d = new Date(day * 86400000)
  return String(d.getUTCDate()).padStart(2, '0') + '.' + String(d.getUTCMonth() + 1).padStart(2, '0')
}

const today = computed(() => {
  const now = new Date()
  return Math.round(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate()) / 86400000)
})

// The scale: from the earliest start (or today) to the latest deadline
const range = computed(() => {
  let from = today.value
  let to = today.value + MIN_DAYS_AHEAD
  for (const t of props.tasks) {
    from = Math.min(from, dayOf(t.start))
    to = Math.max(to, dayOf(t.due))
  }
  from = Math.max(from, today.value - MAX_DAYS_BACK)
  // The day of the deadline is a part of the bar
  return { from, days: Math.max(1, to + 1 - from) }
})

function leftOf(day: number): number {
  return Math.min(100, Math.max(0, ((day - range.value.from) / range.value.days) * 100))
}

const todayLeft = computed(() => leftOf(today.value))

const ticks = computed(() => {
  const result: { day: number; left: number; label: string }[] = []
  for (let i = 0; i < TICKS; i++) {
    const day = range.value.from + Math.round((range.value.days * i) / TICKS)
    if (!result.some(t => t.day === day)) result.push({ day, left: leftOf(day), label: dayLabel(day) })
  }
  return result
})

const bars = computed(() =>
  props.tasks.map(t => {
    const start = dayOf(t.start)
    const due = dayOf(t.due)
    const left = leftOf(start)
    const hint = [dayLabel(start) + ' — ' + dayLabel(due), t.assigned_to_name, t.project_name].filter(Boolean).join(' · ')
    return { ...t, left, width: Math.max(0.8, leftOf(due + 1) - left), overdue: due < today.value, hint }
  })
)

function onProject(e: Event) {
  const value = (e.target as HTMLSelectElement).value
  emit('update:project', value === '' ? null : Number(value))
}
</script>

<style scoped>
.empty-hint {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  padding: 24px;
}

.gantt-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 12px;
}

.gantt-filter {
  max-width: 100%;
  height: 32px;
  padding: 0 8px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  font-size: 13px;
  color: var(--text);
}

.gantt-note {
  font-size: 12px;
  color: var(--text-muted);
}

.gantt-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 28px;
  border-top: 1px solid var(--border-light);
}

.gantt-head {
  min-height: 22px;
  border-top: none;
}

.gantt-label {
  flex: 0 0 280px;
  min-width: 0;
}

.gantt-name {
  padding: 0;
  border: none;
  background: none;
  font-size: 13px;
  color: var(--text);
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.gantt-name:hover {
  color: var(--accent);
}

.gantt-id {
  color: var(--accent);
  font-size: 12px;
}

.gantt-track {
  position: relative;
  flex: 1;
  min-width: 0;
  align-self: stretch;
  min-height: inherit;
}

.gantt-tick {
  position: absolute;
  top: 2px;
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.gantt-line {
  position: absolute;
  top: 50%;
  height: 12px;
  margin-top: -6px;
  border-radius: 9999px;
  background: var(--accent);
}

.gantt-line.overdue {
  background: var(--danger);
}

.gantt-today {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: var(--warning);
}

.gantt-legend {
  display: flex;
  gap: 16px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--text-dim);
}

.gantt-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.gantt-legend-today {
  width: 2px;
  height: 12px;
  background: var(--warning);
}

.gantt-legend-dot {
  width: 12px;
  height: 8px;
  border-radius: 9999px;
  background: var(--danger);
}

/* In a narrow widget the names give way to the timeline */
@container (max-width: 640px) {
  .gantt-label {
    flex-basis: 130px;
  }
}
</style>
