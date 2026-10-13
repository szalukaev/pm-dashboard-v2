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

      <div class="gantt-legend">
        <span class="gantt-legend-item"><span class="gantt-legend-dot planned"></span>{{ $t('analytics.gantt_planned') }}</span>
        <span class="gantt-legend-item"><span class="gantt-legend-dot late"></span>{{ $t('analytics.stats.overdue') }}</span>
        <span class="gantt-legend-item"><span class="gantt-legend-today"></span>{{ $t('analytics.gantt_today') }}</span>
      </div>
    </div>

    <div v-if="tasks.length === 0" class="empty-hint">{{ $t('common.empty') }}</div>

    <div v-else class="gantt-table" data-testid="gantt">
      <!-- Даты шкалы -->
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
        <div class="gantt-track" @mousemove="showTip($event, task)" @mouseleave="tip = null">
          <span class="gantt-today" :style="{ left: todayLeft + '%' }"></span>
          <!-- Что было запланировано на задачу: от начала до срока -->
          <span class="gantt-line planned" :style="{ left: task.left + '%', width: task.width + '%' }">
            {{ $t('analytics.gantt_days', { n: task.plannedDays }) }}
          </span>
          <!-- Просроченная задача продолжается от срока до сегодняшнего дня -->
          <span
            v-if="task.lateDays > 0"
            class="gantt-line late"
            :style="{ left: task.lateLeft + '%', width: task.lateWidth + '%' }"
          >
            {{ $t('analytics.gantt_late_days', { n: task.lateDays }) }}
          </span>
        </div>
      </div>
    </div>

    <AppPagination :total="total" :limit="PAGE" :offset="offset" :sizes="[PAGE]" @update:offset="$emit('update:offset', $event)" />

    <!-- Следует за указателем над полосой; лежит в body, чтобы её не обрезал ни один виджет -->
    <Teleport to="body">
      <div v-if="tip" class="gantt-tip" :style="{ left: tip.x + 'px', top: tip.y + 'px' }">
        <div class="gantt-tip-title">#{{ tip.task.external_id }} {{ tip.task.subject }}</div>
        <div>{{ $t('analytics.gantt_tip_planned', { from: tip.task.startLabel, to: tip.task.dueLabel, n: tip.task.plannedDays }) }}</div>
        <div v-if="tip.task.lateDays > 0" class="gantt-tip-late">{{ $t('analytics.gantt_tip_late', { n: tip.task.lateDays }) }}</div>
        <div v-if="tip.task.estimated_hours !== null">{{ $t('analytics.gantt_tip_estimate', { hours: tip.task.estimated_hours }) }}</div>
        <div class="gantt-tip-meta">{{ tip.task.meta }}</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { GanttTask, GanttProject } from '../../stores/analytics'
import AppPagination from '../ui/AppPagination.vue'

// tasks: полосы показанной страницы по сроку; total и offset: всего задач и
// с какой начинается страница; project: проект, выбранный в фильтре.
const props = defineProps<{
  tasks: GanttTask[]
  projects: GanttProject[]
  total: number
  offset: number
  project: number | null
}>()

const emit = defineEmits<{
  (e: 'open-task', id: number): void
  (e: 'update:project', id: number | null): void
  (e: 'update:offset', offset: number): void
}>()

const PAGE = 25

// Очень долгая задача не должна сжимать остальные в ниточку: шкала
// начинается не раньше чем за столько дней до сегодняшнего
const MAX_DAYS_BACK = 90
const MIN_DAYS_AHEAD = 7
const TICKS = 6
// Полоса не бывает уже этого значения, в процентах шкалы
const MIN_WIDTH = 0.8

// Дата как номер её дня, чтобы даты можно было вычитать
function dayOf(date: string): number {
  const [y, m, d] = date.split('-').map(Number)
  return Math.round(Date.UTC(y, m - 1, d) / 86400000)
}

function dayLabel(day: number): string {
  const d = new Date(day * 86400000)
  return String(d.getUTCDate()).padStart(2, '0') + '.' + String(d.getUTCMonth() + 1).padStart(2, '0') + '.' + d.getUTCFullYear()
}

const today = computed(() => {
  const now = new Date()
  return Math.round(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate()) / 86400000)
})

// Шкала: от самого раннего начала (или сегодняшнего дня) до самого позднего срока
const range = computed(() => {
  let from = today.value
  let to = today.value + MIN_DAYS_AHEAD
  for (const t of props.tasks) {
    from = Math.min(from, dayOf(t.start))
    to = Math.max(to, dayOf(t.due))
  }
  from = Math.max(from, today.value - MAX_DAYS_BACK)
  // День срока входит в полосу
  return { from, days: Math.max(1, to + 1 - from) }
})

// Где на шкале начинается день, в процентах
function leftOf(day: number): number {
  return Math.min(100, Math.max(0, ((day - range.value.from) / range.value.days) * 100))
}

const todayLeft = computed(() => leftOf(today.value))

const ticks = computed(() => {
  const result: { day: number; left: number; label: string }[] = []
  for (let i = 0; i < TICKS; i++) {
    const day = range.value.from + Math.round((range.value.days * i) / TICKS)
    if (!result.some(t => t.day === day)) result.push({ day, left: leftOf(day), label: dayLabel(day).slice(0, 5) })
  }
  return result
})

const bars = computed(() =>
  props.tasks.map(t => {
    const start = dayOf(t.start)
    const due = dayOf(t.due)
    const left = leftOf(start)
    const end = leftOf(due + 1)
    // Просрочка: дни после срока до сегодняшнего включительно
    const lateDays = Math.max(0, today.value - due)
    return {
      ...t,
      left,
      width: Math.max(MIN_WIDTH, end - left),
      plannedDays: due - start + 1,
      lateDays,
      lateLeft: end,
      lateWidth: Math.max(MIN_WIDTH, leftOf(today.value + 1) - end),
      startLabel: dayLabel(start),
      dueLabel: dayLabel(due),
      meta: [t.assigned_to_name, t.project_name].filter(Boolean).join(' · '),
    }
  })
)

type Bar = (typeof bars.value)[number]

const TIP_WIDTH = 320
const tip = ref<{ x: number; y: number; task: Bar } | null>(null)

function showTip(e: MouseEvent, task: Bar) {
  // Рядом с указателем, в пределах окна
  const x = Math.min(e.clientX + 14, window.innerWidth - TIP_WIDTH - 8)
  tip.value = { x: Math.max(8, x), y: e.clientY + 16, task }
}

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
  justify-content: space-between;
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

.gantt-table {
  margin-bottom: 12px;
}

.gantt-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 30px;
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

/* Полоса с цифрой внутри; не помещающаяся цифра обрезается */
.gantt-line {
  position: absolute;
  top: 50%;
  height: 18px;
  margin-top: -9px;
  padding: 0 4px;
  box-sizing: border-box;
  overflow: hidden;
  font-size: 11px;
  font-weight: 500;
  line-height: 18px;
  color: #fff;
  text-align: center;
  white-space: nowrap;
}

.gantt-line.planned {
  border-radius: 9999px;
  background: var(--accent);
}

.gantt-line.late {
  border-radius: 0 9999px 9999px 0;
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
  flex-wrap: wrap;
  gap: 16px;
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
  width: 14px;
  height: 8px;
  border-radius: 9999px;
}

.gantt-legend-dot.planned { background: var(--accent); }
.gantt-legend-dot.late { background: var(--danger); }

.gantt-tip {
  position: fixed;
  z-index: var(--z-toast);
  width: 320px;
  padding: 10px 12px;
  background: var(--surface-3);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  box-shadow: var(--shadow);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-dim);
  pointer-events: none;
}

.gantt-tip-title {
  margin-bottom: 4px;
  font-weight: 600;
  color: var(--text-bright);
}

.gantt-tip-late {
  color: var(--danger);
}

.gantt-tip-meta {
  margin-top: 4px;
  color: var(--text-muted);
}

/* В узком виджете названия уступают место шкале */
@container (max-width: 640px) {
  .gantt-label {
    flex-basis: 130px;
  }
}
</style>
