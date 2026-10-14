import { ref } from 'vue'

// Мобильный вид вкладки «Задачи»: узкий экран и выбранный режим списка
// (карточки или лента сроков). Состояние общее для всех списков на странице —
// плоского и списков внутри групп.

export type TaskMobileMode = 'cards' | 'deadlines'

const MOBILE_QUERY = '(max-width: 768px)'
const MODE_KEY = 'pm-dashboard-tasks-mobile-mode'

function readMode(): TaskMobileMode {
  try {
    return localStorage.getItem(MODE_KEY) === 'deadlines' ? 'deadlines' : 'cards'
  } catch {
    return 'cards'
  }
}

const query = window.matchMedia(MOBILE_QUERY)
const isMobile = ref(query.matches)
query.addEventListener('change', e => { isMobile.value = e.matches })

const mode = ref<TaskMobileMode>(readMode())

function setMode(value: TaskMobileMode) {
  mode.value = value
  try {
    localStorage.setItem(MODE_KEY, value)
  } catch {}
}

export function useMobileTasks() {
  return { isMobile, mode, setMode }
}
