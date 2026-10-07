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
    sort_by: 'external_id',
    sort_dir: 'desc',
  })

  // Reference data
  const projects = ref<{ id: number; name: string; parent_id: number | null }[]>([])
  const categories = ref<string[]>([])
  const projectCategories = ref<Record<number, string[]>>({})
  const members = ref<{ id: number; name: string }[]>([])
  const statuses = ref<{ id: number; name: string; is_closed: boolean; group: string }[]>([])
  const priorities = ref<{ id: number; name: string; sort_order: number }[]>([])

  // Only the latest request may update the list.
  let tasksRequest = 0

  // silent: refresh in the background without the loading skeleton, so the
  // table does not flicker and expanded groups stay open.
  async function fetchTasks(opts: { silent?: boolean } = {}) {
    const requestId = ++tasksRequest
    if (!opts.silent) {
      loading.value = true
      error.value = ''
    }
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

      if (useGrouping.value) params.group_by = filters.value.group_by
      const { data } = await axios.get('/api/tasks', { params })
      if (requestId !== tasksRequest) return
      if (useGrouping.value) {
        groups.value = data.groups || []
        tasks.value = []
      } else {
        tasks.value = data.tasks || []
        groups.value = []
      }
      total.value = tasks.value.length || groups.value.reduce((s, g) => s + g.task_count, 0)
    } catch (e: any) {
      if (requestId === tasksRequest && !opts.silent) error.value = 'Не удалось загрузить данные'
    } finally {
      if (requestId === tasksRequest) loading.value = false
    }
  }

  // Applies a saved change to every loaded copy of the task right away.
  function applyTaskChange(id: number, fields: Record<string, any>) {
    const patch = (t: Task) => {
      if (t.external_id !== id) return
      for (const [k, v] of Object.entries(fields)) {
        ;(t as any)[k] = k === 'estimated_hours' && v !== null && v !== '' ? Number(v) : v
      }
    }
    tasks.value.forEach(patch)
    for (const g of groups.value) {
      g.tasks.forEach(patch)
      g.estimate_total = g.tasks.reduce((s, t) => s + (t.estimated_hours || 0), 0)
      g.fact_total = g.tasks.reduce((s, t) => s + (t.spent_hours || 0), 0)
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

  async function updateTask(id: number, fields: Record<string, any>, skipReload = false): Promise<{ old_values: any; new_values: any; redmine_ok: boolean; redmine_error: string }> {
    const { data } = await axios.put(`/api/tasks/${id}`, fields)
    if (!skipReload) {
      await fetchTasks()
    } else if (data?.redmine_ok) {
      // Redmine and our database already hold the new value: show it at once,
      // then re-read quietly so the task moves to its new group / filter.
      applyTaskChange(id, fields)
      fetchTasks({ silent: true })
    }
    return data
  }

  async function fetchProjects() {
    try {
      const { data } = await axios.get('/api/tasks/projects')
      projects.value = data.projects || []
    } catch {}
  }

  // Only the latest request may update the list: a slow response for a
  // previous project must not overwrite the categories of the current one.
  let categoriesRequest = 0

  async function fetchCategories(projectId?: string | number) {
    const requestId = ++categoriesRequest
    try {
      const url = projectId ? `/api/tasks/categories?project_id=${projectId}` : '/api/tasks/categories'
      const { data } = await axios.get(url)
      if (requestId === categoriesRequest) categories.value = data.categories || []
    } catch {}
  }

  // Assignee choices; loaded once and shared by every task table.
  let membersRequest: Promise<void> | null = null

  function fetchMembers(): Promise<void> {
    if (!membersRequest) {
      membersRequest = axios.get('/api/tasks/members')
        .then(({ data }) => { members.value = data.members || [] })
        .catch(() => { membersRequest = null })
    }
    return membersRequest
  }

  // All categories defined in a project, for inline editing; cached per project.
  async function fetchProjectCategories(projectId: number) {
    if (projectCategories.value[projectId]) return
    try {
      const { data } = await axios.get('/api/tasks/project-categories', { params: { project_id: projectId } })
      projectCategories.value = { ...projectCategories.value, [projectId]: data.categories || [] }
    } catch {}
  }

  async function fetchPriorities() {
    try {
      const { data } = await axios.get('/api/tasks/priorities')
      priorities.value = data.priorities || []
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

  // Active query filters that narrow the result set (grouping/sort are view prefs).
  function hasActiveFilters(): boolean {
    return !!(
      filters.value.project_id ||
      filters.value.search ||
      filters.value.category ||
      (filters.value.type && filters.value.type !== 'all')
    )
  }

  async function resetFilters() {
    filters.value = {
      type: 'open',
      project_id: '',
      search: '',
      group_by: filters.value.group_by,
      category: '',
      sort_by: 'external_id',
      sort_dir: 'desc',
    }
    await saveFilters()
    await fetchTasks()
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
    projects, categories, projectCategories, members, statuses, priorities,
    fetchTasks, fetchTask, updateTask,
    fetchProjects, fetchCategories, fetchProjectCategories, fetchMembers, fetchStatuses, fetchPriorities,
    setFilter, restoreFilters, resetFilters, hasActiveFilters,
  }
})
