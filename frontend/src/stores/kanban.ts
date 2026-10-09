import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import i18n from '../i18n'
import { useSwal } from '../composables/useSwal'

// Id of the "no assignee" column in the users mode
export const UNASSIGNED = 'unassigned'

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

  // The board opens in the mode and on the project the user left it
  async function restoreState() {
    try {
      const { data } = await axios.get('/api/settings')
      mode.value = data.kanban_last_mode === 'statuses' ? 'statuses' : 'users'
      projectId.value = data.kanban_last_project ? String(data.kanban_last_project) : ''
    } catch {}
  }

  function saveState(fields: Record<string, unknown>) {
    axios.put('/api/settings', fields).catch(() => {})
  }

  async function fetchBoard() {
    // By statuses the board needs a project; the view asks to choose one
    if (mode.value === 'statuses' && !projectId.value) {
      columns.value = []
      total.value = 0
      return
    }
    loading.value = true
    error.value = ''
    try {
      const params: Record<string, string> = { mode: mode.value }
      if (projectId.value) params.project_id = projectId.value
      const { data } = await axios.get('/api/kanban/board', { params })
      columns.value = data.columns || []
      total.value = data.total || 0
    } catch {
      error.value = i18n.global.t('kanban.load_error')
    } finally {
      loading.value = false
    }
  }

  async function moveCard(issueId: number, targetId: string) {
    const card = findCard(issueId)
    const sourceCol = findColumnWithCard(issueId)
    const targetCol = columns.value.find(c => c.id === targetId)
    if (!card || !sourceCol || !targetCol || sourceCol.id === targetId) return

    const { toast, showChange } = useSwal()
    const t = i18n.global.t
    const byStatuses = mode.value === 'statuses'
    const columnTitle = (col: KanbanColumn) => (col.id === UNASSIGNED ? t('kanban.unassigned') : col.name)

    // Optimistic update; everything needed to undo it is kept
    const sourceIndex = sourceCol.tasks.indexOf(card)
    const before = {
      status_id: card.status_id,
      status_name: card.status_name,
      assigned_to_id: card.assigned_to_id,
      assigned_to_name: card.assigned_to_name,
    }
    sourceCol.tasks.splice(sourceIndex, 1)
    targetCol.tasks.push(card)
    // The card itself shows status and assignee
    if (byStatuses) {
      card.status_id = Number(targetCol.id)
      card.status_name = targetCol.name
    } else {
      card.assigned_to_id = targetCol.id === UNASSIGNED ? null : Number(targetCol.id)
      card.assigned_to_name = targetCol.name
    }

    try {
      await axios.put('/api/kanban/move', {
        issue_id: issueId,
        target_id: targetId,
        mode: mode.value,
      })
      showChange(
        columnTitle(sourceCol),
        columnTitle(targetCol),
        `#${issueId} — ${t(byStatuses ? 'tasks.fields.status' : 'tasks.fields.assignee')}`,
      )
    } catch (e: any) {
      // The data source did not take the change: put just this card back
      // where it was (no board reload) and say why
      Object.assign(card, before)
      const at = targetCol.tasks.indexOf(card)
      if (at >= 0) targetCol.tasks.splice(at, 1)
      sourceCol.tasks.splice(Math.min(sourceIndex, sourceCol.tasks.length), 0, card)
      const data = e?.response?.data || {}
      const message = data.error === 'REDMINE_CHANGE_REJECTED' ? t('kanban.move_rejected')
        : data.error === 'REDMINE_UPDATE_FAILED' ? t('kanban.move_redmine_error', { message: data.message || '' })
        : data.error === 'DATA_SOURCE_NOT_CONFIGURED' ? t('kanban.move_no_source')
        : data.error === 'FORBIDDEN' ? t('kanban.move_forbidden')
        : t('kanban.move_error')
      toast(message, 'error')
    }
  }

  async function saveColumnOrder() {
    const order = columns.value.map(c => c.id)
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

  // Puts the dragged column on the place of the one it was dropped on
  function moveColumn(fromId: string, toId: string) {
    const from = columns.value.findIndex(c => c.id === fromId)
    const to = columns.value.findIndex(c => c.id === toId)
    if (from < 0 || to < 0 || from === to) return
    const reordered = [...columns.value]
    const [moved] = reordered.splice(from, 1)
    reordered.splice(to, 0, moved)
    columns.value = reordered
    saveColumnOrder()
  }

  function setMode(newMode: 'users' | 'statuses') {
    mode.value = newMode
    saveState({ kanban_last_mode: newMode })
    fetchBoard()
  }

  function setProject(id: string) {
    projectId.value = id
    saveState({ kanban_last_project: id ? Number(id) : null })
    fetchBoard()
  }

  return {
    columns, mode, total, loading, error, projectId,
    restoreState, fetchBoard, moveCard, moveColumn, saveColumnOrder, setMode, setProject,
  }
})
