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
        <select v-model="form.project_name">
          <option value="">—</option>
          <option v-for="p in projects" :key="p.name" :value="p.name">{{ p.name }}</option>
        </select>
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.category') }}</label>
        <input v-model="form.category_name" type="text" />
      </div>

      <div class="form-field checkbox-field">
        <label>
          <input v-model="form.auto_fill_category" type="checkbox" />
          {{ $t('sprint.form.auto_fill') }}
        </label>
      </div>

      <div class="form-row">
        <div class="form-field">
          <label>{{ $t('sprint.form.start_date') }}</label>
          <input v-model="form.start_date" type="date" />
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
        <select v-model="form.status">
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
      <AppButton variant="primary" @click="submit" :disabled="!form.name">
        {{ sprint ? $t('common.save') : $t('common.create') }}
      </AppButton>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import AppModal from '../ui/AppModal.vue'
import AppButton from '../ui/AppButton.vue'
import type { Sprint } from '../../stores/sprint'

const props = defineProps<{
  visible: boolean
  sprint: Sprint | null
  projects: { name: string }[]
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

watch(() => props.sprint, (s) => {
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
}, { immediate: true })

function submit() {
  emit('submit', { ...form })
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
