<template>
  <div class="stats-grid">
    <AppStatCard
      v-for="stat in stats"
      :key="stat.label"
      :label="$t('analytics.stats.' + stat.label)"
      :value="stat.is_pct ? stat.value.toFixed(1) + '%' : Math.round(stat.value)"
      :variant="(stat.variant as 'danger' | 'success' | 'warning' | 'info' | undefined)"
      data-testid="stat-card"
    />
  </div>
</template>

<script setup lang="ts">
import AppStatCard from '../ui/AppStatCard.vue'
import type { StatCard } from '../../stores/analytics'

defineProps<{ stats: StatCard[] }>()
</script>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}

@media (max-width: 480px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
