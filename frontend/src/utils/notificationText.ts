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
  return t('notifications.rules.' + n.rule + '.title', {
    days: n.params.days ?? 0,
    due: dateText(n.params.due),
  })
}

// Строки под заголовком: о какой задаче речь и что в ней изменилось
export function notificationLines(n: AppNotification, t: Translate): string[] {
  const lines: string[] = []
  if (n.params.issue_id) {
    lines.push('#' + n.params.issue_id + ' ' + (n.params.subject || ''))
  }
  for (const change of n.params.changes || []) {
    const isDate = change.field === 'due_date'
    lines.push(t('notifications.change', {
      field: t('notifications.fields.' + change.field),
      from: isDate ? dateText(change.from) : change.from || '—',
      to: isDate ? dateText(change.to) : change.to || '—',
    }))
  }
  if (n.params.project) lines.push(String(n.params.project))
  return lines
}
