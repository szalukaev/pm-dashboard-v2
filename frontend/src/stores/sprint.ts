import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import i18n from '../i18n'

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
  // Группа статуса: open, testing или closed
  status_group: string
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
  // В чём считается продолжительность: рабочие дни или календарные
  duration_unit?: 'workdays' | 'calendar'
  // Общий спринт виден команде создателя и забирает задачи из бэклога;
  // личный виден только создателю
  is_shared: boolean
  // Создан ли спринт самим пользователем и кем он создан
  is_own: boolean
  owner_name: string
  // Открыт, а дата окончания прошла
  is_overdue: boolean
  // Когда закрыт и закрыт ли позже даты окончания
  closed_at: string | null
  closed_late: boolean
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

// Под этим типом в данных перетаскивания лежит номер спринта, из которого
// тянут задачу; у задачи из бэклога его нет
export const SPRINT_DRAG_TYPE = 'application/x-sprint-id'

export const useSprintStore = defineStore('sprint', () => {
  const sprints = ref<Sprint[]>([])
  const backlog = ref<BacklogGroup[]>([])
  const loading = ref(false)
  const error = ref('')

  // silent: обновление в фоне, без индикатора загрузки.
  async function fetchAll(opts: { silent?: boolean } = {}) {
    const silent = opts.silent === true
    if (!silent) {
      loading.value = true
      error.value = ''
    }
    // Сбой показывается ошибкой с повтором, а не пустыми списками
    try {
      await Promise.all([loadSprints(), loadBacklog()])
    } catch {
      if (!silent) error.value = i18n.global.t('common.error')
    } finally {
      if (!silent) loading.value = false
    }
  }

  async function loadSprints() {
    const { data } = await axios.get('/api/sprints')
    sprints.value = data.sprints || []
  }

  async function loadBacklog() {
    const { data } = await axios.get('/api/sprints/backlog')
    backlog.value = data.backlog || []
  }

  // Обновление после изменения: оно уже сохранено, сбой обновления не показывается
  async function fetchSprints() {
    try { await loadSprints() } catch {}
  }

  async function fetchBacklog() {
    try { await loadBacklog() } catch {}
  }

  async function createSprint(payload: Partial<Sprint>) {
    const { data } = await axios.post('/api/sprints', payload)
    await fetchAll({ silent: true })
    return data
  }

  async function updateSprint(id: number, payload: Partial<Sprint>) {
    await axios.put(`/api/sprints/${id}`, payload)
    await fetchAll({ silent: true })
  }

  async function deleteSprint(id: number) {
    await axios.delete(`/api/sprints/${id}`)
    await fetchAll({ silent: true })
  }

  // Пересчитывает счётчики и готовность спринта по его задачам — так же, как
  // сервер: закрытая задача — целая доля, задача в тестировании — половина
  function recount(sprint: Sprint) {
    const count = (group: string) => sprint.tasks.filter(t => t.status_group === group).length
    sprint.task_count = sprint.tasks.length
    sprint.testing_count = count('testing')
    sprint.closed_count = count('closed')
    sprint.open_count = sprint.task_count - sprint.testing_count - sprint.closed_count
    sprint.progress = sprint.task_count > 0
      ? ((sprint.closed_count + sprint.testing_count * 0.5) / sprint.task_count) * 100
      : 0
  }

  // Порядок задач в спринте: открытые, в тестировании, закрытые
  const GROUP_ORDER: Record<string, number> = { open: 0, testing: 1, closed: 2 }

  function removeFromSprint(sprint: Sprint, issueId: number): SprintTask | undefined {
    const task = sprint.tasks.find(t => t.external_id === issueId)
    if (task) {
      sprint.tasks = sprint.tasks.filter(t => t.external_id !== issueId)
      recount(sprint)
    }
    return task
  }

  function removeFromBacklog(issueId: number): SprintTask | undefined {
    let found: SprintTask | undefined
    backlog.value = backlog.value.flatMap(group => {
      const task = group.tasks.find(t => t.external_id === issueId)
      if (!task) return [group]
      found = task
      const tasks = group.tasks.filter(t => t.external_id !== issueId)
      return tasks.length > 0 ? [{ ...group, tasks, task_count: tasks.length }] : []
    })
    return found
  }

  // В бэклоге лежат только открытые задачи
  function addToBacklog(task: SprintTask) {
    if (task.status_group !== 'open') return
    if (backlog.value.some(g => g.tasks.some(t => t.external_id === task.external_id))) return
    const group = backlog.value.find(g => g.project_name === task.project_name)
    if (group) {
      group.tasks = [...group.tasks, task]
      group.task_count = group.tasks.length
    } else {
      backlog.value = [...backlog.value, { project_name: task.project_name, tasks: [task], task_count: 1 }]
    }
  }

  // Переносит задачу: в спринт toSprintId или, если он null, обратно в
  // бэклог; fromSprintId — спринт, из которого её взяли (null — из бэклога).
  // Экран меняется сразу, запрос идёт следом; после ответа данные сверяются
  // с сервером, а при ошибке перечитываются и ошибка отдаётся вызвавшему.
  async function moveTask(issueId: number, fromSprintId: number | null, toSprintId: number | null) {
    if (fromSprintId === toSprintId) return
    const from = fromSprintId === null ? undefined : sprints.value.find(s => s.id === fromSprintId)
    const to = toSprintId === null ? undefined : sprints.value.find(s => s.id === toSprintId)

    let task = from ? removeFromSprint(from, issueId) : undefined
    if (to) {
      // Задача лежит только в одном незакрытом спринте своего вида
      for (const other of sprints.value) {
        if (other.id !== to.id && other.status !== 'closed' && other.is_shared === to.is_shared) {
          task = removeFromSprint(other, issueId) || task
        }
      }
      // Общий спринт забирает задачу из бэклога, личный — нет
      const inBacklog = to.is_shared ? removeFromBacklog(issueId) : backlog.value.flatMap(g => g.tasks).find(t => t.external_id === issueId)
      task = task || inBacklog
      if (task && !to.tasks.some(t => t.external_id === issueId)) {
        to.tasks = [...to.tasks, task].sort((a, b) => (GROUP_ORDER[a.status_group] ?? 0) - (GROUP_ORDER[b.status_group] ?? 0))
        recount(to)
      }
    }
    // Задача, вышедшая из общего спринта, снова доступна в бэклоге
    if (task && from?.is_shared && !to?.is_shared) addToBacklog(task)

    try {
      if (toSprintId !== null) {
        await axios.post(`/api/sprints/${toSprintId}/assign`, {
          issue_external_id: issueId,
          from_sprint_id: fromSprintId ?? 0,
        })
      } else if (fromSprintId !== null) {
        await axios.delete(`/api/sprints/${fromSprintId}/assign/${issueId}`)
      }
    } catch (e) {
      await fetchAll({ silent: true })
      throw e
    }
    await fetchAll({ silent: true })
  }

  function assignTask(sprintId: number, issueExternalId: number, fromSprintId: number | null = null) {
    return moveTask(issueExternalId, fromSprintId, sprintId)
  }

  function unassignTask(sprintId: number, issueExternalId: number) {
    return moveTask(issueExternalId, sprintId, null)
  }

  async function refreshSprint(id: number) {
    await axios.post(`/api/sprints/${id}/refresh`)
    await fetchAll({ silent: true })
  }

  return {
    sprints, backlog, loading, error,
    fetchAll, fetchSprints, fetchBacklog,
    createSprint, updateSprint, deleteSprint,
    assignTask, unassignTask, refreshSprint,
  }
})
