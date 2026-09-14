import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export interface SprintTask {
  external_id: number
  subject: string
  project_name: string
  status_name: string
  priority_name: string
  priority_id: number
  assigned_to_name: string
  due_date: string | null
  estimated_hours: number | null
  is_overdue: boolean
}

export interface Sprint {
  id: number
  name: string
  project_name: string | null
  status: string
  start_date: string | null
  due_date: string | null
  description: string | null
  category_name: string | null
  auto_fill_category: boolean
  tasks: SprintTask[]
  task_count: number
  open_count: number
  testing_count: number
  closed_count: number
  progress: number
}

export interface BacklogGroup {
  project_name: string
  tasks: SprintTask[]
  task_count: number
}

export const useSprintStore = defineStore('sprint', () => {
  const sprints = ref<Sprint[]>([])
  const backlog = ref<BacklogGroup[]>([])
  const loading = ref(false)
  const error = ref('')

  async function fetchAll() {
    loading.value = true
    error.value = ''
    try {
      await Promise.all([fetchSprints(), fetchBacklog()])
    } catch {
      error.value = 'Не удалось загрузить данные'
    } finally {
      loading.value = false
    }
  }

  async function fetchSprints() {
    try {
      const { data } = await axios.get('/api/sprints')
      sprints.value = data.sprints || []
    } catch {}
  }

  async function fetchBacklog() {
    try {
      const { data } = await axios.get('/api/sprints/backlog')
      backlog.value = data.backlog || []
    } catch {}
  }

  async function createSprint(payload: Partial<Sprint>) {
    const { data } = await axios.post('/api/sprints', payload)
    await fetchAll()
    return data
  }

  async function updateSprint(id: number, payload: Partial<Sprint>) {
    await axios.put(`/api/sprints/${id}`, payload)
    await fetchAll()
  }

  async function deleteSprint(id: number) {
    await axios.delete(`/api/sprints/${id}`)
    await fetchAll()
  }

  async function assignTask(sprintId: number, issueExternalId: number) {
    await axios.post(`/api/sprints/${sprintId}/assign`, { issue_external_id: issueExternalId })
    await fetchAll()
  }

  async function unassignTask(sprintId: number, issueExternalId: number) {
    await axios.delete(`/api/sprints/${sprintId}/assign/${issueExternalId}`)
    await fetchAll()
  }

  async function refreshSprint(id: number) {
    await axios.post(`/api/sprints/${id}/refresh`)
    await fetchAll()
  }

  return {
    sprints, backlog, loading, error,
    fetchAll, fetchSprints, fetchBacklog,
    createSprint, updateSprint, deleteSprint,
    assignTask, unassignTask, refreshSprint,
  }
})
