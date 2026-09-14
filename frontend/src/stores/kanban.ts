import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export interface KanbanCard {
  external_id: number
  subject: string
  project_name: string
  status_name: string
  status_id: number
  priority_name: string
  priority_id: number
  assigned_to_name: string
  assigned_to_id: number | null
  category_name: string
  due_date: string | null
  estimated_hours: number | null
  is_overdue: boolean
}

export interface KanbanColumn {
  id: string
  name: string
  icon?: string
  tasks: KanbanCard[]
}

export const useKanbanStore = defineStore('kanban', () => {
  const columns = ref<KanbanColumn[]>([])
  const mode = ref<'users' | 'statuses'>('users')
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')
  const projectId = ref('')

  async function fetchBoard() {
    loading.value = true
    error.value = ''
    try {
      const params: Record<string, string> = { mode: mode.value }
      if (projectId.value) params.project_id = projectId.value
      const { data } = await axios.get('/api/kanban/board', { params })
      columns.value = data.columns || []
      total.value = data.total || 0
    } catch {
      error.value = 'Не удалось загрузить доску'
    } finally {
      loading.value = false
    }
  }

  async function moveCard(issueId: number, targetId: string) {
    // Optimistic update
    const card = findCard(issueId)
    const sourceCol = findColumnWithCard(issueId)

    if (card && sourceCol) {
      // Remove from source
      sourceCol.tasks = sourceCol.tasks.filter(t => t.external_id !== issueId)
      // Find target column
      const targetCol = columns.value.find(c => c.id === targetId)
      if (targetCol) {
        targetCol.tasks.push(card)
      }
    }

    try {
      await axios.put('/api/kanban/move', {
        issue_id: issueId,
        target_id: targetId,
        mode: mode.value,
      })
    } catch {
      // Revert on error
      await fetchBoard()
    }
  }

  async function saveColumnOrder() {
    const order = columns.value.map(c => c.name)
    try {
      await axios.put('/api/kanban/column-order', { mode: mode.value, order })
    } catch {}
  }

  function findCard(issueId: number): KanbanCard | undefined {
    for (const col of columns.value) {
      const card = col.tasks.find(t => t.external_id === issueId)
      if (card) return card
    }
    return undefined
  }

  function findColumnWithCard(issueId: number): KanbanColumn | undefined {
    return columns.value.find(col => col.tasks.some(t => t.external_id === issueId))
  }

  function setMode(newMode: 'users' | 'statuses') {
    mode.value = newMode
    fetchBoard()
  }

  function setProject(id: string) {
    projectId.value = id
    fetchBoard()
  }

  return {
    columns, mode, total, loading, error, projectId,
    fetchBoard, moveCard, saveColumnOrder, setMode, setProject,
  }
})
