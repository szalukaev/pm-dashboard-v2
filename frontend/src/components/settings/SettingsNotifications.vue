<template>
  <div class="settings-notifications">
    <h3 class="section-title">{{ $t('settings.notifications.title') }}</h3>

    <div class="setting-block">
      <label class="toggle-row">
        <span class="toggle-label">{{ $t('settings.notifications.toast') }}</span>
        <input
          type="checkbox"
          v-model="notificationsEnabled"
          @change="save"
          class="toggle-input"
        />
      </label>
    </div>

    <div class="setting-block">
      <label class="toggle-row">
        <span class="toggle-label">{{ $t('settings.notifications.overdue_alerts') }}</span>
        <input
          type="checkbox"
          v-model="overdueAlerts"
          @change="save"
          class="toggle-input"
        />
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings'

const settingsStore = useSettingsStore()
const notificationsEnabled = ref(true)
const overdueAlerts = ref(false)

onMounted(() => {
  notificationsEnabled.value = settingsStore.settings.notifications_enabled
})

async function save() {
  await settingsStore.updateSettings({
    notifications_enabled: notificationsEnabled.value,
  } as any)
}
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
</style>
