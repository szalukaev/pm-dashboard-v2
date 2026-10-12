<template>
  <div class="progress-bar-wrapper">
    <div class="progress-line">
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
      <!-- Готовность числом: закрытая задача — целая доля, в тестировании — половина -->
      <span class="progress-percent" :title="$t('sprint.progress.hint')" data-testid="sprint-progress">{{ percent }}%</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

// Сами числа задач показаны в шапке спринта; здесь они нужны только для
// ширины частей шкалы
const props = defineProps<{
  progress: number
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

// Целое число, а где оно скрыло бы долю — с одним знаком: 12.5
const percent = computed(() => Math.round(props.progress * 10) / 10)

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

.progress-line {
  display: flex;
  align-items: center;
  gap: 10px;
}

.progress-bar {
  flex: 1;
  height: 6px;
  background: var(--surface-3);
  border-radius: 3px;
  overflow: hidden;
  display: flex;
}

.progress-percent {
  min-width: 44px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: v-bind(progressColor);
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

</style>
