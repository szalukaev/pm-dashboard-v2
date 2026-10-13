import { useTasksStore } from '../stores/tasks'

// Цвета статусов и приоритетов. Что задано в базе — берётся из неё: цвет
// приоритета администратор выбирает в «Порядке приоритетов», а статус без
// своего цвета красится по группе, к которой он отнесён в настройках.

// Цвета статусов, как в прежней версии дашборда
const statusColors: Record<string, string> = {
  'new': '#5e6ad2',
  'in progress': '#ffd93d',
  'review': '#a855f7',
  'feedback': '#f97316',
  'bugs': '#ff6b6b',
  'testing': '#06b6d4',
  'closed': '#6bcb77',
  'tested': '#6bcb77',
  'resolved': '#6bcb77',
}

// Цвет группы статусов — для статусов, которых нет в списке выше
const groupColors: Record<string, string> = {
  open: '#5e6ad2',
  testing: '#06b6d4',
  closed: '#6bcb77',
}

const NEUTRAL = '#888888'

export function statusColor(name: string): string {
  const n = (name || '').toLowerCase()
  if (statusColors[n]) return statusColors[n]
  // Незнакомое имя: смотрим, как статус настроен в системе
  const status = useTasksStore().statuses.find(s => s.name.toLowerCase() === n)
  if (status?.is_bug) return statusColors.bugs
  if (status && groupColors[status.group]) return groupColors[status.group]
  if (n.includes('test')) return statusColors.testing
  if (n.includes('bug')) return statusColors.bugs
  return NEUTRAL
}

// Цвета приоритетов по умолчанию — пока администратор не задал свой
const priorityColors: Record<string, string> = {
  immediate: '#ff4444',
  urgent: '#ff8800',
  high: '#ffaa00',
  normal: '#4488ff',
  low: NEUTRAL,
}

// Приоритет находится по номеру или по имени — что известно вызывающему.
// Серый цвет в базе — значение «не задан»: тогда действует цвет по умолчанию.
export function priorityColor(priority: { id?: number | null; name?: string | null }): string {
  const name = (priority.name || '').toLowerCase()
  const saved = useTasksStore().priorities.find(p =>
    priority.id ? p.id === priority.id : p.name.toLowerCase() === name,
  )
  const color = (saved?.color || '').toLowerCase()
  if (color && color !== NEUTRAL) return color
  return priorityColors[(saved?.name || priority.name || '').toLowerCase()] || NEUTRAL
}
