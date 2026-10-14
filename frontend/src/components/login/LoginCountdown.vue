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
    <div class="caption">{{ mode === 'sprint' ? $t('auth.countdown.caption_sprint', { name: sprintName }) : $t('auth.countdown.caption_week') }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

// Отсчёт до конца рабочей недели или до конца спринта — выбирается случайно
// при каждом открытии экрана. До входа неизвестно, в каком спринте
// пользователь, поэтому «спринт» здесь условный: двухнедельный цикл,
// который заканчивается в пятницу в 18:00 по местному времени.

const now = ref(Date.now())
const mode: 'week' | 'sprint' = Math.random() < 0.5 ? 'week' : 'sprint'
// Название условного спринта — случайное из списка в словаре
const { tm, rt } = useI18n()
const sprintNames = (tm('auth.countdown.sprint_names') as unknown[]).map(n => rt(n as any))
const sprintName = sprintNames[Math.floor(Math.random() * sprintNames.length)]
// Пятница, от которой отсчитываются двухнедельные спринты
const SPRINT_ANCHOR = new Date(2026, 0, 2, 18, 0, 0, 0).getTime()
const WEEK_MS = 7 * 24 * 60 * 60 * 1000

function weekEnd(from: Date): Date {
  const end = new Date(from)
  end.setDate(from.getDate() + ((5 - from.getDay() + 7) % 7))
  end.setHours(18, 0, 0, 0)
  if (end.getTime() <= from.getTime()) end.setDate(end.getDate() + 7)
  return end
}

function sprintEnd(from: Date): Date {
  const end = weekEnd(from)
  const weeks = Math.round((end.getTime() - SPRINT_ANCHOR) / WEEK_MS)
  // Нечётная неделя от опорной пятницы — середина спринта, конец через неделю
  if (Math.abs(weeks) % 2 === 1) end.setDate(end.getDate() + 7)
  return end
}

const parts = computed(() => {
  const left = Math.max(0, (mode === 'sprint' ? sprintEnd : weekEnd)(new Date(now.value)).getTime() - now.value)
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
  gap: 14px;
}

.flap {
  display: flex;
  align-items: flex-end;
}

.digits {
  display: flex;
  gap: 8px;
  perspective: 600px;
}

.digit {
  position: relative;
  width: 92px;
  height: 128px;
  display: grid;
  place-items: center;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  font-size: 84px;
  font-weight: 800;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
  box-shadow: 0 14px 28px -12px rgba(0, 0, 0, 0.45);
  animation: flip 0.5s;
}

/* Линия сгиба, как у перекидного табло */
.digit::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 3px;
  background: var(--bg);
}

@keyframes flip {
  0% { transform: rotateX(90deg); }
  100% { transform: rotateX(0); }
}

.unit {
  margin: 0 24px 10px 10px;
  font-size: 14px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.unit:last-child {
  margin-right: 0;
}

.caption {
  font-size: 15px;
  color: var(--text-muted);
}

/* На узком экране табло возвращается к прежнему размеру */
@media (max-width: 760px) {
  .digits { gap: 5px; }
  .digit { width: 46px; height: 64px; font-size: 42px; border-radius: 8px; }
  .unit { margin: 0 12px 6px 6px; font-size: 11px; }
  .caption { font-size: 12px; }
}

/* Телефон: табло внутри шторки, совсем компактное */
@media (max-width: 700px) {
  .login-countdown { gap: 6px; }
  .digits { gap: 4px; }
  .digit { width: 30px; height: 42px; font-size: 26px; border-radius: 7px; }
  .unit { margin: 0 10px 5px 5px; font-size: 10px; }
}

@media (prefers-reduced-motion: reduce) {
  .digit { animation: none; }
}
</style>
