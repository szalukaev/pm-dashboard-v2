<template>
  <div class="tasks-view">
    <h1 class="page-title">{{ $t('tasks.title') }}</h1>

    <TaskFilters />

    <!-- Loading state -->
    <template v-if="loading">
      <div class="skeleton-list">
        <div v-for="i in 5" :key="i" class="skeleton-row">
          <div class="skeleton-cell" style="width: 40%"></div>
          <div class="skeleton-cell" style="width: 15%"></div>
          <div class="skeleton-cell" style="width: 20%"></div>
          <div class="skeleton-cell" style="width: 10%"></div>
          <div class="skeleton-cell" style="width: 15%"></div>
        </div>
      </div>
    </template>

    <!-- Error state -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchTasks" />

    <!-- Empty state -->
    <AppEmptyState v-else-if="total === 0 && !loading" state="empty" />

    <!-- Grouped view -->
    <TaskAccordion
      v-else-if="useGrouping && groups.length > 0"
      :groups="groups"
      @open-task="openTask"
    />

    <!-- Flat table view -->
    <TaskTable
      v-else-if="tasks.length > 0"
      :tasks="tasks"
      @open-task="openTask"
      @sort-change="handleSortChange"
      @inline-edit="handleInlineEdit"
    />

    <!-- Task detail modal -->
    <TaskModal
      :task-id="selectedTaskId"
      @close="selectedTaskId = null"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useTasksStore } from '../stores/tasks'
import TaskFilters from '../components/tasks/TaskFilters.vue'
import TaskAccordion from '../components/tasks/TaskAccordion.vue'
import TaskTable from '../components/tasks/TaskTable.vue'
import TaskModal from '../components/tasks/TaskModal.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'

const store = useTasksStore()
const { tasks, groups, loading, error, total, useGrouping } = storeToRefs(store)
const { fetchTasks } = store

const selectedTaskId = ref<number | null>(null)

function openTask(id: number) {
  selectedTaskId.value = id
}

function handleSortChange(sort: { sortBy: string; sortDir: string }) {
  store.setFilter('sort_by', sort.sortBy)
  store.setFilter('sort_dir', sort.sortDir)
  store.fetchTasks()
}

async function handleInlineEdit(payload: { taskId: number; field: string; value: any }) {
  await store.updateTask(payload.taskId, { [payload.field]: payload.value })
}

onMounted(async () => {
  await store.restoreFilters()
  await Promise.all([
    store.fetchTasks(),
    store.fetchProjects(),
    store.fetchCategories(),
    store.fetchStatuses(),
  ])
})
</script>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
  margin-bottom: 24px;
}

.skeleton-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-row {
  display: flex;
  gap: 16px;
  padding: 14px 16px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 8px;
}

.skeleton-cell {
  height: 14px;
  background: var(--surface-2);
  border-radius: 4px;
  animation: skeleton-pulse 1.2s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}
</style>
