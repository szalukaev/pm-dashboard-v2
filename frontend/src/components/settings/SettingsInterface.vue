<template>
  <div class="settings-interface">
    <h3 class="section-title">{{ $t('settings.interface.title') }}</h3>

    <!-- Theme -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.theme') }}</h4>
      <div class="theme-grid">
        <button
          v-for="t in themes"
          :key="t.value"
          class="theme-option"
          :class="{ active: currentTheme === t.value }"
          @click="changeTheme(t.value)"
          :data-testid="'theme-' + t.value"
        >
          <div class="theme-preview" :style="{ background: t.preview }"></div>
          <span class="theme-name">{{ t.label }}</span>
        </button>
      </div>
    </div>

    <!-- Language -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.language') }}</h4>
      <select v-model="currentLang" @change="changeLanguage" class="lang-select">
        <option value="ru">Русский</option>
        <option value="en">English</option>
      </select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTheme } from '../../composables/useTheme'
import { useSettingsStore } from '../../stores/settings'

const { currentTheme, setTheme } = useTheme()
const { locale } = useI18n()
const settingsStore = useSettingsStore()

const currentLang = ref(locale.value)

const themes = [
  { value: 'dark', label: 'Dark', preview: '#010102' },
  { value: 'light', label: 'Light', preview: '#ffffff' },
  { value: 'nord', label: 'Nord', preview: '#2e3440' },
  { value: 'amber', label: 'Amber', preview: '#1a1814' },
  { value: 'forest', label: 'Forest', preview: '#0a120d' },
  { value: 'dusk', label: 'Dusk', preview: '#0d0a14' },
]

async function changeTheme(theme: string) {
  setTheme(theme)
  await settingsStore.updateSettings({ theme } as any)
}

async function changeLanguage() {
  locale.value = currentLang.value
  localStorage.setItem('pm-dashboard-lang', currentLang.value)
  await settingsStore.updateSettings({ language: currentLang.value } as any)
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
  margin-bottom: 24px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
}

.theme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
  gap: 10px;
}

.theme-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: var(--surface-2);
  border: 2px solid transparent;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;
}

.theme-option:hover {
  border-color: var(--hairline-strong);
}

.theme-option.active {
  border-color: var(--accent);
}

.theme-preview {
  width: 48px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--hairline);
}

.theme-name {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
}

.lang-select {
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  min-width: 200px;
}

.lang-select:focus {
  border-color: var(--accent);
}
</style>
