<template>
  <div class="stat-card" :class="variant">
    <div class="label">{{ label }}</div>
    <div class="value">{{ displayValue }}</div>
    <div v-if="sub" class="sub">{{ sub }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  label: string
  value: string | number
  formattedValue?: string
  sub?: string
  variant?: 'danger' | 'warning' | 'success' | 'info'
}>()

const displayValue = computed(() => props.formattedValue ?? props.value)
</script>

<style scoped>
.stat-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 20px;
  transition: all 0.15s;
}

.stat-card:hover {
  border-color: var(--hairline-strong);
  transform: translateY(-1px);
}

.label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 6px;
}

.value {
  font-size: 28px;
  font-weight: 600;
  letter-spacing: -0.6px;
  color: var(--text-bright);
}

.sub {
  font-size: 12px;
  color: var(--text-faint);
  margin-top: 4px;
}

.danger .value {
  color: var(--danger);
}

.warning .value {
  color: var(--warning);
}

.success .value {
  color: var(--success);
}
</style>
