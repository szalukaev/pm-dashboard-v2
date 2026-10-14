<template>
  <div class="login-countdown">
    <div class="flap">
      <template v-for="part in parts" :key="part.unit">
        <div class="digits">
          <span v-for="(digit, i) in part.value" :key="part.unit + i + digit" class="digit">{{ digit }}</span>
        </div>
        <span class="unit">{{ $t('auth.countdown.' + part.unit) }}</span>
      </template>
    </div>
    <div class="caption">{{ $t('auth.countdown.caption') }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

// Отсчёт до конца рабочей недели (пятница, 18:00 по местному времени).
// До входа неизвестно, в каком спринте пользователь, поэтому берём неделю.

const now = ref(Date.now())

function weekEnd(from: Date): Date {
  const end = new Date(from)
  end.setDate(from.getDate() + ((5 - from.getDay() + 7) % 7))
  end.setHours(18, 0, 0, 0)
  if (end.getTime() <= from.getTime()) end.setDate(end.getDate() + 7)
  return end
}

const parts = computed(() => {
  const left = Math.max(0, weekEnd(new Date(now.value)).getTime() - now.value)
  const minutes = Math.floor(left / 60000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return [
    { unit: 'days', value: pad(Math.floor(minutes / 1440)) },
    { unit: 'hours', value: pad(Math.floor(minutes / 60) % 24) },
    { unit: 'minutes', value: pad(minutes % 60) },
  ]
})

let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 1000) })
onBeforeUnmount(() => clearInterval(timer))
</script>

<style scoped>
.login-countdown {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.flap {
  display: flex;
  align-items: flex-end;
}

.digits {
  display: flex;
  gap: 5px;
  perspective: 400px;
}

.digit {
  position: relative;
  width: 46px;
  height: 64px;
  display: grid;
  place-items: center;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 42px;
  font-weight: 800;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
  box-shadow: 0 8px 16px -8px rgba(0, 0, 0, 0.45);
  animation: flip 0.5s;
}

/* Линия сгиба, как у перекидного табло */
.digit::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 2px;
  background: var(--bg);
}

@keyframes flip {
  0% { transform: rotateX(90deg); }
  100% { transform: rotateX(0); }
}

.unit {
  margin: 0 14px 6px 6px;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.unit:last-child {
  margin-right: 0;
}

.caption {
  font-size: 12px;
  color: var(--text-muted);
}

@media (prefers-reduced-motion: reduce) {
  .digit { animation: none; }
}
</style>
