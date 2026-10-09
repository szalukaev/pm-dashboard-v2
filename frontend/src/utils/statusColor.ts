// Colors of the statuses, as in the previous version of the dashboard
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

export function statusColor(name: string): string {
  const n = (name || '').toLowerCase()
  if (statusColors[n]) return statusColors[n]
  if (n.includes('test')) return statusColors.testing
  if (n.includes('bug')) return statusColors.bugs
  return '#888888'
}
