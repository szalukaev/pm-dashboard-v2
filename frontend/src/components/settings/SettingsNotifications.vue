<template>
  <div class="settings-notifications">
    <h3 class="section-title">{{ $t('settings.notifications.title') }}</h3>

    <div class="setting-block">
      <label class="toggle-row">
        <span class="toggle-label">{{ $t('settings.notifications.toast') }}</span>
        <input
          type="checkbox"
          v-model="notificationsEnabled"
          @change="saveToast"
          class="toggle-input"
        />
      </label>
    </div>

    <!-- Правила: о чём сообщать и куда -->
    <h4 class="block-title">{{ $t('notifications.rules_title') }}</h4>
    <p class="block-hint">{{ $t('notifications.rules_hint') }}</p>

    <!-- Личные правила работают для того, кто связан с сотрудником Redmine -->
    <div v-if="!linked" class="warning-note" data-testid="alert-rules-not-linked">
      {{ $t('notifications.not_linked') }}
    </div>

    <div class="rules" data-testid="alert-rules">
      <template v-for="group in GROUPS" :key="group.key">
        <div class="rule-group">{{ $t('notifications.groups.' + group.key) }}</div>
        <div v-for="key in group.rules" :key="key" class="rule-row" :data-testid="'alert-rule-' + key">
          <div class="rule-name">
            {{ $t('notifications.rules.' + key + '.name') }}
            <div class="rule-hint">{{ $t('notifications.rules.' + key + '.hint') }}</div>
          </div>
          <label v-if="DAYS[key]" class="rule-days">
            {{ $t('notifications.days') }}
            <input
              type="number"
              :min="DAYS[key].min"
              :max="DAYS[key].max"
              :value="rule(key).days ?? 0"
              @change="setDays(key, $event)"
            />
          </label>
          <label class="rule-check">
            <input type="checkbox" :checked="rule(key).bell" @change="set(key, { bell: !rule(key).bell })" />
            {{ $t('notifications.to_bell') }}
          </label>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings'
import type { AlertRule } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'

// Правила по группам, в порядке показа. Подписи: notifications.rules.<правило>
const GROUPS: { key: string; rules: string[] }[] = [
  { key: 'tasks', rules: ['task_assigned', 'task_changed', 'task_due_soon', 'task_overdue'] },
]

// Правила с порогом в днях и его пределы — те же, что проверяет сервер
const DAYS: Record<string, { min: number; max: number }> = {
  task_due_soon: { min: 0, max: 60 },
}

const settingsStore = useSettingsStore()
const auth = useAuthStore()
const notificationsEnabled = ref(true)

const linked = computed(() => !!auth.user?.source?.member_id)

function rule(key: string): AlertRule {
  return settingsStore.settings.alert_rules?.[key] || { bell: false, email: false }
}

// Каждое изменение сохраняется сразу
function set(key: string, change: Partial<AlertRule>) {
  settingsStore.setAlertRule(key, { ...rule(key), ...change })
}

function setDays(key: string, e: Event) {
  const input = e.target as HTMLInputElement
  const limits = DAYS[key]
  const days = Math.min(limits.max, Math.max(limits.min, Math.round(Number(input.value) || 0)))
  input.value = String(days)
  set(key, { days })
}

async function saveToast() {
  await settingsStore.updateSettings({ notifications_enabled: notificationsEnabled.value })
}

onMounted(async () => {
  await settingsStore.ensureLoaded()
  notificationsEnabled.value = settingsStore.settings.notifications_enabled
})
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 20px;
}

.setting-block {
  margin-bottom: 16px;
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  cursor: pointer;
}

.toggle-label {
  font-size: 13px;
  color: var(--text);
}

.toggle-input {
  width: 20px;
  height: 20px;
  accent-color: var(--accent);
}

.block-title {
  margin: 28px 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.block-hint {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-muted);
}

.warning-note {
  margin-bottom: 12px;
  padding: 10px 14px;
  border: 1px solid var(--warning);
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
}

.rules {
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
}

.rule-group {
  padding: 8px 16px;
  background: var(--surface-2);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.rule-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-light);
}

.rule-name {
  flex: 1;
  min-width: 220px;
  font-size: 13px;
  color: var(--text);
}

.rule-hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-muted);
}

.rule-days,
.rule-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  cursor: pointer;
}

.rule-days input {
  width: 64px;
  padding: 6px 8px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  font-size: 13px;
  color: var(--text);
}

.rule-check input {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}
</style>
