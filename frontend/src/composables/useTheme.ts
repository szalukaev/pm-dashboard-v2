import { ref, computed } from 'vue'

export type ThemeName = 'nord' | 'amber' | 'forest' | 'dusk'
export type ThemeMode = 'dark' | 'light'
// Выбор пользователя: «как в системе» или явный режим
export type ModePreference = ThemeMode | 'system'

// Описание темы — в словаре: settings.interface.themes.<id>
export const THEMES: { id: ThemeName; name: string; preview: { accent: string; bg: string; surface: string } }[] = [
  { id: 'nord', name: 'Nord', preview: { accent: '#5e9ab0', bg: '#1a1d23', surface: '#272b34' } },
  { id: 'amber', name: 'Amber', preview: { accent: '#c8942a', bg: '#1c1710', surface: '#2c251c' } },
  { id: 'forest', name: 'Forest', preview: { accent: '#5a9a68', bg: '#171c1a', surface: '#242b28' } },
  { id: 'dusk', name: 'Dusk', preview: { accent: '#8a6eb0', bg: '#1a1920', surface: '#272630' } },
]

const THEME_KEY = 'pm-dashboard-theme'
const MODE_KEY = 'pm-dashboard-mode'
const themeIds = THEMES.map(t => t.id) as string[]

function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStorage(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {}
}

// Прежние версии хранили одно значение: 'dark', 'light' или название темы.
// Без сохранённого выбора режим берётся из системы пользователя.
function initialState(): { theme: ThemeName; pref: ModePreference } {
  const stored = readStorage(THEME_KEY) || ''
  const storedMode = readStorage(MODE_KEY)
  const theme = (themeIds.includes(stored) ? stored : 'nord') as ThemeName
  const pref: ModePreference = storedMode === 'light' || storedMode === 'dark' || storedMode === 'system'
    ? storedMode
    : stored === 'light' || stored === 'dark' ? stored : 'system'
  return { theme, pref }
}

const systemQuery = window.matchMedia('(prefers-color-scheme: dark)')
const systemMode = ref<ThemeMode>(systemQuery.matches ? 'dark' : 'light')

const initial = initialState()
const currentTheme = ref<ThemeName>(initial.theme)
const modePreference = ref<ModePreference>(initial.pref)
// Действующий режим: при «Системной» — тот, что сейчас в системе
const currentMode = computed<ThemeMode>(() => modePreference.value === 'system' ? systemMode.value : modePreference.value)

function apply() {
  const root = document.documentElement
  root.setAttribute('data-theme', currentTheme.value)
  root.setAttribute('data-mode', currentMode.value)
}

// Тема ставится сразу при загрузке, ещё до экрана входа
apply()

// Система переключилась (например, вечером) — при «Системной» тема меняется сразу
systemQuery.addEventListener('change', e => {
  systemMode.value = e.matches ? 'dark' : 'light'
  apply()
})

export function useTheme() {
  function setTheme(theme: ThemeName) {
    currentTheme.value = theme
    writeStorage(THEME_KEY, theme)
    apply()
  }

  function setMode(mode: ModePreference) {
    modePreference.value = mode
    writeStorage(MODE_KEY, mode)
    apply()
  }

  function toggleMode() {
    setMode(currentMode.value === 'dark' ? 'light' : 'dark')
  }

  apply()

  return { currentTheme, currentMode, modePreference, setTheme, setMode, toggleMode }
}
