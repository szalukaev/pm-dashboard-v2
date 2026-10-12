<template>
  <div class="analytics-view">
    <!-- Loading -->
    <template v-if="loading || !ready">
      <h1 class="page-title page-title-alone">{{ $t('analytics.title') }}</h1>
      <div class="skeleton-stats">
        <div v-for="i in 7" :key="i" class="skeleton-card"></div>
      </div>
    </template>

    <!-- Error -->
    <template v-else-if="error">
      <h1 class="page-title page-title-alone">{{ $t('analytics.title') }}</h1>
      <AppEmptyState state="error" @retry="fetchAll()" />
    </template>

    <!-- Content: the user arranges the widgets; the ones hidden from the
         user by the administrator are not shown -->
    <DashboardGrid v-else :widgets="WIDGETS" :available="available">
      <template #title>
        <h1 class="page-title">{{ $t('analytics.title') }}</h1>
      </template>

      <template #stats>
        <StatCards :stats="stats" />
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
        <TeamLoadTable :team="teamLoad" />
      </template>

      <template #distribution-actions>
        <DistributionFilter :projects="distributionProjects" :hidden="distributionHidden" @update:hidden="setDistributionHidden" />
      </template>
      <template #distribution>
        <ProjectDistribution :distribution="shownDistribution" />
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
          @update:project="store.setGanttProject($event)"
          @open-task="openTask"
        />
      </template>
    </DashboardGrid>

    <!-- Task modal (shared with Tasks) -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
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

// The widgets of the tab in the standard order with their standard widths.
// The ids are the keys of the widgets in the access rights.
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
]

const { t } = useI18n()
const store = useAnalyticsStore()
const auth = useAuthStore()
const settingsStore = useSettingsStore()
const { stats, teamLoad, distribution, deadlines, hours, velocity, health, gantt, ganttProject, loading, error } = storeToRefs(store)
const { fetchAll } = store

// The projects the user switched off in «Distribution»; remembered in the
// user's settings under this name
const DISTRIBUTION_FILTER = 'analytics_distribution'

const distributionHidden = computed<number[]>(() => {
  const saved = settingsStore.settings.last_filters?.[DISTRIBUTION_FILTER]?.hidden_projects
  return Array.isArray(saved) ? saved.filter((id: unknown): id is number => typeof id === 'number') : []
})
const distributionProjects = computed(() => distribution.value.map(p => ({ id: p.project_id, name: p.project_name })))
const shownDistribution = computed(() => distribution.value.filter(p => !distributionHidden.value.includes(p.project_id)))

function setDistributionHidden(ids: number[]) {
  settingsStore.setLastFilters(DISTRIBUTION_FILTER, { hidden_projects: ids })
}

// An employee without hours adds nothing to a chart of hours
const hoursRows = computed(() => hours.value.filter(r => r.estimated_hours > 0 || r.spent_hours > 0))
const bugRows = computed(() => hours.value.filter(r => r.bug_hours > 0))

// What the charts draw. Kept as computed values so that a chart is redrawn
// when its data changes and not on every repaint of the page.
const velocityChart = computed(() => ({
  labels: velocity.value.map(w => weekLabel(w.week_start)),
  datasets: [{ label: t('analytics.charts.closed'), data: velocity.value.map(w => w.closed), color: '--accent' }],
}))

const hoursChart = computed(() => ({
  labels: hoursRows.value.map(r => r.name),
  datasets: [
    { label: t('analytics.charts.estimated'), data: hoursRows.value.map(r => roundHours(r.estimated_hours)), color: '--accent' },
    { label: t('analytics.charts.spent'), data: hoursRows.value.map(r => roundHours(r.spent_hours)), color: '--orange' },
  ],
}))

const bugChart = computed(() => ({
  labels: bugRows.value.map(r => r.name),
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

// The Monday a week starts on, YYYY-MM-DD, as DD.MM
function weekLabel(weekStart: string): string {
  const [, month, day] = weekStart.split('-')
  return day + '.' + month
}

const available = computed(() => WIDGETS.map(w => w.id).filter(id => auth.widgetVisible(id)))

// The saved arrangement is known: the widgets do not jump after appearing
const ready = ref(false)
const selectedTaskId = ref<number | null>(null)

function openTask(id: number) {
  selectedTaskId.value = id
}

// Changes from a sync or from other users appear without a reload
useLiveRefresh(() => fetchAll({ silent: true }))

onMounted(async () => {
  await Promise.all([settingsStore.ensureLoaded(), fetchAll()])
  ready.value = true
})
</script>

<style scoped>
.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
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
