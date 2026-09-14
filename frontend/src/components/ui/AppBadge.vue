<template>
  <span class="badge" :class="color" :data-testid="testId">
    <slot />
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  color?: 'default' | 'success' | 'danger' | 'warning' | 'info' | 'purple' | 'orange' | 'cyan'
  testId?: string
}>(), {
  color: 'default'
})
</script>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
  white-space: nowrap;
  background: var(--tag-bg);
  color: var(--text-muted);
}

.success {
  background: var(--success)22;
  color: var(--success);
}

.danger {
  background: var(--danger)22;
  color: var(--danger);
}

.warning {
  background: var(--warning)22;
  color: var(--warning);
}

.info {
  background: var(--accent-bg);
  color: var(--accent);
}

.purple {
  background: var(--purple)22;
  color: var(--purple);
}

.orange {
  background: var(--orange)22;
  color: var(--orange);
}

.cyan {
  background: var(--cyan)22;
  color: var(--cyan);
}
</style>
