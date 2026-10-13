<template>
  <div class="health">
    <div v-if="!health" class="empty-hint">{{ $t('common.empty') }}</div>

    <template v-else>
      <div class="gauge" :class="'level-' + health.level" data-testid="health-gauge">
        <!-- Полукруг: дорожка и поверх неё часть, которую заполняет оценка -->
        <svg viewBox="0 0 200 110" class="gauge-svg" role="img" :aria-label="$t('analytics.health') + ': ' + health.score">
          <path :d="ARC" class="gauge-track" />
          <path :d="ARC" class="gauge-value" pathLength="100" :stroke-dasharray="health.score + ' 100'" />
        </svg>
        <div class="gauge-center">
          <div class="gauge-score" data-testid="health-score">{{ health.score }}</div>
          <div class="gauge-level">{{ $t('analytics.health_levels.' + health.level) }}</div>
        </div>
      </div>

      <div class="health-active">{{ $t('analytics.health_active', { n: health.active }) }}</div>

      <!-- Что снижает оценку и насколько -->
      <ul class="parts">
        <li v-for="part in health.parts" :key="part.key" class="part">
          <span class="part-name">{{ $t('analytics.health_parts.' + part.key) }}</span>
          <span class="part-count">{{ $t('analytics.health_share', { count: part.count, pct: part.share }) }}</span>
          <span class="part-penalty" :class="{ lost: part.penalty > 0 }">{{ part.penalty > 0 ? '−' + part.penalty : '0' }}</span>
        </li>
      </ul>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { Health } from '../../stores/analytics'

defineProps<{ health: Health | null }>()

// Полукруг от левого конца к правому через верх
const ARC = 'M 14 100 A 86 86 0 0 1 186 100'
</script>

<style scoped>
.empty-hint {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  padding: 24px;
}

.gauge {
  position: relative;
  max-width: 260px;
  margin: 0 auto;
}

.gauge-svg {
  display: block;
  width: 100%;
}

.gauge-track,
.gauge-value {
  fill: none;
  stroke-width: 16;
  stroke-linecap: round;
}

.gauge-track {
  stroke: var(--surface-2);
}

.gauge-value {
  transition: stroke-dasharray 0.4s;
}

.level-good .gauge-value { stroke: var(--success); }
.level-warning .gauge-value { stroke: var(--warning); }
.level-bad .gauge-value { stroke: var(--danger); }

.gauge-center {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  text-align: center;
}

.gauge-score {
  font-size: 40px;
  font-weight: 600;
  line-height: 1;
  color: var(--text-bright);
  letter-spacing: -1px;
  font-variant-numeric: tabular-nums;
}

.gauge-level {
  margin-top: 4px;
  font-size: 12px;
  font-weight: 500;
}

.level-good .gauge-level { color: var(--success); }
.level-warning .gauge-level { color: var(--warning); }
.level-bad .gauge-level { color: var(--danger); }

.health-active {
  margin-top: 12px;
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
}

.parts {
  list-style: none;
  margin: 16px 0 0;
  padding: 0;
}

.part {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 7px 0;
  border-top: 1px solid var(--border-light);
  font-size: 13px;
}

.part-name {
  flex: 1;
  min-width: 0;
  color: var(--text);
}

.part-count {
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.part-penalty {
  min-width: 40px;
  text-align: right;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.part-penalty.lost {
  color: var(--danger);
  font-weight: 500;
}
</style>
