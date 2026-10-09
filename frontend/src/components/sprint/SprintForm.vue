<template>
  <AppModal
    :model-value="visible"
    :title="sprint ? $t('sprint.edit_sprint') : $t('sprint.new_sprint')"
    width="520px"
    @update:model-value="!$event && $emit('close')"
  >
    <form class="sprint-form" @submit.prevent="submit">
      <div class="form-field">
        <label>{{ $t('sprint.form.name') }} *</label>
        <input v-model="form.name" type="text" required />
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.project') }}</label>
        <select v-model="form.project_name" @change="onProjectChange">
          <option value="">—</option>
          <option v-for="p in projects" :key="p.name" :value="p.name">{{ p.name }}</option>
        </select>
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.category') }}</label>
        <!-- The categories of the chosen project -->
        <select v-model="form.category_name" :disabled="!projectId">
          <option value="">{{ projectId ? '—' : $t('sprint.form.category_pick_project') }}</option>
          <option v-for="c in categoryOptions" :key="c" :value="c">{{ c }}</option>
        </select>
      </div>

      <div class="form-field checkbox-field">
        <label>
          <input v-model="form.auto_fill_category" type="checkbox" />
          {{ $t('sprint.form.auto_fill') }}
        </label>
      </div>

      <div class="form-row">
        <div class="form-field">
          <label>{{ $t('sprint.form.start_date') }} *</label>
          <input v-model="form.start_date" type="date" required />
        </div>
        <div class="form-field">
          <label>{{ $t('sprint.form.due_date') }}</label>
          <input v-model="form.due_date" type="date" />
        </div>
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.description') }}</label>
        <textarea v-model="form.description" rows="3"></textarea>
      </div>

      <div class="form-field" v-if="sprint">
        <label>{{ $t('sprint.form.status') }}</label>
        <!-- Changing the status closes or reopens the sprint: a separate right -->
        <select v-model="form.status" :disabled="!auth.canSprint('close')">
          <option value="open">open</option>
          <option value="active">active</option>
          <option value="closed">closed</option>
        </select>
      </div>
    </form>

    <template #footer>
      <AppButton variant="ghost" @click="$emit('close')">
        {{ $t('common.cancel') }}
      </AppButton>
      <AppButton variant="primary" @click="submit" :disabled="!canSubmit">
        {{ sprint ? $t('common.save') : $t('common.create') }}
      </AppButton>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import AppModal from '../ui/AppModal.vue'
import AppButton from '../ui/AppButton.vue'
import type { Sprint } from '../../stores/sprint'
import { useAuthStore } from '../../stores/auth'
import { useTasksStore } from '../../stores/tasks'

const auth = useAuthStore()
const tasksStore = useTasksStore()

const props = defineProps<{
  visible: boolean
  sprint: Sprint | null
  projects: { id: number; name: string }[]
}>()

const emit = defineEmits(['close', 'submit'])

const form = reactive({
  name: '',
  project_name: '',
  category_name: '',
  auto_fill_category: false,
  start_date: '',
  due_date: '',
  description: '',
  status: 'open',
})

const projectId = computed(() => props.projects.find(p => p.name === form.project_name)?.id)

// A category saved earlier stays selectable even if the project no longer has it
const categoryOptions = computed(() => {
  const list = (projectId.value && tasksStore.projectCategories[projectId.value]) || []
  return form.category_name && !list.includes(form.category_name) ? [form.category_name, ...list] : list
})

function loadCategories() {
  if (projectId.value) tasksStore.fetchProjectCategories(projectId.value)
}

function onProjectChange() {
  form.category_name = ''
  loadCategories()
}

watch(projectId, loadCategories)

// Filled every time the window opens: a new sprint starts from a blank form
watch([() => props.visible, () => props.sprint], ([visible, s]) => {
  if (!visible) return
  if (s) {
    form.name = s.name
    form.project_name = s.project_name || ''
    form.category_name = s.category_name || ''
    form.auto_fill_category = s.auto_fill_category
    form.start_date = s.start_date || ''
    form.due_date = s.due_date || ''
    form.description = s.description || ''
    form.status = s.status
  } else {
    form.name = ''
    form.project_name = ''
    form.category_name = ''
    form.auto_fill_category = false
    form.start_date = ''
    form.due_date = ''
    form.description = ''
    form.status = 'open'
  }
  loadCategories()
}, { immediate: true })

// The name and the start date are required
const canSubmit = computed(() => form.name.trim() !== '' && form.start_date !== '')

function submit() {
  if (!canSubmit.value) return
  // A field left blank is "not set", not an empty string
  emit('submit', {
    ...form,
    name: form.name.trim(),
    project_name: form.project_name || null,
    category_name: form.category_name || null,
    due_date: form.due_date || null,
    description: form.description.trim() || null,
  })
}
</script>

<style scoped>
.sprint-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 6px;
}

.form-field input[type="text"],
.form-field input[type="date"],
.form-field select,
.form-field textarea {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-field input:focus,
.form-field select:focus,
.form-field textarea:focus {
  border-color: var(--accent);
}

.form-field select:disabled {
  color: var(--text-muted);
  cursor: not-allowed;
}

.form-field textarea {
  resize: vertical;
  min-height: 60px;
}

.checkbox-field label {
  display: flex;
  align-items: center;
  gap: 8px;
  text-transform: none;
  font-size: 13px;
  cursor: pointer;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
</style>
