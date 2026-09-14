<template>
  <div class="team-load-section">
    <h3 class="section-title">{{ $t('analytics.team_load') }}</h3>
    <div class="table-wrapper">
      <table class="team-table" data-testid="team-load-table">
        <thead>
          <tr>
            <th>{{ $t('tasks.table.assignee') }}</th>
            <th class="num">{{ $t('analytics.stats.open') }}</th>
            <th class="num">{{ $t('analytics.stats.in_test') }}</th>
            <th class="num">{{ $t('tasks.table.status') }}</th>
            <th class="num">{{ $t('analytics.stats.overdue') }}</th>
            <th class="num">{{ $t('analytics.stats.bugs') }}</th>
            <th class="num">Высокий</th>
            <th class="num">{{ $t('analytics.stats.no_estimate') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in team" :key="row.name" data-testid="team-row">
            <td class="name-cell">{{ row.name }}</td>
            <td class="num-cell">{{ row.open }}</td>
            <td class="num-cell">{{ row.testing }}</td>
            <td class="num-cell">{{ row.closed }}</td>
            <td class="num-cell" :class="{ 'danger-text': row.overdue > 0 }">{{ row.overdue }}</td>
            <td class="num-cell" :class="{ 'danger-text': row.bugs > 0 }">{{ row.bugs }}</td>
            <td class="num-cell">{{ row.high_priority }}</td>
            <td class="num-cell" :class="{ 'warning-text': row.no_estimate > 0 }">{{ row.no_estimate }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { TeamLoadRow } from '../../stores/analytics'

defineProps<{ team: TeamLoadRow[] }>()
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
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
