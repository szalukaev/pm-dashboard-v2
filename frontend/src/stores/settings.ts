import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import type { DashboardLayout } from '../utils/dashboardLayout'
import { setTimeZone } from '../utils/format'
import i18n from '../i18n'

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
  // Число строк на странице, выбранное для каждой таблицы; у таблицы, которой нет в списке, — значение по умолчанию
  pagination: Record<string, number>
  // Видимые колонки и их порядок, выбранные для каждой таблицы
  table_columns: Record<string, { visible: string[]; order: string[] }>
  // Раскладка виджетов вкладки «Аналитика»; {} — стандартная
  dashboard_layout: DashboardLayout
  // Правила уведомлений: куда доставлять и порог в днях
  alert_rules: Record<string, AlertRule>
  // Регулярная сводка на почту
  email_report: EmailReport
  // Пороги индикаторов «Аналитики»
  indicator_thresholds: IndicatorThresholds
  // Часовой пояс показа времени: свой выбор, пояс от администратора и пояс
  // сервера, по которому идут расписания. Пусто — не задан
  timezone: string
  default_timezone: string
  server_timezone: string
}

// Пороги: индикатор подсвечивается, когда задач больше порога. per_project —
// порог сравнивается с каждым проектом, а не со всеми сразу
export interface IndicatorThresholds {
  overdue: number
  bugs: number
  no_estimate: number
  per_project: boolean
}

// Настройка регулярной сводки: email пуст — письмо уходит на адрес из
// профиля; period — daily, weekly (по понедельникам) или monthly (1-го числа)
export interface EmailReport {
  enabled: boolean
  email: string
  period: string
  time: string
  sections: string[]
  overdue_list: boolean
}

// Настройка одного правила уведомлений
export interface AlertRule {
  bell: boolean
  email: boolean
  days?: number
  projects?: number[]
}

export const PAGE_SIZES = [10, 25, 50, 100]
export const DEFAULT_PAGE_SIZE = 25

// Виджет при перетаскивании или изменении размера меняет раскладку много
// раз подряд; она сохраняется, когда пользователь остановился
const LAYOUT_SAVE_DELAY_MS = 500

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
  dashboard_layout: {},
  alert_rules: {},
  indicator_thresholds: { overdue: 5, bugs: 10, no_estimate: 3, per_project: false },
  email_report: { enabled: false, email: '', period: 'weekly', time: '09:00', sections: ['stats', 'team_load', 'distribution', 'deadlines'], overdue_list: true },
  timezone: '',
  default_timezone: '',
  server_timezone: '',
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({ ...defaultSettings })
  const loading = ref(false)

  let loaded: Promise<void> | null = null

  // Для тех, кому нужно только, чтобы настройки были: загружает их один раз
  function ensureLoaded(): Promise<void> {
    if (!loaded) loaded = fetchSettings()
    return loaded
  }

  async function fetchSettings() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/settings')
      settings.value = { ...defaultSettings, ...data, pagination: data.pagination || {}, table_columns: data.table_columns || {}, dashboard_layout: data.dashboard_layout || {}, alert_rules: data.alert_rules || {} }
      applyTimeZone()
      applyLanguage()
    } catch {
      // Остаются значения по умолчанию
    } finally {
      loading.value = false
    }
  }

  // Свой пояс важнее пояса от администратора; без обоих — пояс браузера
  function applyTimeZone() {
    setTimeZone(settings.value.timezone || settings.value.default_timezone)
  }

  // Язык выбран пользователем и хранится на сервере, поэтому действует в
  // любом браузере, а не только там, где его выбирали
  function applyLanguage() {
    const lang = settings.value.language
    const locale = i18n.global.locale
    if (locale.value === lang || !i18n.global.availableLocales.includes(lang as any)) return
    locale.value = lang as any
    localStorage.setItem('pm-dashboard-lang', lang)
  }

  async function updateSettings(partial: Partial<Settings>) {
    try {
      await axios.put('/api/settings', partial)
      settings.value = { ...settings.value, ...partial }
      applyTimeZone()
    } catch {}
  }

  function pageSize(table: string): number {
    const size: number | undefined = settings.value.pagination?.[table]
    return size !== undefined && PAGE_SIZES.includes(size) ? size : DEFAULT_PAGE_SIZE
  }

  // Выбор хранится по каждой таблице и переживает перезагрузку
  async function setPageSize(table: string, size: number) {
    if (!PAGE_SIZES.includes(size)) return
    settings.value = { ...settings.value, pagination: { ...settings.value.pagination, [table]: size } }
    try {
      await axios.put('/api/settings', { pagination: { [table]: size } })
    } catch {}
  }

  // Все колонки таблицы в порядке, выбранном пользователем. all — стандартный
  // порядок. Колонка, добавленная в таблицу позже, встаёт после своей соседки
  // слева из стандартного порядка, а не в конец.
  function columnOrder(table: string, all: string[]): string[] {
    const saved = settings.value.table_columns?.[table]
    if (!saved || !Array.isArray(saved.order)) return all
    const order = saved.order.filter(k => all.includes(k))
    all.forEach((key, i) => {
      if (order.includes(key)) return
      const left = i > 0 ? order.indexOf(all[i - 1]) : -1
      order.splice(left + 1, 0, key)
    })
    return order
  }

  // Колонки таблицы, которые видит пользователь, в их порядке. Колонка,
  // добавленная в таблицу позже, видна, пока пользователь её не скроет.
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

  // Фильтры страницы или виджета, которые запоминаются между посещениями.
  // Фильтры, сохранённые под другими именами, остаются как есть.
  async function setLastFilters(name: string, filters: Record<string, any>) {
    settings.value = { ...settings.value, last_filters: { ...settings.value.last_filters, [name]: filters } }
    try {
      await axios.put('/api/settings', { last_filters: { [name]: filters } })
    } catch {}
  }

  // Сохраняет настройку одного правила уведомлений; остальные не трогает
  async function setAlertRule(key: string, rule: AlertRule) {
    settings.value = { ...settings.value, alert_rules: { ...settings.value.alert_rules, [key]: rule } }
    try {
      await axios.put('/api/settings', { alert_rules: { [key]: rule } })
    } catch {}
  }

  let layoutTimer: ReturnType<typeof setTimeout> | null = null

  // Применяется сразу, сохраняется после паузы
  function setDashboardLayout(layout: DashboardLayout) {
    settings.value = { ...settings.value, dashboard_layout: layout }
    if (layoutTimer) clearTimeout(layoutTimer)
    layoutTimer = setTimeout(async () => {
      layoutTimer = null
      try {
        await axios.put('/api/settings', { dashboard_layout: settings.value.dashboard_layout })
      } catch {}
    }, LAYOUT_SAVE_DELAY_MS)
  }

  return {
    settings, loading, fetchSettings, ensureLoaded, updateSettings, pageSize, setPageSize,
    columnOrder, tableColumns, setTableColumns, setDashboardLayout, setLastFilters, setAlertRule,
  }
})
