<template>
  <!-- Период отчётов «Аналитики»: готовые варианты и свой диапазон дат -->
  <div class="period" data-testid="analytics-period">
    <div class="period-modes" role="group" :aria-label="$t('analytics.period.label')">
      <button
        v-for="mode in PERIOD_MODES"
        :key="mode"
        type="button"
        class="period-mode"
        :class="{ active: modelValue.mode === mode }"
        :data-testid="'period-' + mode"
        @click="setMode(mode)"
      >
        {{ $t('analytics.period.modes.' + modeKey(mode)) }}
      </button>
    </div>

    <div v-if="modelValue.mode === 'range'" class="period-range">
      <input
        type="date"
        class="period-date"
        :value="modelValue.from"
        :max="modelValue.to || undefined"
        :aria-label="$t('analytics.period.from')"
        data-testid="period-from"
        @change="setDate('from', ($event.target as HTMLInputElement).value)"
      />
      <span class="period-dash">—</span>
      <input
        type="date"
        class="period-date"
        :value="modelValue.to"
        :min="modelValue.from || undefined"
        :aria-label="$t('analytics.period.to')"
        data-testid="period-to"
        @change="setDate('to', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <AppHint :text="$t('analytics.period.hint')" />
  </div>
</template>

<script setup lang="ts">
import AppHint from '../ui/AppHint.vue'
import { PERIOD_MODES, type AnalyticsPeriod, type PeriodMode } from '../../stores/analytics'

const props = defineProps<{ modelValue: AnalyticsPeriod }>()
const emit = defineEmits(['update:modelValue'])

// Имя подписи в словаре: у вариантов «N дней» оно начинается с буквы
function modeKey(mode: PeriodMode): string {
  return mode === 'all' || mode === 'range' ? mode : 'days' + mode
}

function setMode(mode: PeriodMode) {
  if (mode !== props.modelValue.mode) emit('update:modelValue', { ...props.modelValue, mode })
}

// Перепутанные границы меняются местами: в полях всегда тот период, по
// которому построен отчёт (раньше поле показывало непринятую дату)
function setDate(edge: 'from' | 'to', value: string) {
  const next = { ...props.modelValue, [edge]: value }
  if (next.from && next.to && next.from > next.to) {
    [next.from, next.to] = [next.to, next.from]
  }
  emit('update:modelValue', next)
}
</script>

<style scoped>
.period {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.period-modes {
  display: inline-flex;
  gap: 2px;
  padding: 3px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 9999px;
}

.period-mode {
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  color: var(--text-muted);
  border-radius: 9999px;
  transition: all 0.15s;
}

.period-mode:hover {
  color: var(--text-bright);
}

.period-mode.active {
  color: #fff;
  background: var(--accent);
}

.period-range {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.period-date {
  padding: 5px 8px;
  font-size: 12px;
  color: var(--text);
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
}

.period-date:focus-visible {
  border-color: var(--accent);
}

.period-dash {
  color: var(--text-muted);
}

/* На телефоне переключатель — во всю ширину, кнопки поровну и выше под палец;
   длинная подпись переносится, а не уходит за край */
@media (max-width: 767px) {
  .period,
  .period-modes {
    width: 100%;
  }

  .period-mode {
    flex: 1 1 0;
    min-width: 0;
    min-height: 36px;
    padding: 4px 6px;
    white-space: normal;
    line-height: 1.2;
  }

  .period-range {
    width: 100%;
  }

  .period-date {
    flex: 1;
    min-width: 0;
    min-height: 36px;
  }
}
</style>
