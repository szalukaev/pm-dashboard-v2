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

// A card or a column is being dragged somewhere on the page
let dragging = false
if (typeof document !== 'undefined') {
  document.addEventListener('dragstart', () => { dragging = true }, true)
  document.addEventListener('dragend', () => { dragging = false }, true)
  document.addEventListener('drop', () => { dragging = false }, true)
}

// Re-rendering under the user's hands would reset a field being typed in or
// break a drag, so a refresh waits until they are done.
function userIsBusy(): boolean {
  if (dragging) return true
  const el = document.activeElement
  return !!el && ['INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName)
}

/**
 * Keeps a page up to date while it is open: when the server reports changed
 * issues (after a sync or another user's edit), `refresh` re-reads the data
 * quietly and the changed issues flash.
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

  // A finished sync may also have changed statuses, members or projects
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
