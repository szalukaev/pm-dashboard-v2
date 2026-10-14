<template>
  <div class="kanban-view">
    <h1 class="page-title">{{ $t('kanban.title') }}</h1>

    <KanbanToolbar
      :mode="mode"
      :project-id="projectId"
      :total="total"
      :projects="projects"
      @change-mode="setMode"
      @change-project="setProject"
    />

    <!-- Загрузка -->
    <div v-if="loading" class="kanban-loading">
      <AppSpinner :size="32" />
    </div>

    <!-- Ошибка -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchBoard()" />

    <!-- Пусто -->
    <div v-else-if="mode === 'statuses' && !projectId" class="kanban-placeholder">
      <AppEmptyState state="empty" />
      <p class="placeholder-hint">{{ $t('kanban.select_project') }}</p>
    </div>

    <!-- Телефон: колонки стопкой -->
    <KanbanMobileBoard
      v-else-if="isMobile"
      :columns="columns"
      @open-task="openTask"
      @move="moveFromMenu"
    />

    <!-- Доска -->
    <div
      v-else
      class="kanban-board"
      @kanban-drop="handleDrop"
      @kanban-column-drop="handleColumnDrop"
      data-testid="kanban-board"
    >
      <KanbanColumn
        v-for="col in columns"
        :key="col.id"
        :column="col"
        @open-task="openTask"
      />
    </div>

    <!-- Карточка задачи -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useKanbanStore } from '../stores/kanban'
import { useTasksStore } from '../stores/tasks'
import { useLiveRefresh } from '../composables/useLiveRefresh'
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'
import { useSwal } from '../composables/useSwal'
import KanbanToolbar from '../components/kanban/KanbanToolbar.vue'
import KanbanColumn from '../components/kanban/KanbanColumn.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppSpinner from '../components/ui/AppSpinner.vue'
import TaskModal from '../components/tasks/TaskModal.vue'
import KanbanMobileBoard from '../components/kanban/KanbanMobileBoard.vue'
import { useMobileTasks } from '../composables/useMobileTasks'

const kanbanStore = useKanbanStore()
const auth = useAuthStore()
const { t } = useI18n()
const { toast } = useSwal()
const tasksStore = useTasksStore()
const { columns, mode, total, loading, error, projectId } = storeToRefs(kanbanStore)
const { fetchBoard, moveCard, moveColumn, setMode, setProject } = kanbanStore

const projects = ref<{ id: number; name: string }[]>([])
const selectedTaskId = ref<number | null>(null)
const { isMobile } = useMobileTasks()

function openTask(id: number) {
  selectedTaskId.value = id
}

function handleDrop(e: Event) {
  const customEvent = e as CustomEvent
  const { issueId, columnId } = customEvent.detail
  if (!auth.canWrite) {
    toast(t('kanban.read_only'), 'info')
    return
  }
  if (issueId && columnId) {
    moveCard(issueId, columnId)
  }
}

// Перенос из меню карточки на телефоне — те же проверки, что у перетаскивания
function moveFromMenu({ issueId, columnId }: { issueId: number; columnId: string }) {
  if (!auth.canWrite) {
    toast(t('kanban.read_only'), 'info')
    return
  }
  moveCard(issueId, columnId)
}

function handleColumnDrop(e: Event) {
  const { fromId, toId } = (e as CustomEvent).detail
  if (fromId && toId) {
    moveColumn(fromId, toId)
  }
}

// Изменения от синхронизации и других пользователей появляются без перезагрузки
useLiveRefresh(() => fetchBoard({ silent: true }))

onMounted(async () => {
  await kanbanStore.restoreState()
  // Приоритеты — ради их цветов на карточках
  if (!tasksStore.priorities.length) tasksStore.fetchPriorities()
  await Promise.all([
    fetchBoard(),
    tasksStore.fetchProjects().then(() => {
      projects.value = tasksStore.projects
    }),
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

.kanban-board {
  display: flex;
  gap: 16px;
  overflow-x: auto;
  padding-bottom: 16px;
}

.kanban-loading {
  display: flex;
  justify-content: center;
  padding: 60px;
}

.kanban-placeholder {
  text-align: center;
}

.placeholder-hint {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: -20px;
}
</style>
