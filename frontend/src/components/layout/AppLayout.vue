<template>
  <div class="app-layout">
    <AppHeader />
    <main class="app-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import AppHeader from './AppHeader.vue'
</script>

<style scoped>
.app-layout {
  display: grid;
  grid-template-rows: 56px 1fr;
  height: 100vh;
  overflow: hidden;
}

.app-content {
  overflow-y: auto;
  padding: 24px;
}

@media (max-width: 768px) {
  .app-content {
    padding: 16px;
  }
}
</style>
