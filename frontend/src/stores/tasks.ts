import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export interface Task {
  id: number
  external_id: number
  project_id: number
  project_name: string
  subject: string
  description: string
  status_name: string
  status_id: number
  priority_name: string
  priority_id: number
  assigned_to_name: string
  assigned_to_id: number | null
  category_name: string
  start_date: string | null
  due_date: string | null
  estimated_hours: number | null
  spent_hours: number
  done_ratio: number
  tracker_name: string
  author_name: string
  bug_fix_hours: number
  bug_fix_pct: number
}

export interface TaskGroup {
  name: string
  task_count: number
  estimate_total: number
  fact_total: number
  tasks: Task[]
}

export interface TaskFilters {
  type: string
  project_id: string
  search: string
  group_by: string
  category: string
  sort_by: string
  sort_dir: string
}

export const useTasksStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([])
  const groups = ref<TaskGroup[]>([])
  const loading = ref(false)
  const error = ref('')
  const total = ref(0)
  const useGrouping = ref(false)

  const filters = ref<TaskFilters>({
    type: 'open',
    project_id: '',
    search: '',
    group_by: 'project',
    category: '',
    sort_by: '',
    sort_dir: 'asc',
  })

  // Reference data
  const projects = ref<{ id: number; name: string; parent_id: number | null }[]>([])
  const categories = ref<string[]>([])
  const statuses = ref<{ id: number; name: string; is_closed: boolean; group: string }[]>([])

  async function fetchTasks() {
    loading.value = true
    error.value = ''
    try {
      const params: Record<string, string> = {}
      if (filters.value.type) params.type = filters.value.type
      if (filters.value.project_id) params.project_id = filters.value.project_id
      if (filters.value.search) params.search = filters.value.search
      if (filters.value.category) params.category = filters.value.category
      if (filters.value.sort_by) {
        params.sort_by = filters.value.sort_by
        params.sort_dir = filters.value.sort_dir
      }

      if (useGrouping.value) {
        params.group_by = filters.value.group_by
        const { data } = await axios.get('/api/tasks', { params })
        groups.value = data.groups || []
        tasks.value = []
      } else {
        const { data } = await axios.get('/api/tasks', { params })
        tasks.value = data.tasks || []
        groups.value = []
      }
      total.value = tasks.value.length || groups.value.reduce((s, g) => s + g.task_count, 0)
    } catch (e: any) {
      error.value = 'Не удалось загрузить данные'
    } finally {
      loading.value = false
    }
  }

  async function fetchTask(id: number): Promise<Task | null> {
    try {
      const { data } = await axios.get(`/api/tasks/${id}`)
      return data
    } catch {
      return null
    }
  }

  async function updateTask(id: number, fields: Record<string, any>) {
    await axios.put(`/api/tasks/${id}`, fields)
    await fetchTasks()
  }

  async function fetchProjects() {
    try {
      const { data } = await axios.get('/api/tasks/projects')
      projects.value = data.projects || []
    } catch {}
  }

  async function fetchCategories() {
    try {
      const { data } = await axios.get('/api/tasks/categories')
      categories.value = data.categories || []
    } catch {}
  }

  async function fetchStatuses() {
    try {
      const { data } = await axios.get('/api/tasks/statuses')
      statuses.value = data.statuses || []
    } catch {}
  }

  function setFilter(key: keyof TaskFilters, value: string) {
    filters.value[key] = value
    saveFilters()
  }

  async function saveFilters() {
    try {
      await axios.put('/api/settings', { last_filters: { tasks: { ...filters.value, useGrouping: useGrouping.value } } })
    } catch {}
  }

  async function restoreFilters() {
    try {
      const { data } = await axios.get('/api/settings')
      if (data.last_filters?.tasks) {
        const saved = data.last_filters.tasks
        if (saved.type) filters.value.type = saved.type
        if (saved.project_id) filters.value.project_id = saved.project_id
        if (saved.group_by) filters.value.group_by = saved.group_by
        if (saved.category) filters.value.category = saved.category
        if (saved.sort_by) filters.value.sort_by = saved.sort_by
        if (saved.sort_dir) filters.value.sort_dir = saved.sort_dir
        if (saved.useGrouping !== undefined) useGrouping.value = saved.useGrouping
      }
    } catch {}
  }

  return {
    tasks, groups, loading, error, total, useGrouping, filters,
    projects, categories, statuses,
    fetchTasks, fetchTask, updateTask,
    fetchProjects, fetchCategories, fetchStatuses, setFilter, restoreFilters,
  }
})
