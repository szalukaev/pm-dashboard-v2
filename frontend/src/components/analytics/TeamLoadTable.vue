<template>
  <div class="team-load-section">
    <div class="table-wrapper">
      <table class="team-table" data-testid="team-load-table">
        <thead>
          <tr>
            <th v-for="col in shownColumns" :key="col.key" :class="{ num: col.key !== 'employee' }">{{ $t(col.label) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in team" :key="row.name" data-testid="team-row">
            <!-- The cells follow the columns the user chose and their order -->
            <template v-for="col in shownColumns" :key="col.key">
              <td v-if="col.key === 'employee'" class="name-cell">{{ row.name }}</td>
              <!-- A figure opens the tasks it counts -->
              <td v-else class="num-cell">
                <button
                  v-if="count(row, col.key) > 0"
                  type="button"
                  class="num-btn"
                  :class="tone(col.key)"
                  :title="$t('analytics.show_tasks')"
                  @click="$emit('open-tasks', row.name, METRICS[col.key], col.label)"
                >
                  {{ count(row, col.key) }}
                </button>
                <span v-else>0</span>
              </td>
            </template>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TeamLoadRow } from '../../stores/analytics'
import { useSettingsStore } from '../../stores/settings'
import { TEAM_LOAD_TABLE as TABLE, TEAM_LOAD_COLUMNS as COLUMNS } from './teamLoadColumns'

// The button that sets up the columns lives in the header of the widget
defineProps<{ team: TeamLoadRow[] }>()

// open-tasks: a figure was clicked — the employee, the figure in the list of
// tasks behind it and the dictionary key of the column
defineEmits<{
  (e: 'open-tasks', name: string, metric: string, label: string): void
}>()

// The figure each column of numbers stands for
const METRICS: Record<string, keyof TeamLoadRow> = {
  open: 'open',
  testing: 'testing',
  closed: 'closed',
  overdue: 'overdue',
  bugs: 'bugs',
  high: 'high_priority',
  no_estimate: 'no_estimate',
}

function count(row: TeamLoadRow, column: string): number {
  const value = row[METRICS[column]]
  return typeof value === 'number' ? value : 0
}

// Problems stand out
function tone(column: string): string {
  if (column === 'overdue' || column === 'bugs') return 'danger-text'
  return column === 'no_estimate' ? 'warning-text' : ''
}

const settingsStore = useSettingsStore()
const shownColumns = computed(() =>
  settingsStore.tableColumns(TABLE, COLUMNS.map(c => c.key)).map(key => COLUMNS.find(c => c.key === key)!)
)
</script>

<style scoped>
.table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.team-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.team-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
  position: sticky;
  top: 0;
  white-space: nowrap;
}

.team-table th.num {
  text-align: center;
}

.team-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
}

.team-table tr:hover td {
  background: var(--bg-hover);
}

.name-cell {
  font-weight: 500;
  color: var(--text);
}

.num-cell {
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.num-btn {
  min-width: 32px;
  padding: 2px 8px;
  border: none;
  border-radius: 8px;
  background: none;
  font: inherit;
  color: var(--text);
  cursor: pointer;
}

.num-btn:hover {
  background: var(--accent-bg);
  color: var(--accent);
}

.num-btn.danger-text {
  color: var(--danger);
}

.num-btn.warning-text {
  color: var(--warning);
}

.danger-text {
  color: var(--danger);
  font-weight: 500;
}

.warning-text {
  color: var(--warning);
}
</style>
