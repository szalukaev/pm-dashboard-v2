import { onMounted, onUnmounted } from 'vue'
import { useWebSocket } from './useWebSocket'
import { flashIssues } from './useFlash'

interface IssueChanges {
  added?: number[]
  updated?: number[]
  removed?: number[]
}

const DEBOUNCE_MS = 400
const BUSY_RETRY_MS = 1500
const BUSY_RETRIES = 20

// Где-то на странице перетаскивают карточку или колонку
let dragging = false
if (typeof document !== 'undefined') {
  document.addEventListener('dragstart', () => { dragging = true }, true)
  document.addEventListener('dragend', () => { dragging = false }, true)
  document.addEventListener('drop', () => { dragging = false }, true)
}

// Перерисовка под руками пользователя сбросила бы поле, в которое он
// печатает, или сорвала бы перетаскивание, поэтому обновление ждёт, пока он
// закончит.
function userIsBusy(): boolean {
  if (dragging) return true
  const el = document.activeElement
  return !!el && ['INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName)
}

/**
 * Поддерживает страницу в актуальном виде, пока она открыта: когда сервер
 * сообщает об изменённых задачах (после синхронизации или правки другого
 * пользователя), `refresh` тихо перечитывает данные, а изменённые задачи
 * подсвечиваются.
 */
export function useLiveRefresh(refresh: () => Promise<unknown> | unknown) {
  const { on, off } = useWebSocket()

  const pending = new Set<number>()
  let timer: ReturnType<typeof setTimeout> | null = null
  let retries = 0
  let active = false

  function schedule(delay: number) {
    if (timer) clearTimeout(timer)
    timer = setTimeout(run, delay)
  }

  async function run() {
    timer = null
    if (!active) return
    if (userIsBusy() && retries < BUSY_RETRIES) {
      retries++
      schedule(BUSY_RETRY_MS)
      return
    }
    retries = 0
    const ids = [...pending]
    pending.clear()
    try {
      await refresh()
    } catch {
      return
    }
    flashIssues(ids)
  }

  function onIssuesChanged(changes: IssueChanges | null) {
    for (const id of [...(changes?.added || []), ...(changes?.updated || [])]) pending.add(id)
    schedule(DEBOUNCE_MS)
  }

  // Завершившаяся синхронизация могла изменить и статусы, сотрудников или проекты
  function onCollectionComplete() {
    schedule(DEBOUNCE_MS)
  }

  onMounted(() => {
    active = true
    on('issues-changed', onIssuesChanged)
    on('collection-complete', onCollectionComplete)
  })

  onUnmounted(() => {
    active = false
    off('issues-changed', onIssuesChanged)
    off('collection-complete', onCollectionComplete)
    if (timer) clearTimeout(timer)
  })
}
