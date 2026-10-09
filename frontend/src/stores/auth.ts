import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

interface User {
  id: number
  username: string
  display_name?: string
  role: string
  avatar?: string
  last_login?: string
  force_password_change?: boolean
  // What the administrator lets the user see and do. The server enforces
  // it; the client only hides what would not work anyway.
  access?: {
    visible_tabs: string[]
    widgets: string[]
    read_only: boolean
  }
  // The user's account in the data source, tied by a personal API key.
  // type is the kind of the source ('redmine'); needs_token asks for the key.
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
  // The user may change data (not a read-only one)
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
    const { data } = await axios.post('/api/auth/login', { username, password })
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

  async function updateMe(displayName: string) {
    const { data } = await axios.put('/api/auth/me', { display_name: displayName })
    if (user.value) {
      user.value.display_name = data.display_name
    }
  }

  // Ties the user to the owner of the personal API key of the data source.
  // Only the user can do it, and only for themselves.
  async function setSourceToken(token: string) {
    const { data } = await axios.put('/api/auth/source-token', { token })
    if (user.value) user.value.source = data.source
  }

  async function clearSourceToken() {
    const { data } = await axios.delete('/api/auth/source-token')
    if (user.value) user.value.source = data.source
  }

  return {
    user, isAuthenticated, isAdmin, canWrite, tabVisible, widgetVisible,
    login, logout, fetchMe, changePassword, updateMe, setSourceToken, clearSourceToken,
  }
})
