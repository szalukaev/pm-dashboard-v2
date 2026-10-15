import type { AppNotification } from '../stores/notifications'

// Переводчик из vue-i18n: ключ и параметры
type Translate = (key: string, params?: Record<string, unknown>) => string

// Дата ГГГГ-ММ-ДД как ДД.ММ.ГГГГ; пустая — прочерк
function dateText(date: unknown): string {
  if (typeof date !== 'string' || date.length < 10) return '—'
  const [y, m, d] = date.slice(0, 10).split('-')
  return `${d}.${m}.${y}`
}

// Заголовок уведомления: что произошло
export function notificationTitle(n: AppNotification, t: Translate): string {
  if (n.params.count) return t('notifications.overdue_many', { count: n.params.count })
  return t('notifications.rules.' + n.rule + '.title', {
    days: n.params.days ?? 0,
    due: dateText(n.params.due),
    // Состояние лицензии: льготный период или только чтение
    state: typeof n.params.state === 'string' ? t('notifications.license_states.' + n.params.state) : '',
  })
}

// Строки под заголовком: о какой задаче речь и что в ней изменилось
export function notificationLines(n: AppNotification, t: Translate): string[] {
  const lines: string[] = []
  if (n.params.issue_id) {
    lines.push('#' + n.params.issue_id + ' ' + (n.params.subject || ''))
  }
  // Сводка просрочек: первые задачи и сколько ещё
  for (const issue of n.params.issues || []) {
    lines.push(t('notifications.overdue_item', { id: issue.issue_id, subject: issue.subject, days: issue.days }))
  }
  const more = (n.params.count || 0) - (n.params.issues?.length || 0)
  if (n.params.count && more > 0) lines.push(t('notifications.and_more', { count: more }))
  for (const change of n.params.changes || []) {
    const isDate = change.field === 'due_date'
    lines.push(t('notifications.change', {
      field: t('notifications.fields.' + change.field),
      from: isDate ? dateText(change.from) : change.from || '—',
      to: isDate ? dateText(change.to) : change.to || '—',
    }))
  }
  if (n.params.project) lines.push(String(n.params.project))
  // Договор, счёт или спринт: название и срок
  if (typeof n.params.name === 'string' && n.params.name) lines.push(n.params.name)
  if (!n.params.issue_id && n.params.due) lines.push(t('notifications.due', { due: dateText(n.params.due) }))
  if (n.params.issued) lines.push(t('notifications.issued', { date: dateText(n.params.issued) }))
  return lines
}

// Куда ведёт уведомление: задача открывается карточкой, остальное — своей вкладкой
const NOTIFICATION_PAGES: Record<string, string> = {
  issue_list: '/tasks',
  contract: '/payments',
  invoice: '/payments',
  sprint: '/sprint',
  system: '/settings',
}

export function notificationTarget(n: AppNotification): { taskId?: number; path?: string } {
  if (n.entity_type === 'issue' && n.entity_id) return { taskId: n.entity_id }
  const path = NOTIFICATION_PAGES[n.entity_type]
  return path ? { path } : {}
}

// Вид значка уведомления на телефоне: по смыслу правила
export type NotificationKind = 'overdue' | 'soon' | 'assigned' | 'changed' | 'system'

const KINDS: Record<string, NotificationKind> = {
  task_overdue: 'overdue',
  sprint_overdue: 'overdue',
  project_overdue: 'overdue',
  contract_expired: 'overdue',
  invoice_unpaid: 'overdue',
  task_due_soon: 'soon',
  sprint_ending: 'soon',
  contract_ending: 'soon',
  task_assigned: 'assigned',
  task_changed: 'changed',
  project_task_closed: 'changed',
  project_high_priority: 'changed',
  sync_failed: 'system',
  license_state: 'system',
}

export function notificationKind(n: AppNotification): NotificationKind {
  return KINDS[n.rule] || 'changed'
}
