import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

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
  project_name: string
  assignees: AssigneeCount[]
}

export interface DeadlineGroup {
  name: string
  tasks: any[]
}

export const useAnalyticsStore = defineStore('analytics', () => {
  const stats = ref<StatCard[]>([])
  const teamLoad = ref<TeamLoadRow[]>([])
  const distribution = ref<ProjectDist[]>([])
  const deadlines = ref<DeadlineGroup[]>([])
  const loading = ref(false)
  const error = ref('')

  async function fetchAll() {
    loading.value = true
    error.value = ''
    try {
      await Promise.all([
        fetchStats(),
        fetchTeamLoad(),
        fetchDistribution(),
        fetchDeadlines(),
      ])
    } catch {
      error.value = 'Не удалось загрузить данные'
    } finally {
      loading.value = false
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

  return {
    stats, teamLoad, distribution, deadlines, loading, error,
    fetchAll, fetchStats, fetchTeamLoad, fetchDistribution, fetchDeadlines,
  }
})
