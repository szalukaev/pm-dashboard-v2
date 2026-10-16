<template>
  <!-- «Аналитика» на телефоне: блоки раздела, нарисованные под узкий экран -->
  <div class="ma" data-testid="mobile-analytics">
    <!-- Обзор: цифры 2×2, здоровье кольцом, Velocity столбиками -->
    <template v-if="section === 'overview'">
      <div v-if="has('stats')" class="ma-kpis">
        <button
          v-for="stat in kpis"
          :key="stat.label"
          type="button"
          class="ma-box ma-kpi"
          :class="{ danger: isDanger(stat), clickable: stat.label in STAT_METRICS }"
          :data-testid="'ma-stat-' + stat.label"
          @click="openStat(stat.label)"
        >
          <span class="ma-label">{{ $t('analytics.stats.' + stat.label) }}</span>
          <b class="ma-big">{{ stat.is_pct ? stat.value.toFixed(1) + '%' : Math.round(stat.value) }}</b>
        </button>
      </div>

      <div v-if="has('health') && health" class="ma-box">
        <div class="ma-label">{{ $t('analytics.health') }}</div>
        <div class="ma-health">
          <svg viewBox="0 0 36 36" class="ma-ring" :class="'level-' + health.level">
            <circle cx="18" cy="18" r="15" class="ma-ring-track" />
            <circle cx="18" cy="18" r="15" class="ma-ring-fill" :stroke-dasharray="`${health.score * 0.9425} 94.25`" />
            <text x="18" y="21" text-anchor="middle">{{ health.score }}</text>
          </svg>
          <div class="ma-health-text">
            <b>{{ $t('analytics.health_levels.' + health.level) }}</b>
            <span>{{ $t('analytics.health_active', { n: health.active }) }}</span>
            <span v-for="part in health.parts" :key="part.key" class="ma-small">
              {{ $t('analytics.health_parts.' + part.key) }}: {{ part.count }} ({{ part.share }}%)
            </span>
          </div>
        </div>
      </div>

      <div v-if="has('velocity') && velocity.length" class="ma-box">
        <div class="ma-label">{{ $t('analytics.velocity') }}</div>
        <div class="ma-bars">
          <div v-for="(week, i) in velocity" :key="week.week_start" class="ma-bar-col">
            <span class="ma-bar-val">{{ week.closed || '' }}</span>
            <i :class="{ last: i === velocity.length - 1 }" :style="{ height: barHeight(week.closed, maxClosed) }"></i>
            <span class="ma-bar-lbl">{{ weekLabel(week.week_start) }}</span>
          </div>
        </div>
      </div>
    </template>

    <!-- Команда: люди со шкалой открытых задач, часы и баг-часы -->
    <template v-else-if="section === 'team'">
      <div v-if="has('team_load') && teamLoad.length" class="ma-box">
        <div class="ma-label">{{ $t('analytics.team_load') }}</div>
        <button
          v-for="row in teamLoad"
          :key="row.name"
          type="button"
          class="ma-row clickable"
          :data-testid="'ma-team-' + row.name"
          @click="$emit('open-team', row.name, 'open', 'analytics.stats.open')"
        >
          <span class="ma-avatar" :style="{ background: avatarColor(row.name) }">
            <img v-if="avatarSrc(null, row.name)" :src="avatarSrc(null, row.name)" alt="" />
            <template v-else>{{ initials(row.name) }}</template>
          </span>
          <span class="ma-row-main">
            <span class="ma-row-name">{{ row.name }}</span>
            <span class="ma-track"><i :style="{ width: share(row.open, maxOpen) }" :class="{ hot: row.overdue > 0 }"></i></span>
          </span>
          <span class="ma-row-val">
            <b>{{ row.open }}</b>
            <small v-if="row.overdue" class="bad">{{ $t('analytics.mobile.overdue_short', { n: row.overdue }) }}</small>
          </span>
        </button>
      </div>

      <div v-if="has('hours') && hours.length" class="ma-box">
        <div class="ma-label">{{ $t('analytics.hours') }}</div>
        <div v-for="row in hours" :key="row.name" class="ma-row">
          <span class="ma-row-main">
            <span class="ma-row-name">{{ row.name }}</span>
            <span class="ma-track"><i class="spent" :style="{ width: share(row.spent_hours, maxSpent) }"></i></span>
          </span>
          <span class="ma-row-val"><b>{{ round(row.spent_hours) }}</b><small>{{ $t('analytics.mobile.of_estimate', { h: round(row.estimated_hours) }) }}</small></span>
        </div>
      </div>

      <div v-if="has('bug_hours') && bugRows.length" class="ma-box">
        <div class="ma-label">{{ $t('analytics.bug_hours') }}</div>
        <div v-for="row in bugRows" :key="row.name" class="ma-row">
          <span class="ma-row-main">
            <span class="ma-row-name">{{ row.name }}</span>
            <span class="ma-track"><i class="bug" :style="{ width: share(row.bug_hours, maxBug) }"></i></span>
          </span>
          <span class="ma-row-val"><b>{{ round(row.bug_hours) }}</b><small>{{ row.bug_percent }}%</small></span>
        </div>
      </div>
    </template>

    <!-- Сроки: задачи списком, дата слева -->
    <template v-else-if="section === 'deadlines'">
      <template v-if="has('deadlines')">
        <div v-for="group in deadlineGroups" :key="group.name" class="ma-box">
          <div class="ma-label" :class="'grp-' + group.name">{{ $t('analytics.deadline_groups.' + group.name) }} · {{ group.tasks.length }}</div>
          <button
            v-for="task in group.tasks"
            :key="task.external_id"
            type="button"
            class="ma-dl clickable"
            :data-testid="'ma-deadline-' + task.external_id"
            @click="$emit('open-task', task.external_id)"
          >
            <span class="ma-dl-date" :class="'grp-' + group.name">
              <b>{{ day(task.due_date) }}</b><small>{{ month(task.due_date) }}</small>
            </span>
            <span class="ma-dl-main">
              <span class="ma-dl-subject">{{ task.subject }}</span>
              <small>#{{ task.external_id }} · {{ task.project_name }}<template v-if="task.assigned_to_name"> · {{ task.assigned_to_name }}</template></small>
            </span>
          </button>
        </div>
        <div v-if="!deadlineGroups.length" class="ma-empty">{{ $t('analytics.mobile.no_deadlines') }}</div>
      </template>
    </template>

    <!-- Проекты: открытые задачи шкалами -->
    <template v-else-if="section === 'projects'">
      <div v-if="has('distribution') && projects.length" class="ma-box">
        <div class="ma-label">{{ $t('analytics.distribution') }}</div>
        <div v-for="p in projects" :key="p.name" class="ma-row">
          <span class="ma-row-main">
            <span class="ma-row-name">{{ p.name }}</span>
            <span class="ma-track"><i class="proj" :style="{ width: share(p.count, maxProject) }"></i></span>
          </span>
          <span class="ma-row-val"><b>{{ p.count }}</b></span>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { StatCard, Health, VelocityWeek, TeamLoadRow, HoursRow, DeadlineGroup, ProjectDist } from '../../stores/analytics'
import { useMemberAvatars } from '../../composables/useMemberAvatars'
import { initials, avatarColor } from '../../utils/avatar'

const props = defineProps<{
  section: string
  available: string[]
  stats: StatCard[]
  exceeded: string[]
  health: Health | null
  velocity: VelocityWeek[]
  teamLoad: TeamLoadRow[]
  hours: HoursRow[]
  deadlines: DeadlineGroup[]
  distribution: ProjectDist[]
}>()

const emit = defineEmits<{
  'open-stat': [metric: string, label: string]
  'open-team': [name: string, metric: string, label: string]
  'open-task': [id: number]
}>()

const { locale } = useI18n()
const { avatarSrc } = useMemberAvatars()

function has(id: string) {
  return props.available.includes(id)
}

// Цифры статистики, за которыми есть список задач (как у карточек на компьютере)
const STAT_METRICS: Record<string, string> = {
  open: 'open',
  in_test: 'testing',
  overdue: 'overdue',
  bugs: 'bugs',
  no_estimate: 'no_estimate',
}

// Главные четыре цифры — первыми, остальные следом
const KPI_ORDER = ['open', 'overdue', 'in_test', 'closed', 'completion', 'bugs', 'no_estimate', 'high_priority', 'active_projects']
const kpis = computed(() => [...props.stats].sort((a, b) => order(a.label) - order(b.label)))
function order(label: string) {
  const i = KPI_ORDER.indexOf(label)
  return i < 0 ? 99 : i
}

function isDanger(stat: StatCard) {
  return props.exceeded.includes(stat.label) || (stat.variant === 'danger' && stat.value > 0)
}

function openStat(label: string) {
  if (label in STAT_METRICS) emit('open-stat', STAT_METRICS[label], label)
}

const maxClosed = computed(() => Math.max(1, ...props.velocity.map(w => w.closed)))
const maxOpen = computed(() => Math.max(1, ...props.teamLoad.map(r => r.open)))
const maxSpent = computed(() => Math.max(1, ...props.hours.map(r => r.spent_hours)))
const bugRows = computed(() => props.hours.filter(r => r.bug_hours > 0).sort((a, b) => b.bug_hours - a.bug_hours))
const maxBug = computed(() => Math.max(1, ...bugRows.value.map(r => r.bug_hours)))

// Открытые задачи по корневым проектам
const projects = computed(() => {
  const byRoot = new Map<string, number>()
  for (const p of props.distribution) {
    const count = p.assignees.reduce((sum, a) => sum + a.count, 0)
    byRoot.set(p.root_name, (byRoot.get(p.root_name) || 0) + count)
  }
  return [...byRoot.entries()].map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count)
})
const maxProject = computed(() => Math.max(1, ...projects.value.map(p => p.count)))

const deadlineGroups = computed(() => props.deadlines.filter(g => g.tasks.length > 0))

function share(value: number, max: number) {
  return Math.max(2, Math.min(100, (value / max) * 100)) + '%'
}

function barHeight(value: number, max: number) {
  return Math.max(3, (value / max) * 100) + '%'
}

function round(h: number) {
  return Math.round(h * 10) / 10
}

// Неделя подписывается днём и месяцем её понедельника
function weekLabel(weekStart: string) {
  const [, m, d] = weekStart.split('-')
  return `${d}.${m}`
}

function day(date?: string) {
  return date ? String(Number(date.slice(8, 10))) : '—'
}

function month(date?: string) {
  if (!date) return ''
  return new Date(date.slice(0, 10) + 'T00:00:00').toLocaleString(locale.value, { month: 'short' }).replace('.', '')
}

</script>

<style scoped>
.ma {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ma-box {
  padding: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.ma-label {
  margin-bottom: 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.clickable {
  cursor: pointer;
}

button.ma-box,
button.ma-row,
button.ma-dl {
  width: 100%;
  font: inherit;
  color: inherit;
  text-align: left;
}

.ma-kpis {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.ma-kpi {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ma-kpi .ma-label {
  margin: 0;
}

.ma-big {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.1;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
}

.ma-kpi.danger .ma-big {
  color: var(--danger);
}

.ma-health {
  display: flex;
  align-items: center;
  gap: 14px;
}

.ma-ring {
  width: 96px;
  height: 96px;
  flex: none;
}

.ma-ring circle {
  fill: none;
  stroke-width: 4;
}

.ma-ring-track {
  stroke: var(--surface-3);
}

.ma-ring-fill {
  stroke: var(--success);
  stroke-linecap: round;
  transform: rotate(-90deg);
  transform-origin: 18px 18px;
}

.ma-ring.level-warning .ma-ring-fill { stroke: var(--warning); }
.ma-ring.level-bad .ma-ring-fill { stroke: var(--danger); }

.ma-ring text {
  font-size: 9px;
  font-weight: 700;
  fill: var(--text-bright);
}

.ma-health-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.ma-health-text b {
  font-size: 15px;
  color: var(--text-bright);
}

.ma-small {
  font-size: 12px;
}

.ma-bars {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 120px;
}

.ma-bar-col {
  flex: 1;
  height: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
  gap: 3px;
}

.ma-bar-col i {
  display: block;
  width: 100%;
  border-radius: 4px 4px 0 0;
  background: var(--accent);
  opacity: 0.75;
}

.ma-bar-col i.last {
  opacity: 1;
  background: var(--accent-hover);
}

.ma-bar-val {
  font-size: 10px;
  color: var(--text-muted);
}

.ma-bar-lbl {
  font-size: 9px;
  color: var(--text-faint);
}

.ma-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 0;
  border: 0;
  border-bottom: 1px solid var(--border-light);
  background: none;
}

.ma-row:last-child {
  border-bottom: 0;
}

.ma-avatar {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

.ma-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ma-row-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.ma-row-name {
  overflow: hidden;
  font-size: 13px;
  color: var(--text-bright);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ma-track {
  display: block;
  height: 7px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--surface-3);
}

.ma-track i {
  display: block;
  height: 100%;
  border-radius: 4px;
  background: var(--accent);
}

.ma-track i.hot { background: var(--danger); }
.ma-track i.spent { background: var(--warning); }
.ma-track i.bug { background: var(--danger); }
.ma-track i.proj { background: var(--success); }

.ma-row-val {
  flex: none;
  min-width: 44px;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font-variant-numeric: tabular-nums;
}

.ma-row-val b {
  font-size: 15px;
  color: var(--text-bright);
}

.ma-row-val small {
  font-size: 11px;
  color: var(--text-faint);
}

.ma-row-val small.bad {
  color: var(--danger);
}

.ma-dl {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 0;
  border: 0;
  border-bottom: 1px solid var(--border-light);
  background: none;
}

.ma-dl:last-child {
  border-bottom: 0;
}

.ma-dl-date {
  width: 42px;
  flex: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 4px 0;
  border-radius: 10px;
  background: var(--surface-3);
}

.ma-dl-date b {
  font-size: 17px;
  line-height: 1.1;
  color: var(--text-bright);
}

.ma-dl-date small {
  font-size: 10px;
  color: var(--text-muted);
}

.ma-dl-date.grp-overdue { background: color-mix(in srgb, var(--danger) 20%, transparent); }
.ma-dl-date.grp-today,
.ma-dl-date.grp-three_days { background: color-mix(in srgb, var(--warning) 20%, transparent); }
.ma-label.grp-overdue { color: var(--danger); }
.ma-label.grp-today,
.ma-label.grp-three_days { color: var(--warning); }

.ma-dl-main {
  flex: 1;
  min-width: 0;
}

.ma-dl-subject {
  display: block;
  overflow: hidden;
  font-size: 14px;
  color: var(--text-bright);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ma-dl-main small {
  display: block;
  overflow: hidden;
  font-size: 12px;
  color: var(--text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ma-empty {
  padding: 40px 16px;
  text-align: center;
  color: var(--text-faint);
}
</style>
