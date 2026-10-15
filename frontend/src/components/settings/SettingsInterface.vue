<template>
  <div class="settings-interface">
    <h3 class="section-title">{{ $t('settings.interface.title') }}</h3>

    <!-- Тема -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.theme') }}</h4>
      <div class="mode-switch">
        <button class="mode-btn" :class="{ active: modePreference === 'system' }" @click="changeMode('system')" data-testid="mode-system">
          <Monitor :size="14" /> {{ $t('settings.interface.mode_system') }}
        </button>
        <button class="mode-btn" :class="{ active: modePreference === 'dark' }" @click="changeMode('dark')" data-testid="mode-dark">
          <Moon :size="14" /> {{ $t('settings.interface.mode_dark') }}
        </button>
        <button class="mode-btn" :class="{ active: modePreference === 'light' }" @click="changeMode('light')" data-testid="mode-light">
          <Sun :size="14" /> {{ $t('settings.interface.mode_light') }}
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
            <span class="theme-desc">{{ $t('settings.interface.themes.' + t.id) }}</span>
          </span>
        </button>
      </div>
    </div>

    <!-- Язык -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.language') }}</h4>
      <select v-model="currentLang" @change="changeLanguage" class="lang-select">
        <option value="ru">Русский</option>
        <option value="en">English</option>
      </select>
    </div>

    <!-- Часовой пояс: в нём показывается время на всех экранах -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.interface.timezone') }}</h4>
      <select v-model="currentZone" @change="changeTimeZone" class="lang-select" data-testid="timezone-select">
        <option value="">{{ $t('settings.interface.timezone_default', { zone: defaultZone }) }}</option>
        <option v-for="zone in zones" :key="zone" :value="zone">{{ zone }}</option>
      </select>
      <p class="block-hint">{{ $t('settings.interface.timezone_hint', { zone: settings.server_timezone || '—' }) }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { browserTimeZone, timeZoneList } from '../../utils/format'
import { Moon, Sun, Monitor } from 'lucide-vue-next'
import { useTheme, THEMES, type ThemeName, type ModePreference } from '../../composables/useTheme'
import { useSettingsStore } from '../../stores/settings'

const { currentTheme, modePreference, setTheme, setMode } = useTheme()
const { locale } = useI18n()
const settingsStore = useSettingsStore()

const currentLang = ref(locale.value)

const { settings } = storeToRefs(settingsStore)
const currentZone = ref('')
// Пояс, действующий без своего выбора: от администратора или пояс браузера
const defaultZone = computed(() => settings.value.default_timezone || browserTimeZone())
// Сохранённый пояс остаётся в списке, даже если браузер его не знает
const zones = computed(() => {
  const all = timeZoneList()
  return currentZone.value && !all.includes(currentZone.value) ? [currentZone.value, ...all] : all
})

onMounted(async () => {
  await settingsStore.ensureLoaded()
  // Загрузка настроек могла сменить язык на сохранённый
  currentLang.value = locale.value
  currentZone.value = settings.value.timezone || ''
})

async function changeTimeZone() {
  await settingsStore.updateSettings({ timezone: currentZone.value })
}

// На сервере хранится в виде "<тема>-<режим>", например "nord-dark" или "nord-system".
function persist() {
  return settingsStore.updateSettings({ theme: `${currentTheme.value}-${modePreference.value}` } as any)
}

async function changeTheme(theme: ThemeName) {
  setTheme(theme)
  await persist()
}

async function changeMode(mode: ModePreference) {
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

.block-hint {
  max-width: 560px;
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-muted);
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
  border-radius: 12px;
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

.lang-select:focus-visible {
  border-color: var(--accent);
}
</style>
