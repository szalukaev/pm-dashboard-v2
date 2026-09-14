<template>
  <div class="progress-bar-wrapper" :title="`${Math.round(progress)}%`">
    <div class="progress-bar">
      <div
        class="progress-fill closed"
        :style="{ width: closedPct + '%' }"
      ></div>
      <div
        class="progress-fill testing"
        :style="{ width: testingPct + '%' }"
      ></div>
    </div>
    <div class="progress-stats">
      <span class="stat open">{{ openCount }} {{ $t('sprint.progress.open') }}</span>
      <span class="stat testing">{{ testingCount }} {{ $t('sprint.progress.testing') }}</span>
      <span class="stat total">{{ totalCount }} {{ $t('sprint.progress.total') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  progress: number
  openCount: number
  testingCount: number
  closedCount: number
  totalCount: number
}>()

const closedPct = computed(() => {
  if (props.totalCount === 0) return 0
  return (props.closedCount / props.totalCount) * 100
})

const testingPct = computed(() => {
  if (props.totalCount === 0) return 0
  return (props.testingCount / props.totalCount) * 100 * 0.5
})

const progressColor = computed(() => {
  if (props.progress >= 70) return 'var(--success)'
  if (props.progress >= 30) return 'var(--warning)'
  return 'var(--danger)'
})
</script>

<style scoped>
.progress-bar-wrapper {
  width: 100%;
}

.progress-bar {
  height: 6px;
  background: var(--surface-3);
  border-radius: 3px;
  overflow: hidden;
  display: flex;
}

.progress-fill {
  height: 100%;
  transition: width 0.3s;
}

.progress-fill.closed {
  background: v-bind(progressColor);
}

.progress-fill.testing {
  background: v-bind(progressColor);
  opacity: 0.5;
}

.progress-stats {
  display: flex;
  gap: 12px;
  margin-top: 6px;
  font-size: 11px;
}

.stat {
  color: var(--text-faint);
}

.stat.open { color: var(--accent); }
.stat.testing { color: var(--teal); }
.stat.total { color: var(--text-muted); }
</style>
