import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import i18n from '../i18n'

export interface StatCard {
  label: string
  value: number
  is_pct: boolean
  variant: string
}

export interface TeamLoadRow {
  name: string
  open: number
  testing: number
  closed: number
  overdue: number
  bugs: number
  high_priority: number
  no_estimate: number
}

export interface AssigneeCount {
  name: string
  count: number
}

// root_id / root_name: the topmost parent of the project, the project itself
// when it has none
export interface ProjectDist {
  project_id: number
  project_name: string
  root_id: number
  root_name: string
  assignees: AssigneeCount[]
}

// A chart of the distribution: a project, or a parent project together with
// its nested ones — then parts are the projects it is made of
export interface DistChart {
  project_id: number
  project_name: string
  assignees: AssigneeCount[]
  parts?: ProjectDist[]
}

export interface DeadlineGroup {
  name: string
  tasks: any[]
}

// Hours of an employee over their tasks that are not closed
export interface HoursRow {
  name: string
  estimated_hours: number
  spent_hours: number
  bug_hours: number
  bug_percent: number
}

// Tasks closed during a week; week_start is its Monday, YYYY-MM-DD
export interface VelocityWeek {
  week_start: string
  closed: number
  estimated_hours: number
}

// One of the problems that lower the health score: share is the percent of
// the active tasks, penalty is the points taken off
export interface HealthPart {
  key: string
  count: number
  share: number
  penalty: number
}

// Health of the projects, 0–100, over the active tasks
export interface Health {
  score: number
  level: 'good' | 'warning' | 'bad'
  active: number
  parts: HealthPart[]
}

// The tasks behind a figure of the tab. metric is the figure; assignee, when
// given, is the employee whose row of the team load table was clicked —
// otherwise the figure is a card of the statistics.
export interface TaskListQuery {
  title: string
  metric: string
  assignee?: string
}

// A bar of the timeline, from start to due (YYYY-MM-DD)
export interface GanttTask {
  external_id: number
  subject: string
  project_id: number
  project_name: string
  assigned_to_name: string
  start: string
  due: string
  // Hours planned for the task; null when it has no estimate
  estimated_hours: number | null
}

export interface GanttProject {
  id: number
  name: string
}

// Превышение порога индикатора: какой индикатор (overdue, bugs, no_estimate),
// сколько задач и какой порог. project_name пуст, когда порог сравнивался со
// всеми проектами сразу.
export interface Exceeded {
  indicator: string
  value: number
  limit: number
  project_id: number
  project_name: string
}

export const useAnalyticsStore = defineStore('analytics', () => {
  const exceeded = ref<Exceeded[]>([])
  const gantt = ref<{ tasks: GanttTask[]; projects: GanttProject[]; total: number }>({ tasks: [], projects: [], total: 0 })
  // The project chosen in the filter of the timeline; null — every project
  const ganttProject = ref<number | null>(null)
  // The first task of the shown page of the timeline
  const ganttOffset = ref(0)
  const health = ref<Health | null>(null)
  const stats = ref<StatCard[]>([])
  const hours = ref<HoursRow[]>([])
  const velocity = ref<VelocityWeek[]>([])
  const teamLoad = ref<TeamLoadRow[]>([])
  const distribution = ref<ProjectDist[]>([])
  const deadlines = ref<DeadlineGroup[]>([])
  const loading = ref(false)
  const error = ref('')

  // silent: refresh in the background, without the loading skeleton.
  async function fetchAll(opts: { silent?: boolean } = {}) {
    const silent = opts.silent === true
    if (!silent) {
      loading.value = true
      error.value = ''
    }
    try {
      await Promise.all([
        fetchStats(),
        fetchTeamLoad(),
        fetchDistribution(),
        fetchDeadlines(),
        fetchHours(),
        fetchVelocity(),
        fetchHealth(),
        fetchGantt(),
        fetchThresholds(),
      ])
    } catch {
      if (!silent) error.value = i18n.global.t('common.error')
    } finally {
      if (!silent) loading.value = false
    }
  }

  async function fetchStats() {
    try {
      const { data } = await axios.get('/api/analytics/stats')
      stats.value = data.stats || []
    } catch {}
  }

  async function fetchTeamLoad() {
    try {
      const { data } = await axios.get('/api/analytics/team-load')
      teamLoad.value = data.team || []
    } catch {}
  }

  async function fetchDistribution() {
    try {
      const { data } = await axios.get('/api/analytics/distribution')
      distribution.value = data.distribution || []
    } catch {}
  }

  async function fetchDeadlines() {
    try {
      const { data } = await axios.get('/api/analytics/deadlines')
      deadlines.value = data.deadlines || []
    } catch {}
  }

  async function fetchHours() {
    try {
      const { data } = await axios.get('/api/analytics/hours')
      hours.value = data.hours || []
    } catch {}
  }

  async function fetchVelocity() {
    try {
      const { data } = await axios.get('/api/analytics/velocity')
      velocity.value = data.weeks || []
    } catch {}
  }

  // Какие индикаторы сейчас выше порогов пользователя
  async function fetchThresholds() {
    try {
      const { data } = await axios.get('/api/analytics/thresholds')
      exceeded.value = data.exceeded || []
    } catch {}
  }

  async function fetchHealth() {
    try {
      const { data } = await axios.get('/api/analytics/health')
      health.value = data && Array.isArray(data.parts) ? data : null
    } catch {}
  }

  async function fetchGantt(): Promise<void> {
    try {
      const params: Record<string, number> = { offset: ganttOffset.value }
      if (ganttProject.value !== null) params.project_id = ganttProject.value
      const { data } = await axios.get('/api/analytics/gantt', { params })
      const projects: GanttProject[] = data.projects || []
      const tasks: GanttTask[] = data.tasks || []
      const total: number = data.total || 0
      // The chosen project has no tasks with a deadline left: back to all
      if (ganttProject.value !== null && !projects.some(p => p.id === ganttProject.value)) {
        ganttProject.value = null
        ganttOffset.value = 0
        return fetchGantt()
      }
      // Tasks were closed and the page is past the end now: back to the first
      if (tasks.length === 0 && total > 0 && ganttOffset.value > 0) {
        ganttOffset.value = 0
        return fetchGantt()
      }
      gantt.value = { tasks, projects, total }
    } catch {}
  }

  function setGanttProject(id: number | null) {
    ganttProject.value = id
    ganttOffset.value = 0
    return fetchGantt()
  }

  function setGanttOffset(offset: number) {
    ganttOffset.value = offset
    return fetchGantt()
  }

  return {
    stats, teamLoad, distribution, deadlines, hours, velocity, health, gantt, ganttProject, loading, error,
    fetchAll, fetchStats, fetchTeamLoad, fetchDistribution, fetchDeadlines, fetchHours, fetchVelocity, fetchHealth,
    fetchGantt, setGanttProject, ganttOffset, setGanttOffset, exceeded, fetchThresholds,
  }
})
