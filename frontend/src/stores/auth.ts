import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

interface User {
  id: number
  username: string
  display_name?: string
  // Адрес для писем; пусто — не задан
  email?: string
  role: string
  avatar?: string
  last_login?: string
  force_password_change?: boolean
  // Что администратор разрешает пользователю видеть и делать. Проверяет это
  // сервер; клиент лишь скрывает то, что всё равно не сработало бы.
  access?: {
    visible_tabs: string[]
    widgets: string[]
    read_only: boolean
    // Разрешённые действия со спринтами: create, edit, tasks, close, delete
    sprint_actions?: string[]
    // Доступные виды доски Kanban: users, statuses
    kanban_modes?: string[]
    // Разрешённые действия с оплатами (см. PAYMENT_ACTIONS)
    payment_actions?: string[]
    // В «Оплатах» видны только свои записи
    payments_own_only?: boolean
  }
  // Учётная запись пользователя в источнике данных, привязанная личным
  // ключом API. type — вид источника ('redmine'); needs_token просит ввести ключ.
  source?: {
    type: string
    linked: boolean
    member_id: number | null
    member_name: string
    token_hint: string
    needs_token: boolean
  }
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = ref(false)

  const isAdmin = computed(() => user.value?.role === 'admin')
  // Пользователь может менять данные (не «только чтение»)
  const canWrite = computed(() => !user.value?.access?.read_only)

  function tabVisible(tab: string): boolean {
    const tabs = user.value?.access?.visible_tabs
    return isAdmin.value || !tabs || tabs.includes(tab)
  }

  function widgetVisible(widget: string): boolean {
    const widgets = user.value?.access?.widgets
    return isAdmin.value || !widgets || widgets.includes(widget)
  }

  async function login(username: string, password: string) {
    // Без ответа за 15 с вход прерывается: на плохой сети телефон иначе
    // ждёт минуту и дольше, и пользователь не понимает, что происходит
    const { data } = await axios.post('/api/auth/login', { username, password }, { timeout: 15000 })
    user.value = data.user
    isAuthenticated.value = true
    return data
  }

  async function logout() {
    try {
      await axios.post('/api/auth/logout')
    } catch {}
    user.value = null
    isAuthenticated.value = false
  }

  async function fetchMe() {
    try {
      const { data } = await axios.get('/api/auth/me')
      user.value = data
      isAuthenticated.value = true
    } catch {
      user.value = null
      isAuthenticated.value = false
    }
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    await axios.post('/api/auth/change-password', {
      current_password: currentPassword,
      new_password: newPassword,
    })
    if (user.value) {
      user.value.force_password_change = false
    }
  }

  // email задаётся, только если передан: без него адрес не меняется
  async function updateMe(displayName: string, email?: string) {
    const body: Record<string, string> = { display_name: displayName }
    if (email !== undefined) body.email = email
    const { data } = await axios.put('/api/auth/me', body)
    if (user.value) {
      user.value.display_name = data.display_name
      user.value.email = data.email || ''
    }
  }

  // Пользователю разрешено действие со спринтами (create | edit | tasks | close | delete).
  // «Только чтение» закрывает их все.
  function canSprint(action: string): boolean {
    if (isAdmin.value) return true
    if (!canWrite.value) return false
    const actions = user.value?.access?.sprint_actions
    return !actions || actions.includes(action)
  }

  // Пользователю доступен вид доски Kanban (users | statuses)
  function canKanban(mode: string): boolean {
    const modes = user.value?.access?.kanban_modes
    return isAdmin.value || !modes || modes.includes(mode)
  }

  // Пользователю разрешено действие с оплатами (см. PAYMENT_ACTIONS). При
  // «только чтении» изменения закрыты, скачивание остаётся.
  function canPayment(action: string): boolean {
    if (isAdmin.value) return true
    if (action !== 'download' && !canWrite.value) return false
    const actions = user.value?.access?.payment_actions
    return !actions || actions.includes(action)
  }

  // Привязывает пользователя к владельцу личного ключа API источника
  // данных. Сделать это может только сам пользователь и только для себя.
  async function setSourceToken(token: string) {
    const { data } = await axios.put('/api/auth/source-token', { token })
    if (user.value) user.value.source = data.source
  }

  async function clearSourceToken() {
    const { data } = await axios.delete('/api/auth/source-token')
    if (user.value) user.value.source = data.source
  }

  return {
    user, isAuthenticated, isAdmin, canWrite, tabVisible, widgetVisible, canSprint, canKanban, canPayment,
    login, logout, fetchMe, changePassword, updateMe, setSourceToken, clearSourceToken,
  }
})
