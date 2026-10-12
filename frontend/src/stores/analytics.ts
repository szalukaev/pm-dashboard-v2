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

export interface ProjectDist {
  project_id: number
  project_name: string
  assignees: AssigneeCount[]
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

// A bar of the timeline, from start to due (YYYY-MM-DD)
export interface GanttTask {
  external_id: number
  subject: string
  project_id: number
  project_name: string
  assigned_to_name: string
  start: string
  due: string
}

export interface GanttProject {
  id: number
  name: string
}

export const useAnalyticsStore = defineStore('analytics', () => {
  const gantt = ref<{ tasks: GanttTask[]; projects: GanttProject[]; total: number }>({ tasks: [], projects: [], total: 0 })
  // The project chosen in the filter of the timeline; null — every project
  const ganttProject = ref<number | null>(null)
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

  async function fetchHealth() {
    try {
      const { data } = await axios.get('/api/analytics/health')
      health.value = data && Array.isArray(data.parts) ? data : null
    } catch {}
  }

  async function fetchGantt(): Promise<void> {
    try {
      const params = ganttProject.value === null ? {} : { project_id: ganttProject.value }
      const { data } = await axios.get('/api/analytics/gantt', { params })
      const projects: GanttProject[] = data.projects || []
      // The chosen project has no tasks with a deadline left: back to all
      if (ganttProject.value !== null && !projects.some(p => p.id === ganttProject.value)) {
        ganttProject.value = null
        return fetchGantt()
      }
      gantt.value = { tasks: data.tasks || [], projects, total: data.total || 0 }
    } catch {}
  }

  function setGanttProject(id: number | null) {
    ganttProject.value = id
    return fetchGantt()
  }

  return {
    stats, teamLoad, distribution, deadlines, hours, velocity, health, gantt, ganttProject, loading, error,
    fetchAll, fetchStats, fetchTeamLoad, fetchDistribution, fetchDeadlines, fetchHours, fetchVelocity, fetchHealth,
    fetchGantt, setGanttProject,
  }
})
