import { ref } from 'vue'

const THEME_KEY = 'pm-dashboard-theme'
const currentTheme = ref<string>(localStorage.getItem(THEME_KEY) || 'dark')

export function useTheme() {
  function setTheme(theme: string) {
    currentTheme.value = theme
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(THEME_KEY, theme)
  }

  function initTheme() {
    document.documentElement.setAttribute('data-theme', currentTheme.value)
  }

  initTheme()

  return { currentTheme, setTheme }
}
