import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

interface Settings {
  selected_projects: number[]
  selected_team: number[]
  theme: string
  language: string
  tab_order: string[]
  kanban_column_order_statuses: string[]
  kanban_column_order_users: string[]
  last_filters: Record<string, any>
  notifications_enabled: boolean
  overdue_alerts: boolean
  data_source_url: string
  redmine_url: string
}

const defaultSettings: Settings = {
  selected_projects: [],
  selected_team: [],
  theme: 'nord-dark',
  language: 'ru',
  tab_order: [],
  kanban_column_order_statuses: [],
  kanban_column_order_users: [],
  last_filters: {},
  notifications_enabled: true,
  overdue_alerts: false,
  data_source_url: '',
  redmine_url: '',
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({ ...defaultSettings })
  const loading = ref(false)

  async function fetchSettings() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/settings')
      settings.value = { ...defaultSettings, ...data }
    } catch {
      // Use defaults
    } finally {
      loading.value = false
    }
  }

  async function updateSettings(partial: Partial<Settings>) {
    try {
      await axios.put('/api/settings', partial)
      settings.value = { ...settings.value, ...partial }
    } catch {}
  }

  return { settings, loading, fetchSettings, updateSettings }
})
