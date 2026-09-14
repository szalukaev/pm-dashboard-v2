<template>
  <button
    :class="['app-btn', variant, { disabled: disabled || loading }]"
    :disabled="disabled || loading"
    :data-testid="testId"
    @click="$emit('click', $event)"
  >
    <span v-if="loading" class="btn-spinner"></span>
    <slot v-else />
  </button>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  variant?: 'primary' | 'ghost' | 'danger'
  loading?: boolean
  disabled?: boolean
  testId?: string
}>(), {
  variant: 'primary',
  loading: false,
  disabled: false
})

defineEmits(['click'])
</script>

<style scoped>
.app-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: -0.05px;
  border-radius: 8px;
  transition: all 0.15s;
  white-space: nowrap;
}

.primary {
  background: var(--accent);
  color: #fff;
}

.primary:hover {
  background: var(--accent-hover);
}

.ghost {
  background: transparent;
  border: 1px solid var(--hairline);
  color: var(--text-muted);
}

.ghost:hover {
  border-color: var(--text-faint);
  color: var(--text-bright);
}

.danger {
  background: var(--danger);
  color: #fff;
}

.danger:hover {
  opacity: 0.9;
}

.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: btn-spin 0.8s linear infinite;
}

@keyframes btn-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
