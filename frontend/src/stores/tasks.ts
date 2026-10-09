import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import { useSettingsStore } from './settings'

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
  // The loaded page of the group
  tasks: Task[]
  offset: number
  loading: boolean
}

// Name of the table in the user's page size settings
const PAGE_TABLE = 'tasks_table'

interface GroupHeader {
  name: string
  task_count: number
  estimate_total: number
  fact_total: number
}

interface GroupPage {
  ids: number[]
  offset: number
  loading: boolean
  loaded: boolean
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
  const settingsStore = useSettingsStore()

  // Every loaded task lives once in this map (number → task). The flat list
  // and the groups only refer to it, so a change of a task shows everywhere
  // and its table row is updated in place instead of being rebuilt.
  //
  // Tasks are loaded page by page: the flat list holds one page, and every
  // expanded group holds one page of its own. The headers of the groups
  // (number of tasks, totals) come from the server and cover the whole group.
  const taskMap = ref(new Map<number, Task>())
  const taskOrder = ref<number[]>([])
  const groupDefs = ref<GroupHeader[]>([])
  const groupPages = ref<Record<string, GroupPage>>({})
  const offset = ref(0)

  const pageSize = computed(() => settingsStore.pageSize(PAGE_TABLE))

  function tasksByIds(ids: number[]): Task[] {
    const list: Task[] = []
    for (const id of ids) {
      const task = taskMap.value.get(id)
      if (task) list.push(task)
    }
    return list
  }

  const tasks = computed<Task[]>(() => tasksByIds(taskOrder.value))
  const groups = computed<TaskGroup[]>(() => groupDefs.value.map(def => {
    const page = groupPages.value[def.name]
    return {
      ...def,
      tasks: page ? tasksByIds(page.ids) : [],
      offset: page?.offset || 0,
      loading: !!page?.loading && !page.loaded,
    }
  }))

  // Puts a fresh server answer into the map. A task that was already loaded
  // keeps its object and only gets the new field values.
  function storeTasks(incoming: Task[]) {
    const map = taskMap.value
    for (const task of incoming) {
      const existing = map.get(task.external_id)
      if (existing) Object.assign(existing, task)
      else map.set(task.external_id, task)
    }
  }

  // Drops the tasks no loaded page refers to any more
  function pruneTasks() {
    const used = new Set<number>(taskOrder.value)
    for (const page of Object.values(groupPages.value)) {
      for (const id of page.ids) used.add(id)
    }
    for (const id of [...taskMap.value.keys()]) {
      if (!used.has(id)) taskMap.value.delete(id)
    }
  }

  // The page the user was on may be gone after a refresh (tasks were closed
  // or filtered out): the last page that still exists is shown instead.
  function lastPageOffset(count: number): number {
    return count > 0 ? Math.floor((count - 1) / pageSize.value) * pageSize.value : 0
  }

  const loading = ref(false)
  const error = ref('')
  const total = ref(0)
  const useGrouping = ref(false)
  // Accordion groups the user expanded; everything else is collapsed.
  const expandedGroups = ref<string[]>([])

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
      await settingsStore.ensureLoaded()
      if (useGrouping.value) {
        const { data } = await axios.get('/api/tasks', { params: { ...queryParams(), group_by: filters.value.group_by } })
        if (requestId !== tasksRequest) return
        const headers: GroupHeader[] = (data.groups || []).map((g: GroupHeader) => ({
          name: g.name,
          task_count: g.task_count,
          estimate_total: g.estimate_total,
          fact_total: g.fact_total,
        }))
        // Pages of the groups that are gone or collapsed are not kept
        const kept: Record<string, GroupPage> = {}
        for (const g of headers) {
          const page = groupPages.value[g.name]
          if (page && isGroupExpanded(g.name)) kept[g.name] = page
        }
        groupPages.value = kept
        groupDefs.value = headers
        taskOrder.value = []
        total.value = data.total || 0
        await Promise.all(headers.filter(g => isGroupExpanded(g.name)).map(g => loadGroup(g.name)))
      } else {
        let data = await fetchPage(offset.value)
        if (requestId !== tasksRequest) return
        if ((data.tasks || []).length === 0 && data.total > 0) {
          offset.value = lastPageOffset(data.total)
          data = await fetchPage(offset.value)
          if (requestId !== tasksRequest) return
        }
        const list: Task[] = data.tasks || []
        storeTasks(list)
        taskOrder.value = list.map(t => t.external_id)
        groupDefs.value = []
        groupPages.value = {}
        total.value = data.total || 0
      }
      pruneTasks()
    } catch (e: any) {
      if (requestId === tasksRequest && !opts.silent) error.value = 'Не удалось загрузить данные'
    } finally {
      if (requestId === tasksRequest) loading.value = false
    }
  }

  function queryParams(): Record<string, string> {
    const params: Record<string, string> = {}
    if (filters.value.type) params.type = filters.value.type
    if (filters.value.project_id) params.project_id = filters.value.project_id
    if (filters.value.search) params.search = filters.value.search
    if (filters.value.category) params.category = filters.value.category
    if (filters.value.sort_by) {
      params.sort_by = filters.value.sort_by
      params.sort_dir = filters.value.sort_dir
    }
    return params
  }

  async function fetchPage(pageOffset: number, group?: string): Promise<{ tasks: Task[]; total: number }> {
    const params: Record<string, string | number> = { ...queryParams(), limit: pageSize.value, offset: pageOffset }
    if (group !== undefined) {
      params.group_by = filters.value.group_by
      params.group = group
    }
    const { data } = await axios.get('/api/tasks', { params })
    return data
  }

  // Loads the current page of a group. Only the latest request for a group
  // may fill it.
  const groupRequests = new Map<string, number>()

  async function loadGroup(name: string) {
    const requestId = (groupRequests.get(name) || 0) + 1
    groupRequests.set(name, requestId)
    const page: GroupPage = groupPages.value[name] || { ids: [], offset: 0, loading: false, loaded: false }
    groupPages.value = { ...groupPages.value, [name]: { ...page, loading: true } }
    try {
      let pageOffset = page.offset
      let data = await fetchPage(pageOffset, name)
      if ((data.tasks || []).length === 0 && data.total > 0) {
        pageOffset = lastPageOffset(data.total)
        data = await fetchPage(pageOffset, name)
      }
      if (groupRequests.get(name) !== requestId || !groupPages.value[name]) return
      const list: Task[] = data.tasks || []
      storeTasks(list)
      groupPages.value = {
        ...groupPages.value,
        [name]: { ids: list.map(t => t.external_id), offset: pageOffset, loading: false, loaded: true },
      }
    } catch {
      if (groupRequests.get(name) === requestId && groupPages.value[name]) {
        groupPages.value = { ...groupPages.value, [name]: { ...groupPages.value[name], loading: false } }
      }
    }
  }

  // Page of the flat list
  async function setPage(pageOffset: number) {
    offset.value = pageOffset
    await fetchTasks({ silent: true })
  }

  // Page of one group
  async function setGroupPage(name: string, pageOffset: number) {
    const page = groupPages.value[name]
    if (!page) return
    groupPages.value = { ...groupPages.value, [name]: { ...page, offset: pageOffset } }
    await loadGroup(name)
    pruneTasks()
  }

  // The page size is one for every table of tasks; changing it starts from
  // the first page.
  async function setPageSize(size: number) {
    await settingsStore.setPageSize(PAGE_TABLE, size)
    resetPages()
    await fetchTasks({ silent: true })
  }

  // A new filter or sorting makes the current pages meaningless
  function resetPages() {
    offset.value = 0
    const pages: Record<string, GroupPage> = {}
    for (const [name, page] of Object.entries(groupPages.value)) pages[name] = { ...page, offset: 0 }
    groupPages.value = pages
  }

  // Applies a saved change to the loaded task right away; the list, the
  // groups and their totals follow by themselves.
  function applyTaskChange(id: number, fields: Record<string, any>) {
    const task = taskMap.value.get(id)
    if (!task) return
    for (const [k, v] of Object.entries(fields)) {
      ;(task as any)[k] = k === 'estimated_hours' && v !== null && v !== '' ? Number(v) : v
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
    resetPages()
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
    resetPages()
    await saveFilters()
    await fetchTasks()
  }

  async function saveFilters() {
    try {
      await axios.put('/api/settings', {
        last_filters: { tasks: { ...filters.value, useGrouping: useGrouping.value, expandedGroups: expandedGroups.value } },
      })
    } catch {}
  }

  async function restoreFilters() {
    try {
      const { data } = await axios.get('/api/settings')
      if (data.last_filters?.tasks) {
        const saved = data.last_filters.tasks
        if (saved.type) filters.value.type = saved.type
        if (saved.project_id) filters.value.project_id = saved.project_id
        if (saved.search) filters.value.search = saved.search
        if (saved.group_by) filters.value.group_by = saved.group_by
        if (saved.category) filters.value.category = saved.category
        if (saved.sort_by) filters.value.sort_by = saved.sort_by
        if (saved.sort_dir) filters.value.sort_dir = saved.sort_dir
        if (saved.useGrouping !== undefined) useGrouping.value = saved.useGrouping
        if (Array.isArray(saved.expandedGroups)) expandedGroups.value = saved.expandedGroups
      }
    } catch {}
  }

  function isGroupExpanded(name: string): boolean {
    return expandedGroups.value.includes(name)
  }

  function toggleGroup(name: string) {
    const expand = !isGroupExpanded(name)
    expandedGroups.value = expand
      ? [...expandedGroups.value, name]
      : expandedGroups.value.filter(n => n !== name)
    // The tasks of a group are loaded when it is opened
    if (expand && !groupPages.value[name]?.loaded) loadGroup(name)
    saveFilters()
  }

  // A new grouping starts with every group collapsed.
  function setGrouping(mode: string) {
    expandedGroups.value = []
    groupPages.value = {}
    offset.value = 0
    if (mode === '') {
      useGrouping.value = false
      saveFilters()
    } else {
      useGrouping.value = true
      setFilter('group_by', mode)
    }
  }

  return {
    tasks, groups, loading, error, total, useGrouping, filters, expandedGroups,
    offset, pageSize, setPage, setGroupPage, setPageSize,
    isGroupExpanded, toggleGroup, setGrouping,
    projects, categories, projectCategories, members, statuses, priorities,
    fetchTasks, fetchTask, updateTask,
    fetchProjects, fetchCategories, fetchProjectCategories, fetchMembers, fetchStatuses, fetchPriorities,
    setFilter, restoreFilters, resetFilters, hasActiveFilters,
  }
})
