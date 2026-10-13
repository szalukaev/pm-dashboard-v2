<template>
  <div class="thresholds" data-testid="thresholds">
    <h4 class="block-title">{{ $t('thresholds.title') }} <AppHint help :text="$t('hints.thresholds')" /></h4>
    <p class="block-hint">{{ $t('thresholds.hint') }}</p>

    <div class="threshold-rows">
      <label v-for="key in INDICATORS" :key="key" class="threshold-row">
        <span class="threshold-name">{{ $t('analytics.stats.' + key) }}</span>
        <span class="threshold-more">{{ $t('thresholds.more_than') }}</span>
        <input
          type="number"
          min="0"
          :max="MAX"
          :value="form[key]"
          :data-testid="'threshold-' + key"
          @change="setLimit(key, $event)"
        />
        <span class="threshold-more">{{ $t('thresholds.tasks') }}</span>
      </label>
    </div>

    <!-- С чем сравнивать порог: со всеми проектами сразу или с каждым -->
    <div class="threshold-scope">
      <label class="threshold-option">
        <input type="radio" :checked="!form.per_project" @change="setScope(false)" />
        {{ $t('thresholds.all_projects') }}
      </label>
      <label class="threshold-option">
        <input type="radio" :checked="form.per_project" data-testid="threshold-per-project" @change="setScope(true)" />
        {{ $t('thresholds.per_project') }}
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import AppHint from '../ui/AppHint.vue'
import { useSettingsStore } from '../../stores/settings'
import type { IndicatorThresholds } from '../../stores/settings'

// Индикаторы с порогом; подписи — те же, что у карточек статистики
const INDICATORS = ['overdue', 'bugs', 'no_estimate'] as const
// Наибольший порог — тот же, что проверяет сервер
const MAX = 100000

const settingsStore = useSettingsStore()
const form = reactive<IndicatorThresholds>({ overdue: 5, bugs: 10, no_estimate: 3, per_project: false })

// Каждое изменение сохраняется сразу
function save() {
  settingsStore.updateSettings({ indicator_thresholds: { ...form } })
}

function setLimit(key: (typeof INDICATORS)[number], e: Event) {
  const input = e.target as HTMLInputElement
  const limit = Math.min(MAX, Math.max(0, Math.round(Number(input.value) || 0)))
  input.value = String(limit)
  form[key] = limit
  save()
}

function setScope(perProject: boolean) {
  form.per_project = perProject
  save()
}

onMounted(async () => {
  await settingsStore.ensureLoaded()
  Object.assign(form, settingsStore.settings.indicator_thresholds)
})
</script>

<style scoped>
.block-title {
  margin: 32px 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.block-hint {
  margin: 0 0 14px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-muted);
}

.threshold-rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.threshold-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
  font-size: 13px;
  color: var(--text);
}

.threshold-name {
  min-width: 140px;
}

.threshold-more {
  font-size: 12px;
  color: var(--text-muted);
}

.threshold-row input {
  width: 90px;
  padding: 6px 8px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  font-size: 13px;
  color: var(--text);
}

.threshold-scope {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 16px;
}

.threshold-option {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}
</style>
