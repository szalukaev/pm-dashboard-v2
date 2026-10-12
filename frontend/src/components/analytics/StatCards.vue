<template>
  <div class="stats-grid">
    <AppStatCard
      v-for="stat in stats"
      :key="stat.label"
      :label="$t('analytics.stats.' + stat.label)"
      :value="stat.is_pct ? stat.value.toFixed(1) + '%' : Math.round(stat.value)"
      :variant="(stat.variant as 'danger' | 'success' | 'warning' | 'info' | undefined)"
      :class="{ clickable: stat.label in METRICS }"
      :title="stat.label in METRICS ? $t('analytics.show_tasks') : undefined"
      data-testid="stat-card"
      @click="open(stat.label)"
    />
  </div>
</template>

<script setup lang="ts">
import AppStatCard from '../ui/AppStatCard.vue'
import type { StatCard } from '../../stores/analytics'

defineProps<{ stats: StatCard[] }>()

// open-tasks: a card that counts tasks was clicked; metric names the figure
// in the list of tasks behind it
const emit = defineEmits<{
  (e: 'open-tasks', metric: string, label: string): void
}>()

// The cards that count tasks and the figures they stand for
const METRICS: Record<string, string> = {
  open: 'open',
  in_test: 'testing',
  overdue: 'overdue',
  bugs: 'bugs',
  no_estimate: 'no_estimate',
}

function open(label: string) {
  if (label in METRICS) emit('open-tasks', METRICS[label], label)
}
</script>

<style scoped>
/* The cards share the whole width of the widget, however many there are */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.stats-grid .clickable {
  cursor: pointer;
}

.stats-grid .clickable:hover {
  border-color: var(--accent);
}

@container (max-width: 420px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
