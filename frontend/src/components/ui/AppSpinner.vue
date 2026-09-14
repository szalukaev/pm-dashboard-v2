<template>
  <div class="spinner" :style="{ width: size + 'px', height: size + 'px' }"></div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 24 })
</script>

<style scoped>
.spinner {
  border: 2px solid var(--hairline);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spinner-rotate 0.8s linear infinite;
}

@keyframes spinner-rotate {
  to {
    transform: rotate(360deg);
  }
}
</style>
