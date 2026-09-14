<template>
  <div class="sprint-view">
    <div class="view-header">
      <h1 class="page-title">{{ $t('sprint.title') }}</h1>
      <AppButton variant="primary" @click="showForm = true" data-testid="new-sprint-btn">
        + {{ $t('sprint.new_sprint') }}
      </AppButton>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="sprint-loading">
      <AppSpinner :size="32" />
    </div>

    <!-- Error -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchAll" />

    <!-- Content -->
    <div v-else class="sprint-content">
      <!-- Sprints -->
      <div class="sprints-list">
        <SprintWidget
          v-for="sprint in sprints"
          :key="sprint.id"
          :sprint="sprint"
          @complete="completeSprint"
          @edit="editSprint"
          @refresh="refreshSprint"
          @delete="confirmDelete"
          @open-task="openTask"
          @assign-task="handleAssignTask"
        />

        <div v-if="sprints.length === 0" class="empty-sprints">
          <AppEmptyState state="empty" />
        </div>
      </div>

      <!-- Backlog -->
      <SprintBacklog
        :backlog="backlog"
        @open-task="openTask"
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
      :title="$t('common.confirm_delete_title', { entity: 'спринт' })"
      @update:model-value="showDeleteConfirm = $event"
      @confirm="handleDelete"
    />

    <!-- Task Modal -->
    <TaskModal :task-id="selectedTaskId" @close="selectedTaskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useSprintStore, type Sprint } from '../stores/sprint'
import { useTasksStore } from '../stores/tasks'
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
const { sprints, backlog, loading, error } = storeToRefs(sprintStore)
const { fetchAll, createSprint, updateSprint, deleteSprint, assignTask, refreshSprint } = sprintStore

const showForm = ref(false)
const editingSprint = ref<Sprint | null>(null)
const showDeleteConfirm = ref(false)
const deletingSprintId = ref<number | null>(null)
const selectedTaskId = ref<number | null>(null)
const projects = ref<{ name: string }[]>([])

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

async function handleFormSubmit(form: any) {
  if (editingSprint.value) {
    await updateSprint(editingSprint.value.id, form)
  } else {
    await createSprint(form)
  }
  closeForm()
}

async function completeSprint(id: number) {
  await updateSprint(id, { status: 'closed' })
}

async function handleDelete() {
  if (deletingSprintId.value) {
    await deleteSprint(deletingSprintId.value)
    showDeleteConfirm.value = false
    deletingSprintId.value = null
  }
}

function confirmDelete(id: number) {
  deletingSprintId.value = id
  showDeleteConfirm.value = true
}

async function handleAssignTask(payload: { sprintId: number; issueId: number }) {
  await assignTask(payload.sprintId, payload.issueId)
}

onMounted(async () => {
  await fetchAll()
  await tasksStore.fetchProjects()
  projects.value = tasksStore.projects.map(p => ({ name: p.name }))
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

.empty-sprints {
  margin-bottom: 24px;
}
</style>
