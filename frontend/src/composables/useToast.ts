import { ref } from 'vue'

export type ToastType = 'success' | 'error' | 'info'

// Смена значения поля задачи: сверху поле и номер, ниже «было → стало»
export interface ToastChange {
  field?: string
  taskId?: string | number
  oldValue: string
  newValue: string
}

export interface ToastItem {
  id: number
  type: ToastType
  message?: string
  change?: ToastChange
  duration: number
}

// Больше пяти уведомлений в стопке не держим — самые старые уходят
const MAX_TOASTS = 5

// Общий список на всё приложение: показывает его AppToast в App.vue
const toasts = ref<ToastItem[]>([])
const timers = new Map<number, { handle: number; left: number; startedAt: number }>()
let nextId = 0

function dismiss(id: number) {
  const timer = timers.get(id)
  if (timer) clearTimeout(timer.handle)
  timers.delete(id)
  toasts.value = toasts.value.filter(t => t.id !== id)
}

function startTimer(id: number, ms: number) {
  timers.set(id, { handle: window.setTimeout(() => dismiss(id), ms), left: ms, startedAt: Date.now() })
}

function push(item: Omit<ToastItem, 'id'>) {
  const id = nextId++
  // Новые — в начало: они лежат сверху стопки
  toasts.value = [{ ...item, id }, ...toasts.value]
  toasts.value.slice(MAX_TOASTS).forEach(t => dismiss(t.id))
  startTimer(id, item.duration)
  return id
}

// Пока курсор над стопкой, уведомления не исчезают
function pauseAll() {
  for (const timer of timers.values()) {
    clearTimeout(timer.handle)
    timer.left -= Date.now() - timer.startedAt
  }
}

function resumeAll() {
  for (const [id, timer] of [...timers.entries()]) startTimer(id, Math.max(timer.left, 1000))
}

export function useToast() {
  function show(message: string, type: ToastType = 'success') {
    // Ошибку обычно нужно дочитать — она держится дольше
    return push({ type, message, duration: type === 'error' ? 6000 : 3500 })
  }

  function showChange(change: ToastChange) {
    return push({ type: 'success', change, duration: 4000 })
  }

  return { toasts, show, showChange, dismiss, pauseAll, resumeAll }
}
