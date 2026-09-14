<template>
  <div
    class="sync-status"
    :class="status"
    :title="statusText"
    data-testid="sync-status"
  >
    <div class="sync-dot"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useWebSocket } from '../../composables/useWebSocket'

const status = ref<'connected' | 'warning' | 'error'>('warning')

const { connected } = useWebSocket()

onMounted(() => {
  // Update status based on WebSocket connection
  const interval = setInterval(() => {
    status.value = connected.value ? 'connected' : 'warning'
  }, 1000)

  return () => clearInterval(interval)
})

const statusText = 'Синхронизация'
</script>

<style scoped>
.sync-status {
  display: flex;
  align-items: center;
  cursor: help;
}

.sync-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  transition: background 0.3s;
}

.connected .sync-dot {
  background: var(--success);
}

.warning .sync-dot {
  background: var(--warning);
}

.error .sync-dot {
  background: var(--danger);
}
</style>
