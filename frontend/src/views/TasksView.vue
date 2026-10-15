<template>
  <div class="tasks-view">
    <h1 class="page-title">{{ $t('tasks.title') }}</h1>

    <TaskFilters />

    <!-- Телефон: переключатель вида списка -->
    <div v-if="isMobile" class="mobile-mode" role="tablist" data-testid="tasks-mobile-mode">
      <button
        v-for="m in MOBILE_MODES"
        :key="m.value"
        role="tab"
        :aria-selected="mode === m.value"
        :class="{ active: mode === m.value }"
        :data-testid="'tasks-mode-' + m.value"
        @click="setMode(m.value)"
      >
        <component :is="m.icon" :size="15" />
        {{ $t(m.label) }}
      </button>
    </div>

    <!-- Загрузка -->
    <AppLoader v-if="loading" :text="$t('common.loader.tasks')" />

    <!-- Ошибка -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchTasks()" />

    <!-- Пусто: строк нет из-за действующих фильтров -->
    <div v-else-if="total === 0 && !loading && hasActiveFilters()" class="filtered-empty" data-testid="filtered-empty">
      <p class="filtered-empty-title">{{ $t('tasks.empty.filtered_title') }}</p>
      <p class="filtered-empty-hint">{{ $t('tasks.empty.filtered_hint') }}</p>
      <button class="reset-filters-btn" data-testid="reset-filters" @click="resetFilters">
        {{ $t('tasks.empty.reset_filters') }}
      </button>
    </div>

    <!-- Пусто -->
    <AppEmptyState v-else-if="total === 0 && !loading" state="empty" />

    <!-- Вид с группировкой -->
    <TaskAccordion
      v-else-if="useGrouping && groups.length > 0"
      :groups="groups"
      :group-by="filters.group_by"
      @open-task="openTask"
      @sort-change="handleSortChange"
    />

    <!-- Плоская таблица -->
    <template v-else-if="tasks.length > 0">
      <TaskTable
        :tasks="tasks"
        @open-task="openTask"
        @sort-change="handleSortChange"
        @inline-edit="handleInlineEdit"
      />
      <AppPagination
        :total="total"
        :limit="pageSize"
        :offset="offset"
        @update:offset="store.setPage"
        @update:limit="store.setPageSize"
      />
    </template>

    <!-- Карточка задачи -->
    <TaskModal
      :task-id="selectedTaskId"
      @close="selectedTaskId = null"
    />
  </div>
</template>

<script setup lang="ts">
import AppLoader from '../components/ui/AppLoader.vue'
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useTasksStore } from '../stores/tasks'
import { useLiveRefresh } from '../composables/useLiveRefresh'
import { useMobileTasks, type TaskMobileMode } from '../composables/useMobileTasks'
import { LayoutList, CalendarClock } from 'lucide-vue-next'
import TaskFilters from '../components/tasks/TaskFilters.vue'
import TaskAccordion from '../components/tasks/TaskAccordion.vue'
import TaskTable from '../components/tasks/TaskTable.vue'
import TaskModal from '../components/tasks/TaskModal.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppPagination from '../components/ui/AppPagination.vue'

const store = useTasksStore()
const { tasks, groups, loading, error, total, useGrouping, filters, offset, pageSize } = storeToRefs(store)
const { fetchTasks, resetFilters, hasActiveFilters } = store

const selectedTaskId = ref<number | null>(null)

const { isMobile, mode, setMode } = useMobileTasks()
const MOBILE_MODES: { value: TaskMobileMode; label: string; icon: unknown }[] = [
  { value: 'cards', label: 'tasks.mobile.view_cards', icon: LayoutList },
  { value: 'deadlines', label: 'tasks.mobile.view_deadlines', icon: CalendarClock },
]

function openTask(id: number) {
  selectedTaskId.value = id
}

function handleSortChange(sort: { sortBy: string; sortDir: string }) {
  store.setFilter('sort_by', sort.sortBy)
  store.setFilter('sort_dir', sort.sortDir)
  // Тихо: таблица остаётся на месте, меняются только её строки
  store.fetchTasks({ silent: true })
}

async function handleInlineEdit(payload: { taskId: number; field: string; value: any }) {
  await store.updateTask(payload.taskId, { [payload.field]: payload.value })
}

// Изменения от синхронизации и других пользователей появляются без перезагрузки
useLiveRefresh(() => store.fetchTasks({ silent: true }))

onMounted(async () => {
  await store.restoreFilters()
  await Promise.all([
    store.fetchTasks(),
    store.fetchProjects(),
    // Категории зависят от восстановленного фильтра проекта
    store.fetchCategories(store.filters.project_id || undefined),
    store.fetchStatuses(),
    store.fetchPriorities(),
  ])
})
</script>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
  margin-bottom: 16px;
}

.mobile-mode {
  display: flex;
  gap: 4px;
  padding: 4px;
  margin-bottom: 12px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 10px;
}

.mobile-mode button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px;
  border: 0;
  border-radius: 7px;
  background: none;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.mobile-mode button.active {
  background: var(--bg-card);
  color: var(--text-bright);
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.filtered-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  gap: 10px;
}

.filtered-empty-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.filtered-empty-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.reset-filters-btn {
  margin-top: 8px;
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--accent);
  background: var(--accent-bg);
  color: var(--text-bright);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.reset-filters-btn:hover {
  filter: brightness(1.1);
}
</style>
