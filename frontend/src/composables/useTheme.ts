import { ref } from 'vue'

export type ThemeName = 'nord' | 'amber' | 'forest' | 'dusk'
export type ThemeMode = 'dark' | 'light'

export const THEMES: { id: ThemeName; name: string; description: string; preview: { accent: string; bg: string; surface: string } }[] = [
  { id: 'nord', name: 'Nord', description: 'Арктическая сдержанность, холодные сине-серые тона', preview: { accent: '#5e9ab0', bg: '#1a1d23', surface: '#272b34' } },
  { id: 'amber', name: 'Amber', description: 'Тёплый янтарно-медный, глубокие насыщенные тона', preview: { accent: '#c8942a', bg: '#1c1710', surface: '#2c251c' } },
  { id: 'forest', name: 'Forest', description: 'Глубокий зелёный, натуральные земляные тона', preview: { accent: '#5a9a68', bg: '#171c1a', surface: '#242b28' } },
  { id: 'dusk', name: 'Dusk', description: 'Сумеречный фиолетово-янтарный, тёплая прохлада', preview: { accent: '#8a6eb0', bg: '#1a1920', surface: '#272630' } },
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

// Earlier versions stored a single value: 'dark', 'light' or a theme name.
function initialState(): { theme: ThemeName; mode: ThemeMode } {
  const stored = readStorage(THEME_KEY) || ''
  const storedMode = readStorage(MODE_KEY)
  const theme = (themeIds.includes(stored) ? stored : 'nord') as ThemeName
  const mode: ThemeMode = storedMode === 'light' || storedMode === 'dark'
    ? storedMode
    : stored === 'light' ? 'light' : 'dark'
  return { theme, mode }
}

const initial = initialState()
const currentTheme = ref<ThemeName>(initial.theme)
const currentMode = ref<ThemeMode>(initial.mode)

function apply() {
  const root = document.documentElement
  root.setAttribute('data-theme', currentTheme.value)
  root.setAttribute('data-mode', currentMode.value)
}

export function useTheme() {
  function setTheme(theme: ThemeName) {
    currentTheme.value = theme
    writeStorage(THEME_KEY, theme)
    apply()
  }

  function setMode(mode: ThemeMode) {
    currentMode.value = mode
    writeStorage(MODE_KEY, mode)
    apply()
  }

  function toggleMode() {
    setMode(currentMode.value === 'dark' ? 'light' : 'dark')
  }

  apply()

  return { currentTheme, currentMode, setTheme, setMode, toggleMode }
}
