<template>
  <div class="team-load-section">
    <div class="section-head">
      <h3 class="section-title">{{ $t('analytics.team_load') }}</h3>
      <AppColumnSettings :table="TABLE" :columns="COLUMNS" />
    </div>
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
              <td v-else-if="col.key === 'open'" class="num-cell">{{ row.open }}</td>
              <td v-else-if="col.key === 'testing'" class="num-cell">{{ row.testing }}</td>
              <td v-else-if="col.key === 'closed'" class="num-cell">{{ row.closed }}</td>
              <td v-else-if="col.key === 'overdue'" class="num-cell" :class="{ 'danger-text': row.overdue > 0 }">{{ row.overdue }}</td>
              <td v-else-if="col.key === 'bugs'" class="num-cell" :class="{ 'danger-text': row.bugs > 0 }">{{ row.bugs }}</td>
              <td v-else-if="col.key === 'high'" class="num-cell">{{ row.high_priority }}</td>
              <td v-else-if="col.key === 'no_estimate'" class="num-cell" :class="{ 'warning-text': row.no_estimate > 0 }">{{ row.no_estimate }}</td>
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
import AppColumnSettings from '../ui/AppColumnSettings.vue'

defineProps<{ team: TeamLoadRow[] }>()

// Name of the table in the user's column settings and its columns in the
// standard order
const TABLE = 'team_load_table'
const COLUMNS = [
  { key: 'employee', label: 'tasks.table.assignee' },
  { key: 'open', label: 'analytics.stats.open' },
  { key: 'testing', label: 'analytics.stats.in_test' },
  { key: 'closed', label: 'tasks.table.status' },
  { key: 'overdue', label: 'analytics.stats.overdue' },
  { key: 'bugs', label: 'analytics.stats.bugs' },
  { key: 'high', label: 'analytics.stats.high_priority' },
  { key: 'no_estimate', label: 'analytics.stats.no_estimate' },
]

const settingsStore = useSettingsStore()
const shownColumns = computed(() =>
  settingsStore.tableColumns(TABLE, COLUMNS.map(c => c.key)).map(key => COLUMNS.find(c => c.key === key)!)
)
</script>

<style scoped>
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.2px;
}

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
  text-align: right;
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
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.danger-text {
  color: var(--danger);
  font-weight: 500;
}

.warning-text {
  color: var(--warning);
}
</style>
