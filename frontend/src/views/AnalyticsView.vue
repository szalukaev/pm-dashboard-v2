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

      <template #team_load-actions>
        <AppColumnSettings :table="TEAM_LOAD_TABLE" :columns="TEAM_LOAD_COLUMNS" />
      </template>
      <template #team_load>
        <TeamLoadTable :team="teamLoad" />
      </template>

      <template #distribution>
        <ProjectDistribution :distribution="distribution" />
      </template>

      <template #deadlines>
        <DeadlineTiles :deadlines="deadlines" @open-task="openTask" />
      </template>
    </DashboardGrid>

    <!-- Task modal (shared with Tasks) -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useAnalyticsStore } from '../stores/analytics'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { useLiveRefresh } from '../composables/useLiveRefresh'
import type { WidgetDef } from '../utils/dashboardLayout'
import DashboardGrid from '../components/analytics/DashboardGrid.vue'
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
  { id: 'team_load', title: 'analytics.team_load', cols: 12 },
  { id: 'distribution', title: 'analytics.distribution', cols: 12 },
  { id: 'deadlines', title: 'analytics.deadlines', cols: 12 },
]

const store = useAnalyticsStore()
const auth = useAuthStore()
const settingsStore = useSettingsStore()
const { stats, teamLoad, distribution, deadlines, loading, error } = storeToRefs(store)
const { fetchAll } = store

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
