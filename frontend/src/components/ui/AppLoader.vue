<template>
  <!-- Загрузка вкладки: три карточки перескакивают из колонки в колонку,
       как задачи по статусам на канбане -->
  <div class="app-loader" role="status" aria-live="polite" data-testid="app-loader">
    <div class="board" aria-hidden="true">
      <div class="column">
        <i class="card c1"></i>
        <i class="card c2"></i>
        <i class="card c3"></i>
      </div>
      <div class="column"></div>
      <div class="column"></div>
    </div>
    <div class="caption">{{ text || $t('common.loader.default') }}<span class="dots"></span></div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ text?: string }>()
</script>

<style scoped>
.app-loader {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  min-height: 260px;
  padding: 40px 16px;
}

.board {
  display: flex;
  gap: 8px;
}

.column {
  position: relative;
  width: 34px;
  height: 58px;
  border: 1px dashed var(--hairline-strong);
  border-radius: 6px;
}

/* Карточки лежат в первой колонке и уезжают поверх соседних */
.column:first-child {
  z-index: 1;
}

.card {
  position: absolute;
  left: 3px;
  right: 3px;
  height: 14px;
  border-radius: 3px;
  background: var(--accent);
  animation: hop 2.4s ease-in-out infinite;
}

.c2 {
  background: var(--warning);
  animation-delay: -0.8s;
}

.c3 {
  background: var(--success);
  animation-delay: -1.6s;
}

/* Шаг колонки — 34 px ширины + 8 px зазора */
@keyframes hop {
  0%, 20% { top: 4px; transform: translateX(0); }
  33%, 53% { top: 22px; transform: translateX(42px); }
  66%, 86% { top: 40px; transform: translateX(84px); }
  100% { top: 4px; transform: translateX(0); }
}

.caption {
  font-size: 13px;
  color: var(--text-muted);
}

.dots::after {
  content: '';
  animation: dots 1.2s steps(4) infinite;
}

@keyframes dots {
  0% { content: ''; }
  25% { content: '.'; }
  50% { content: '..'; }
  75% { content: '...'; }
}

@media (prefers-reduced-motion: reduce) {
  .card,
  .dots::after {
    animation: none;
  }
}
</style>
