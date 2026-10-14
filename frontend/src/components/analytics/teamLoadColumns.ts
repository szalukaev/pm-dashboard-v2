// Имя таблицы «Нагрузка на команду» в настройках колонок пользователя и её
// колонки в стандартном порядке; label — ключ словаря.
export const TEAM_LOAD_TABLE = 'team_load_table'

export const TEAM_LOAD_COLUMNS = [
  { key: 'employee', label: 'tasks.table.assignee' },
  { key: 'open', label: 'analytics.stats.open' },
  { key: 'testing', label: 'analytics.stats.in_test' },
  { key: 'closed', label: 'analytics.stats.closed' },
  { key: 'overdue', label: 'analytics.stats.overdue' },
  { key: 'bugs', label: 'analytics.stats.bugs' },
  { key: 'high', label: 'analytics.stats.high_priority' },
  { key: 'no_estimate', label: 'analytics.stats.no_estimate' },
]
