// The columns of the table of tasks, in the standard order. The key is also
// the name the server sorts by.
export const TASKS_TABLE = 'tasks_table'

export const TASK_COLUMNS = [
  { key: 'external_id', label: 'tasks.table.number', sortable: true },
  { key: 'subject', label: 'tasks.table.name', sortable: true },
  { key: 'priority_name', label: 'tasks.table.priority', sortable: true },
  { key: 'assigned_to', label: 'tasks.table.responsible', sortable: true },
  { key: 'estimate', label: 'tasks.table.estimate', sortable: true },
  { key: 'fact', label: 'tasks.table.fact', sortable: true },
  { key: 'status_name', label: 'tasks.table.status', sortable: true },
  { key: 'bug_fix_hours', label: 'tasks.table.bug_fix', sortable: true },
  { key: 'bug_fix_pct', label: 'tasks.table.bug_fix_pct', sortable: true },
  { key: 'start_date', label: 'tasks.table.start_date', sortable: true },
  { key: 'due_date', label: 'tasks.table.end_date', sortable: true },
  { key: 'category_name', label: 'tasks.table.category', sortable: true },
]
