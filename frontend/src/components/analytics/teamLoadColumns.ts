// Name of the «Team load» table in the user's column settings and its columns
// in the standard order; label is a dictionary key.
export const TEAM_LOAD_TABLE = 'team_load_table'

export const TEAM_LOAD_COLUMNS = [
  { key: 'employee', label: 'tasks.table.assignee' },
  { key: 'open', label: 'analytics.stats.open' },
  { key: 'testing', label: 'analytics.stats.in_test' },
  { key: 'closed', label: 'tasks.table.status' },
  { key: 'overdue', label: 'analytics.stats.overdue' },
  { key: 'bugs', label: 'analytics.stats.bugs' },
  { key: 'high', label: 'analytics.stats.high_priority' },
  { key: 'no_estimate', label: 'analytics.stats.no_estimate' },
]
