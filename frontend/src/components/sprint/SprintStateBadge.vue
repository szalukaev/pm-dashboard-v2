<template>
  <span class="sprint-state" :class="state">{{ $t('sprint.state.' + state) }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Sprint } from '../../stores/sprint'

const props = defineProps<{ sprint: Pick<Sprint, 'status' | 'due_date'> }>()

// Что пользователь видит как статус: спринт закрыт, либо его срок прошёл,
// либо он открыт.
const state = computed(() => {
  if (props.sprint.status === 'closed') return 'closed'
  const due = (props.sprint.due_date || '').slice(0, 10)
  return due && due < new Date().toISOString().slice(0, 10) ? 'overdue' : 'open'
})
</script>

<style scoped>
.sprint-state {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0;
  border-radius: 9999px;
  white-space: nowrap;
  vertical-align: middle;
}

.sprint-state.open {
  color: var(--success);
  background: color-mix(in srgb, var(--success) 14%, transparent);
}

.sprint-state.overdue {
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 14%, transparent);
}

.sprint-state.closed {
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 14%, transparent);
}
</style>
