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
  // Rows per page chosen for each table; a table not listed uses the default
  pagination: Record<string, number>
  // Visible columns and their order chosen for each table
  table_columns: Record<string, { visible: string[]; order: string[] }>
}

export const PAGE_SIZES = [10, 25, 50, 100]
export const DEFAULT_PAGE_SIZE = 25

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
  pagination: {},
  table_columns: {},
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({ ...defaultSettings })
  const loading = ref(false)

  let loaded: Promise<void> | null = null

  // For those who only need the settings to be there: loads them once
  function ensureLoaded(): Promise<void> {
    if (!loaded) loaded = fetchSettings()
    return loaded
  }

  async function fetchSettings() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/settings')
      settings.value = { ...defaultSettings, ...data, pagination: data.pagination || {}, table_columns: data.table_columns || {} }
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

  function pageSize(table: string): number {
    const size: number | undefined = settings.value.pagination?.[table]
    return size !== undefined && PAGE_SIZES.includes(size) ? size : DEFAULT_PAGE_SIZE
  }

  // The choice is kept per table and survives a reload
  async function setPageSize(table: string, size: number) {
    if (!PAGE_SIZES.includes(size)) return
    settings.value = { ...settings.value, pagination: { ...settings.value.pagination, [table]: size } }
    try {
      await axios.put('/api/settings', { pagination: { [table]: size } })
    } catch {}
  }

  // Every column of a table in the order the user chose. all is the standard
  // order; a column added to the table later goes to the end.
  function columnOrder(table: string, all: string[]): string[] {
    const saved = settings.value.table_columns?.[table]
    if (!saved || !Array.isArray(saved.order)) return all
    return [...saved.order.filter(k => all.includes(k)), ...all.filter(k => !saved.order.includes(k))]
  }

  // The columns of a table the user sees, in their order. A column added to
  // the table later is visible until the user hides it.
  function tableColumns(table: string, all: string[]): string[] {
    const saved = settings.value.table_columns?.[table]
    if (!saved || !Array.isArray(saved.order) || !Array.isArray(saved.visible)) return all
    const shown = columnOrder(table, all).filter(k => saved.visible.includes(k) || !saved.order.includes(k))
    return shown.length > 0 ? shown : all
  }

  async function setTableColumns(table: string, visible: string[], order: string[]) {
    const setup = { visible, order }
    settings.value = { ...settings.value, table_columns: { ...settings.value.table_columns, [table]: setup } }
    try {
      await axios.put('/api/settings', { table_columns: { [table]: setup } })
    } catch {}
  }

  return {
    settings, loading, fetchSettings, ensureLoaded, updateSettings, pageSize, setPageSize,
    columnOrder, tableColumns, setTableColumns,
  }
})
