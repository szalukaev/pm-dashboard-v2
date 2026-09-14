<template>
  <div class="kanban-toolbar">
    <div class="mode-switch">
      <button
        class="pill-btn"
        :class="{ active: mode === 'users' }"
        @click="$emit('change-mode', 'users')"
        data-testid="kanban-mode-users"
      >
        {{ $t('kanban.mode_by_users') }}
      </button>
      <button
        class="pill-btn"
        :class="{ active: mode === 'statuses' }"
        @click="$emit('change-mode', 'statuses')"
        data-testid="kanban-mode-statuses"
      >
        {{ $t('kanban.mode_by_statuses') }}
      </button>
    </div>

    <select
      v-if="mode === 'statuses'"
      :value="projectId"
      @change="$emit('change-project', ($event.target as HTMLSelectElement).value)"
      class="project-select"
      data-testid="kanban-project-select"
    >
      <option value="">{{ $t('kanban.select_project') }}</option>
      <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
    </select>

    <span class="total-count">
      {{ total }} {{ $t('kanban.total_tasks') }}
    </span>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  mode: 'users' | 'statuses'
  projectId: string
  total: number
  projects: { id: number; name: string }[]
}>()

defineEmits(['change-mode', 'change-project'])
</script>

<style scoped>
.kanban-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.mode-switch {
  display: flex;
  gap: 4px;
}

.pill-btn {
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 9999px;
  background: transparent;
  border: 1px solid var(--hairline);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.pill-btn:hover {
  border-color: var(--text-faint);
  color: var(--text-bright);
}

.pill-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.project-select {
  padding: 6px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  min-width: 200px;
}

.total-count {
  font-size: 13px;
  color: var(--text-muted);
  margin-left: auto;
}
</style>
