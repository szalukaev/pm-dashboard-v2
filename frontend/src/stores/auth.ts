import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

interface User {
  id: number
  username: string
  role: string
  force_password_change?: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = ref(false)

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

  return { user, isAuthenticated, login, logout, fetchMe, changePassword }
})
