<template>
  <div class="analytics-view">
    <h1 class="page-title">{{ $t('analytics.title') }}</h1>

    <!-- Loading -->
    <template v-if="loading">
      <div class="skeleton-stats">
        <div v-for="i in 7" :key="i" class="skeleton-card"></div>
      </div>
    </template>

    <!-- Error -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchAll" />

    <!-- Content -->
    <template v-else>
      <StatCards :stats="stats" />
      <TeamLoadTable :team="teamLoad" />
      <ProjectDistribution ref="distRef" :distribution="distribution" />
      <DeadlineTiles :deadlines="deadlines" @open-task="openTask" />
    </template>

    <!-- Task modal (shared with Tasks) -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useAnalyticsStore } from '../stores/analytics'
import StatCards from '../components/analytics/StatCards.vue'
import TeamLoadTable from '../components/analytics/TeamLoadTable.vue'
import ProjectDistribution from '../components/analytics/ProjectDistribution.vue'
import DeadlineTiles from '../components/analytics/DeadlineTiles.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import TaskModal from '../components/tasks/TaskModal.vue'

const store = useAnalyticsStore()
const { stats, teamLoad, distribution, deadlines, loading, error } = storeToRefs(store)
const { fetchAll } = store

const distRef = ref<InstanceType<typeof ProjectDistribution> | null>(null)
const selectedTaskId = ref<number | null>(null)

function openTask(id: number) {
  selectedTaskId.value = id
}

// Render charts when distribution data changes
watch(distribution, () => {
  nextTick(() => {
    distRef.value?.renderCharts(distribution.value)
  })
}, { deep: true })

onMounted(async () => {
  await fetchAll()
  nextTick(() => {
    distRef.value?.renderCharts(distribution.value)
  })
})
</script>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
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
