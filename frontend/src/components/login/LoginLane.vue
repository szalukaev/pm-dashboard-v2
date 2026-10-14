<template>
  <div class="login-lane" aria-hidden="true">
    <div class="lane-head">
      <span>{{ column }}</span>
      <span>{{ titles.length }}</span>
    </div>
    <div class="lane-cards">
      <!-- Список повторён дважды: лента сдвигается на половину и бесшовно начинается заново -->
      <div v-for="(title, i) in [...titles, ...titles]" :key="i" class="lane-card">
        <i>#{{ 4511 + (i % titles.length) * 3 }}</i>{{ title }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// Мобильная замена доски на экране входа: одна колонка «В работе»,
// вымышленные задачи медленно плывут вверх.

const { tm, rt } = useI18n()
const column = rt((tm('auth.board.columns') as unknown[])[1] as any)
const titles = (tm('auth.board.tasks') as unknown[]).map(c => rt(c as any))
</script>

<style scoped>
.login-lane {
  position: relative;
  flex: 1;
  min-height: 0;
  padding: 24px 18px 0;
  overflow: hidden;
  pointer-events: none;
  user-select: none;
}

/* Лента растворяется у шторки */
.login-lane::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 90px;
  background: linear-gradient(transparent, var(--bg));
}

.lane-head {
  display: flex;
  justify-content: space-between;
  margin-bottom: 10px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.lane-cards {
  display: flex;
  flex-direction: column;
  gap: 9px;
  animation: lane-up 40s linear infinite;
}

@keyframes lane-up {
  to { transform: translateY(-50%); }
}

.lane-card {
  padding: 11px 13px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 10px;
  font-size: 13px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.lane-card i {
  font-style: normal;
  font-weight: 600;
  color: var(--accent);
  margin-right: 6px;
}

@media (prefers-reduced-motion: reduce) {
  .lane-cards { animation: none; }
}
</style>
