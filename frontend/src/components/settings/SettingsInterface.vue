<template>
  <div class="settings-interface">
    <h3 class="section-title">{{ $t('settings.interface.title') }}</h3>

    <!-- Theme -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.theme') }}</h4>
      <div class="mode-switch">
        <button class="mode-btn" :class="{ active: currentMode === 'dark' }" @click="changeMode('dark')" data-testid="mode-dark">
          <Moon :size="14" /> Тёмная
        </button>
        <button class="mode-btn" :class="{ active: currentMode === 'light' }" @click="changeMode('light')" data-testid="mode-light">
          <Sun :size="14" /> Светлая
        </button>
      </div>
      <div class="theme-grid">
        <button
          v-for="t in THEMES"
          :key="t.id"
          class="theme-option"
          :class="{ active: currentTheme === t.id }"
          @click="changeTheme(t.id)"
          :data-testid="'theme-' + t.id"
        >
          <span class="theme-swatches">
            <span class="swatch" :style="{ background: t.preview.accent }"></span>
            <span class="swatch" :style="{ background: t.preview.bg }"></span>
            <span class="swatch" :style="{ background: t.preview.surface }"></span>
          </span>
          <span class="theme-text">
            <span class="theme-name">{{ t.name }}</span>
            <span class="theme-desc">{{ t.description }}</span>
          </span>
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
import { Moon, Sun } from 'lucide-vue-next'
import { useTheme, THEMES, type ThemeName, type ThemeMode } from '../../composables/useTheme'
import { useSettingsStore } from '../../stores/settings'

const { currentTheme, currentMode, setTheme, setMode } = useTheme()
const { locale } = useI18n()
const settingsStore = useSettingsStore()

const currentLang = ref(locale.value)

// Stored on the server as "<theme>-<mode>", e.g. "nord-dark".
function persist() {
  return settingsStore.updateSettings({ theme: `${currentTheme.value}-${currentMode.value}` } as any)
}

async function changeTheme(theme: ThemeName) {
  setTheme(theme)
  await persist()
}

async function changeMode(mode: ThemeMode) {
  setMode(mode)
  await persist()
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

.mode-switch {
  display: inline-flex;
  gap: 4px;
  padding: 3px;
  margin-bottom: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 9999px;
}

.mode-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-radius: 9999px;
  cursor: pointer;
  transition: all 0.15s;
}

.mode-btn.active {
  background: var(--accent);
  color: #fff;
}

.theme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 10px;
}

.theme-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  text-align: left;
  background: var(--surface-2);
  border: 2px solid var(--hairline);
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

.theme-swatches {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.swatch {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, .1);
}

.theme-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.theme-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.theme-desc {
  font-size: 11px;
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
