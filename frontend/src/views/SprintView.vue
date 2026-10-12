<template>
  <div class="sprint-view">
    <div class="view-header">
      <h1 class="page-title">{{ $t('sprint.title') }}</h1>
      <AppButton v-if="auth.canSprint('create')" variant="primary" @click="showForm = true" data-testid="new-sprint-btn">
        + {{ $t('sprint.new_sprint') }}
      </AppButton>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="sprint-loading">
      <AppSpinner :size="32" />
    </div>

    <!-- Error -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchAll()" />

    <!-- Content -->
    <div v-else class="sprint-content">
      <!-- Sprints -->
      <div class="sprints-list">
        <!-- Незакрытые: просроченные сверху, затем по дате окончания -->
        <SprintWidget
          v-for="sprint in openSprints"
          :key="sprint.id"
          :sprint="sprint"
          @complete="completeSprint"
          @edit="editSprint"
          @refresh="handleRefresh"
          @delete="confirmDelete"
          @open-task="openTask"
          @assign-task="handleAssignTask"
        />

        <!-- Нет ни спринтов, ни задач: скорее всего, не выбраны проекты -->
        <div v-if="sprints.length === 0 && backlog.length === 0" class="empty-note" data-testid="sprint-no-data">
          {{ $t('sprint.no_data') }}
        </div>
        <div v-else-if="openSprints.length === 0" class="empty-note">
          {{ $t('sprint.no_open_sprints') }}
        </div>
      </div>

      <!-- Завершённые спринты: отдельный раздел, свёрнут, пока не раскроют -->
      <div v-if="closedSprints.length > 0" class="closed-section" data-testid="closed-sprints">
        <button type="button" class="closed-head" @click="showClosed = !showClosed">
          <ChevronDown :size="16" class="closed-chevron" :class="{ collapsed: !showClosed }" />
          {{ $t('sprint.closed_section') }}
          <span class="closed-count">{{ closedSprints.length }}</span>
        </button>
        <div v-if="showClosed" class="sprints-list">
          <SprintWidget
            v-for="sprint in closedSprints"
            :key="sprint.id"
            :sprint="sprint"
            @reopen="reopenSprint"
            @delete="confirmDelete"
            @open-task="openTask"
          />
        </div>
      </div>

      <!-- Backlog -->
      <SprintBacklog
        :backlog="backlog"
        @open-task="openTask"
        @unassign-task="handleUnassignTask"
      />
    </div>

    <!-- Sprint Form Modal -->
    <SprintForm
      :visible="showForm"
      :sprint="editingSprint"
      :projects="projects"
      @close="closeForm"
      @submit="handleFormSubmit"
    />

    <!-- Delete Confirm -->
    <AppConfirmDialog
      :model-value="showDeleteConfirm"
      :title="$t('sprint.delete_title')"
      @update:model-value="showDeleteConfirm = $event"
      @confirm="handleDelete"
    />

    <!-- Task Modal -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { storeToRefs } from 'pinia'
import { useSprintStore, type Sprint } from '../stores/sprint'
import { useTasksStore } from '../stores/tasks'
import { useLiveRefresh } from '../composables/useLiveRefresh'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { useI18n } from 'vue-i18n'
import { useSwal } from '../composables/useSwal'
import SprintWidget from '../components/sprint/SprintWidget.vue'
import SprintBacklog from '../components/sprint/SprintBacklog.vue'
import SprintForm from '../components/sprint/SprintForm.vue'
import AppButton from '../components/ui/AppButton.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppSpinner from '../components/ui/AppSpinner.vue'
import AppConfirmDialog from '../components/ui/AppConfirmDialog.vue'
import TaskModal from '../components/tasks/TaskModal.vue'

const sprintStore = useSprintStore()
const tasksStore = useTasksStore()
const auth = useAuthStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const { toast } = useSwal()
const { sprints, backlog, loading, error } = storeToRefs(sprintStore)
const { fetchAll, createSprint, updateSprint, deleteSprint, assignTask, unassignTask, refreshSprint } = sprintStore

// Сервер отдаёт спринты уже в порядке экрана; здесь они только делятся на
// незакрытые и завершённые
const openSprints = computed(() => sprints.value.filter(s => s.status !== 'closed'))
const closedSprints = computed(() => sprints.value.filter(s => s.status === 'closed'))
const showClosed = ref(false)

const showForm = ref(false)
const editingSprint = ref<Sprint | null>(null)
const showDeleteConfirm = ref(false)
const deletingSprintId = ref<number | null>(null)
const selectedTaskId = ref<number | null>(null)
// Проекты для формы спринта: те, что пользователь выбрал в настройках, вместе
// с их подпроектами; если ничего не выбрано — все доступные, как и везде
const projects = computed(() => {
  const all = tasksStore.projects
  const selected = settingsStore.settings.selected_projects || []
  if (selected.length === 0) return all.map(p => ({ id: p.id, name: p.name }))

  const parents = new Map<number, number | null>(all.map(p => [p.id, p.parent_id] as [number, number | null]))
  const isSelected = (id: number) => {
    const seen = new Set<number>()
    for (let current: number | null | undefined = id; current != null && !seen.has(current); current = parents.get(current)) {
      if (selected.includes(current)) return true
      seen.add(current)
    }
    return false
  }
  return all.filter(p => isSelected(p.id)).map(p => ({ id: p.id, name: p.name }))
})

function openTask(id: number) {
  selectedTaskId.value = id
}

function editSprint(id: number) {
  editingSprint.value = sprints.value.find(s => s.id === id) || null
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingSprint.value = null
}

// The buttons of actions the user has no right to are hidden; if the server
// still refuses (the rights changed meanwhile), say so instead of failing silently.
async function guarded(action: () => Promise<unknown>) {
  try {
    await action()
  } catch (e: any) {
    const code = e?.response?.data?.error
    const forbidden = code === 'SPRINT_ACTION_FORBIDDEN'
    const message = code === 'SPRINT_KIND_OWNER_ONLY' ? 'sprint.kind_owner_only' : forbidden ? 'sprint.action_forbidden' : 'sprint.action_failed'
    toast(t(message), 'error')
    if (forbidden) auth.fetchMe()
  }
}

async function handleFormSubmit(form: any) {
  const editing = editingSprint.value
  await guarded(() => (editing ? updateSprint(editing.id, form) : createSprint(form)))
  closeForm()
}

async function completeSprint(id: number) {
  await guarded(() => updateSprint(id, { status: 'closed' }))
}

async function reopenSprint(id: number) {
  await guarded(() => updateSprint(id, { status: 'open' }))
}

async function handleDelete() {
  if (deletingSprintId.value) {
    const id = deletingSprintId.value
    await guarded(() => deleteSprint(id))
    showDeleteConfirm.value = false
    deletingSprintId.value = null
  }
}

function confirmDelete(id: number) {
  deletingSprintId.value = id
  showDeleteConfirm.value = true
}

// Перенос задачи: экран меняется сразу, при отказе сервера данные
// перечитываются и показывается ошибка
async function handleAssignTask(payload: { sprintId: number; issueId: number; fromSprintId?: number | null }) {
  if (!auth.canSprint('tasks')) {
    toast(t('sprint.action_forbidden'), 'info')
    return
  }
  await guarded(() => assignTask(payload.sprintId, payload.issueId, payload.fromSprintId ?? null))
}

async function handleUnassignTask(payload: { sprintId: number; issueId: number }) {
  if (!auth.canSprint('tasks')) {
    toast(t('sprint.action_forbidden'), 'info')
    return
  }
  await guarded(() => unassignTask(payload.sprintId, payload.issueId))
}

async function handleRefresh(id: number) {
  await guarded(() => refreshSprint(id))
}

// Changes from a sync or from other users appear without a reload
useLiveRefresh(() => fetchAll({ silent: true }))

onMounted(async () => {
  await fetchAll()
  await Promise.all([tasksStore.fetchProjects(), settingsStore.ensureLoaded()])
})
</script>

<style scoped>
.view-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
}

.sprint-loading {
  display: flex;
  justify-content: center;
  padding: 60px;
}

.sprint-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.sprints-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.closed-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.closed-head {
  display: flex;
  align-items: center;
  gap: 8px;
  align-self: flex-start;
  padding: 4px 0;
  border: none;
  background: none;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  cursor: pointer;
}

.closed-chevron {
  color: var(--text-faint);
  transition: transform 0.2s;
}

.closed-chevron.collapsed {
  transform: rotate(-90deg);
}

.closed-count {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}

.empty-note {
  padding: 32px 16px;
  border: 1px dashed var(--hairline);
  border-radius: 12px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}
</style>
