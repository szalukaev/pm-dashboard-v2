<template>
  <div class="analytics-view">
    <!-- Загрузка -->
    <template v-if="loading || !ready">
      <h1 class="page-title page-title-alone">{{ $t('analytics.title') }}</h1>
      <div class="skeleton-stats">
        <div v-for="i in 7" :key="i" class="skeleton-card"></div>
      </div>
    </template>

    <!-- Ошибка -->
    <template v-else-if="error">
      <h1 class="page-title page-title-alone">{{ $t('analytics.title') }}</h1>
      <AppEmptyState state="error" @retry="fetchAll()" />
    </template>

    <!-- Содержимое: виджеты расставляет пользователь; скрытые от него
         администратором не показываются -->
    <DashboardGrid v-else :widgets="WIDGETS" :available="available">
      <template #title>
        <div class="title-row">
          <h1 class="page-title">{{ $t('analytics.title') }}</h1>
          <PeriodFilter :model-value="period" @update:model-value="changePeriod" />
        </div>
      </template>

      <!-- Индикаторы выше порогов, заданных пользователем в настройках -->
      <template v-if="exceeded.length > 0" #banner>
        <div class="threshold-banner" data-testid="threshold-banner">
          <AlertTriangle :size="18" class="threshold-icon" />
          <div>
            <div class="threshold-title">{{ $t('analytics.thresholds.title') }}</div>
            <div v-for="(item, index) in exceeded" :key="index" class="threshold-line">
              {{ $t('analytics.thresholds.line', {
                indicator: $t('analytics.stats.' + item.indicator),
                value: item.value,
                limit: item.limit,
              }) }}<template v-if="item.project_name"> — {{ item.project_name }}</template>
            </div>
          </div>
        </div>
      </template>

      <template #stats>
        <StatCards :stats="stats" :exceeded="exceededIndicators" @open-tasks="openStatTasks" />
      </template>

      <template #health>
        <HealthGauge :health="health" />
      </template>

      <template #velocity>
        <BarChart :labels="velocityChart.labels" :datasets="velocityChart.datasets" :hint="velocityHint" />
      </template>

      <template #hours>
        <BarChart horizontal :labels="hoursChart.labels" :datasets="hoursChart.datasets" />
      </template>

      <template #bug_hours>
        <BarChart horizontal :labels="bugChart.labels" :datasets="bugChart.datasets" :hint="bugHint" />
      </template>

      <template #team_load-actions>
        <AppColumnSettings :table="TEAM_LOAD_TABLE" :columns="TEAM_LOAD_COLUMNS" />
      </template>
      <template #team_load>
        <TeamLoadTable :team="teamLoad" @open-tasks="openTeamTasks" />
      </template>

      <template #distribution-actions>
        <DistributionFilter
          :projects="distributionProjects"
          :hidden="distributionHidden"
          :grouped="distributionGrouped"
          @update:hidden="saveDistribution({ hidden_projects: $event })"
          @update:grouped="saveDistribution({ group_nested: $event })"
        />
      </template>
      <template #distribution>
        <ProjectDistribution :distribution="shownDistribution" @open="openedChart = $event" />
      </template>

      <template #deadlines>
        <DeadlineTiles :deadlines="deadlines" @open-task="openTask" />
      </template>

      <template #gantt>
        <GanttChart
          :tasks="gantt.tasks"
          :projects="gantt.projects"
          :total="gantt.total"
          :project="ganttProject"
          :offset="ganttOffset"
          @update:project="store.setGanttProject($event)"
          @update:offset="store.setGanttOffset($event)"
          @open-task="openTask"
        />
      </template>
    </DashboardGrid>

    <!-- Карточка задачи (общая с вкладкой «Задачи») -->
    <!-- Проекты, из которых состоит диаграмма вложенных проектов -->
    <AppModal
      :model-value="openedChart !== null"
      :title="openedChart ? openedChart.project_name : ''"
      width="1000px"
      expandable
      @close="openedChart = null"
    >
      <div class="nested-charts">
        <ProjectDistribution v-if="openedChart" :distribution="openedChart.parts || []" />
      </div>
    </AppModal>

    <!-- Задачи, стоящие за цифрой, по которой кликнули; задача открывается поверх списка -->
    <AnalyticsTasksModal :query="taskList" @close="taskList = null" @open-task="openTask" />

    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { AlertTriangle } from 'lucide-vue-next'
import { useAnalyticsStore } from '../stores/analytics'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { useLiveRefresh } from '../composables/useLiveRefresh'
import type { WidgetDef } from '../utils/dashboardLayout'
import { roundHours } from '../utils/chartTheme'
import DashboardGrid from '../components/analytics/DashboardGrid.vue'
import BarChart from '../components/analytics/BarChart.vue'
import HealthGauge from '../components/analytics/HealthGauge.vue'
import GanttChart from '../components/analytics/GanttChart.vue'
import DistributionFilter from '../components/analytics/DistributionFilter.vue'
import StatCards from '../components/analytics/StatCards.vue'
import TeamLoadTable from '../components/analytics/TeamLoadTable.vue'
import ProjectDistribution from '../components/analytics/ProjectDistribution.vue'
import DeadlineTiles from '../components/analytics/DeadlineTiles.vue'
import { TEAM_LOAD_TABLE, TEAM_LOAD_COLUMNS } from '../components/analytics/teamLoadColumns'
import AppColumnSettings from '../components/ui/AppColumnSettings.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import TaskModal from '../components/tasks/TaskModal.vue'
import AnalyticsTasksModal from '../components/analytics/AnalyticsTasksModal.vue'
import AppModal from '../components/ui/AppModal.vue'
import PeriodFilter from '../components/analytics/PeriodFilter.vue'
import { PERIOD_MODES, DEFAULT_PERIOD, assigneeLabel, type AnalyticsPeriod, type TaskListQuery, type DistChart, type ProjectDist } from '../stores/analytics'

// Виджеты вкладки в стандартном порядке со стандартными ширинами.
// Идентификаторы — ключи виджетов в правах доступа.
const WIDGETS: WidgetDef[] = [
  { id: 'stats', title: 'analytics.stats_title', cols: 12 },
  { id: 'health', title: 'analytics.health', cols: 4 },
  { id: 'velocity', title: 'analytics.velocity', cols: 8 },
  { id: 'team_load', title: 'analytics.team_load', cols: 12 },
  { id: 'hours', title: 'analytics.hours', cols: 6 },
  { id: 'bug_hours', title: 'analytics.bug_hours', cols: 6 },
  { id: 'distribution', title: 'analytics.distribution', cols: 12 },
  { id: 'deadlines', title: 'analytics.deadlines', cols: 12 },
  { id: 'gantt', title: 'analytics.gantt', cols: 12 },
  // Что показывает виджет и как это считается: analytics.hints.<id>
].map(w => ({ ...w, hint: 'analytics.hints.' + w.id }))

const { t } = useI18n()
const store = useAnalyticsStore()
const auth = useAuthStore()
const settingsStore = useSettingsStore()
const { stats, teamLoad, distribution, deadlines, hours, velocity, health, gantt, ganttProject, ganttOffset, exceeded, loading, error, period } = storeToRefs(store)

// Период отчётов запоминается в настройках пользователя под этим именем
const PERIOD_FILTER = 'analytics_period'

// Сохранённый период; чего не было или что испорчено — период по умолчанию
function savedPeriod(): AnalyticsPeriod {
  const saved = settingsStore.settings.last_filters?.[PERIOD_FILTER]
  if (!saved || !PERIOD_MODES.includes(saved.mode)) return { ...DEFAULT_PERIOD }
  return { mode: saved.mode, from: typeof saved.from === 'string' ? saved.from : '', to: typeof saved.to === 'string' ? saved.to : '' }
}

// Отчёты перестраиваются сразу, выбор запоминается
function changePeriod(next: AnalyticsPeriod) {
  store.setPeriod(next)
  settingsStore.setLastFilters(PERIOD_FILTER, { ...next })
}

// Индикаторы, превысившие порог хотя бы в одном проекте: их карточки красные
const exceededIndicators = computed(() => [...new Set(exceeded.value.map(e => e.indicator))])
const { fetchAll } = store

// Проекты, которые пользователь отключил в «Распределении»; запоминаются в
// настройках пользователя под этим именем
const DISTRIBUTION_FILTER = 'analytics_distribution'

const distributionHidden = computed<number[]>(() => {
  const saved = settingsStore.settings.last_filters?.[DISTRIBUTION_FILTER]?.hidden_projects
  return Array.isArray(saved) ? saved.filter((id: unknown): id is number => typeof id === 'number') : []
})
const distributionProjects = computed(() => distribution.value.map(p => ({ id: p.project_id, name: p.project_name })))
// Родительский проект и его вложенные — одна диаграмма, если не отключено
const distributionGrouped = computed<boolean>(() => settingsStore.settings.last_filters?.[DISTRIBUTION_FILTER]?.group_nested !== false)

const shownDistribution = computed<DistChart[]>(() => {
  const projects = distribution.value.filter(p => !distributionHidden.value.includes(p.project_id))
  if (!distributionGrouped.value) return projects

  const charts = new Map<number, DistChart & { parts: ProjectDist[] }>()
  for (const p of projects) {
    let chart = charts.get(p.root_id)
    if (!chart) {
      chart = { project_id: p.root_id, project_name: p.root_name, assignees: [], parts: [] }
      charts.set(p.root_id, chart)
    }
    chart.parts.push(p)
    for (const a of p.assignees) {
      const same = chart.assignees.find(x => x.name === a.name)
      if (same) same.count += a.count
      else chart.assignees.push({ ...a })
    }
  }
  return [...charts.values()]
    .map(chart => ({ ...chart, assignees: [...chart.assignees].sort((a, b) => a.name.localeCompare(b.name)) }))
    .sort((a, b) => a.project_name.localeCompare(b.project_name))
})

// Диаграмма вложенных проектов, открытая, чтобы увидеть каждый её проект
const openedChart = ref<DistChart | null>(null)

function saveDistribution(change: { hidden_projects?: number[]; group_nested?: boolean }) {
  settingsStore.setLastFilters(DISTRIBUTION_FILTER, {
    hidden_projects: distributionHidden.value,
    group_nested: distributionGrouped.value,
    ...change,
  })
}

// Сотрудник без часов ничего не добавляет на график часов
const hoursRows = computed(() => hours.value.filter(r => r.estimated_hours > 0 || r.spent_hours > 0))
const bugRows = computed(() => hours.value.filter(r => r.bug_hours > 0))

// Что рисуют графики. Это вычисляемые значения, чтобы график
// перерисовывался при изменении своих данных, а не при каждой перерисовке
// страницы.
const velocityChart = computed(() => ({
  labels: velocity.value.map(w => weekLabel(w.week_start)),
  datasets: [{ label: t('analytics.charts.closed'), data: velocity.value.map(w => w.closed), color: '--accent' }],
}))

const hoursChart = computed(() => ({
  labels: hoursRows.value.map(r => assigneeLabel(r.name)),
  datasets: [
    { label: t('analytics.charts.estimated'), data: hoursRows.value.map(r => roundHours(r.estimated_hours)), color: '--accent' },
    { label: t('analytics.charts.spent'), data: hoursRows.value.map(r => roundHours(r.spent_hours)), color: '--orange' },
  ],
}))

const bugChart = computed(() => ({
  labels: bugRows.value.map(r => assigneeLabel(r.name)),
  datasets: [{ label: t('analytics.charts.bug_hours'), data: bugRows.value.map(r => roundHours(r.bug_hours)), color: '--danger' }],
}))

function bugHint(index: number): string {
  const row = bugRows.value[index]
  return row ? t('analytics.charts.bug_share', { pct: row.bug_percent }) : ''
}

function velocityHint(index: number): string {
  const week = velocity.value[index]
  return week ? t('analytics.charts.estimate_sum', { hours: roundHours(week.estimated_hours) }) : ''
}

// Понедельник, с которого начинается неделя (YYYY-MM-DD), в виде DD.MM
function weekLabel(weekStart: string): string {
  const [, month, day] = weekStart.split('-')
  return day + '.' + month
}

const available = computed(() => WIDGETS.map(w => w.id).filter(id => auth.widgetVisible(id)))

// Сохранённая раскладка уже известна: виджеты не прыгают после появления
const ready = ref(false)
const selectedTaskId = ref<number | null>(null)

const taskList = ref<TaskListQuery | null>(null)

function openStatTasks(metric: string, label: string) {
  taskList.value = { title: t('analytics.stats.' + label), metric }
}

function openTeamTasks(name: string, metric: string, label: string) {
  taskList.value = { title: name + ' — ' + t(label), metric, assignee: name }
}

function openTask(id: number) {
  selectedTaskId.value = id
}

// Изменения от синхронизации и других пользователей появляются без перезагрузки
useLiveRefresh(() => fetchAll({ silent: true }))

onMounted(async () => {
  // Сначала настройки: в них период, с которым запрашиваются отчёты
  await settingsStore.ensureLoaded()
  period.value = savedPeriod()
  await fetchAll()
  ready.value = true
})
</script>

<style scoped>
/* Заголовок вкладки и период отчётов рядом; на узком экране период уходит под заголовок */
.title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px 20px;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
}

/* Диаграммы в окне следуют за его шириной так же, как в виджете */
.nested-charts {
  container-type: inline-size;
}

.threshold-banner {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border: 1px solid var(--danger);
  border-radius: 12px;
  background: color-mix(in srgb, var(--danger) 8%, transparent);
}

.threshold-icon {
  flex-shrink: 0;
  margin-top: 1px;
  color: var(--danger);
}

.threshold-title {
  margin-bottom: 4px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.threshold-line {
  font-size: 13px;
  line-height: 1.5;
  color: var(--text);
}

.page-title-alone {
  margin-bottom: 24px;
}

.skeleton-stats {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}

.skeleton-card {
  height: 100px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  animation: skeleton-pulse 1.2s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}
</style>
