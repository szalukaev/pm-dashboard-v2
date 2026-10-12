import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

// Изменение одного поля задачи: что было и что стало
export interface NotificationChange {
  field: string
  from: string
  to: string
}

// Запись колокольчика. rule — правило, по которому она создана; params —
// данные для текста; entity_type и entity_id — на что она ведёт.
export interface AppNotification {
  id: number
  rule: string
  params: {
    issue_id?: number
    subject?: string
    project?: string
    due?: string
    days?: number
    changes?: NotificationChange[]
    [key: string]: unknown
  }
  entity_type: string
  entity_id: number
  created_at: string
  read: boolean
}

const PAGE = 20

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<AppNotification[]>([])
  const total = ref(0)
  const unread = ref(0)
  const loading = ref(false)

  // Первая страница журнала и счётчик непрочитанных
  async function fetch() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/notifications', { params: { limit: PAGE, offset: 0 } })
      items.value = data.notifications || []
      total.value = data.total || 0
      unread.value = data.unread || 0
    } catch {
    } finally {
      loading.value = false
    }
  }

  // Следующая страница — более старые уведомления
  async function fetchMore() {
    if (loading.value || items.value.length >= total.value) return
    loading.value = true
    try {
      const { data } = await axios.get('/api/notifications', { params: { limit: PAGE, offset: items.value.length } })
      const known = new Set(items.value.map(n => n.id))
      const more: AppNotification[] = (data.notifications || []).filter((n: AppNotification) => !known.has(n.id))
      items.value = [...items.value, ...more]
      total.value = data.total || 0
      unread.value = data.unread || 0
    } catch {
    } finally {
      loading.value = false
    }
  }

  // Уведомление, пришедшее по WebSocket, встаёт первым
  function add(n: AppNotification) {
    if (items.value.some(item => item.id === n.id)) return
    items.value = [{ ...n, read: false }, ...items.value]
    total.value++
    unread.value++
  }

  async function markRead(id: number) {
    const item = items.value.find(n => n.id === id)
    if (!item || item.read) return
    item.read = true
    unread.value = Math.max(0, unread.value - 1)
    try {
      await axios.post(`/api/notifications/${id}/read`)
    } catch {}
  }

  async function markAllRead() {
    items.value = items.value.map(n => ({ ...n, read: true }))
    unread.value = 0
    try {
      await axios.post('/api/notifications/read-all')
    } catch {}
  }

  return { items, total, unread, loading, fetch, fetchMore, add, markRead, markAllRead }
})
