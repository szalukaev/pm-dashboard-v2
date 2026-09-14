<template>
  <AppModal
    :model-value="!!taskId"
    :title="task ? `#${task.external_id} ${task.subject}` : ''"
    width="720px"
    @update:model-value="!$event && $emit('close')"
  >
    <template v-if="task">
      <!-- Header info -->
      <div class="task-header">
        <div class="header-field">
          <span class="field-label">{{ $t('tasks.table.status') }}</span>
          <select v-model="editFields.status_name" class="field-select" @change="saveField('status_name')">
            <option v-for="s in statuses" :key="s.id" :value="s.name">{{ s.name }}</option>
          </select>
        </div>
        <div class="header-field">
          <span class="field-label">{{ $t('tasks.table.priority') }}</span>
          <select v-model="editFields.priority_name" class="field-select" @change="saveField('priority_name')">
            <option v-for="p in priorities" :key="p" :value="p">{{ p }}</option>
          </select>
        </div>
        <div class="header-field">
          <span class="field-label">{{ $t('tasks.table.assignee') }}</span>
          <select v-model="editFields.assigned_to_name" class="field-select" @change="saveField('assigned_to_name')">
            <option value="">—</option>
            <option v-for="m in members" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>
      </div>

      <div class="task-meta">
        <div class="meta-item">
          <span class="meta-label">Проект:</span>
          <span>{{ task.project_name }}</span>
        </div>
        <div class="meta-item" v-if="task.category_name">
          <span class="meta-label">Категория:</span>
          <span>{{ task.category_name }}</span>
        </div>
        <div class="meta-item" v-if="task.tracker_name">
          <span class="meta-label">Тип:</span>
          <span>{{ task.tracker_name }}</span>
        </div>
        <div class="meta-item">
          <span class="meta-label">Дата начала:</span>
          <input type="date" v-model="editFields.start_date" class="date-input" @change="saveField('start_date')" />
        </div>
        <div class="meta-item">
          <span class="meta-label">Дедлайн:</span>
          <input type="date" v-model="editFields.due_date" class="date-input" @change="saveField('due_date')" />
        </div>
        <div class="meta-item">
          <span class="meta-label">Оценка:</span>
          <span>{{ task.estimated_hours ? task.estimated_hours + ' ч.' : '—' }}</span>
        </div>
        <div class="meta-item">
          <span class="meta-label">Факт:</span>
          <span>{{ formatHours(task.spent_hours) }}</span>
        </div>
      </div>

      <!-- Description -->
      <div class="task-description" v-if="task.description">
        <h4 class="section-title">Описание</h4>
        <div class="description-content" v-html="task.description"></div>
      </div>

      <!-- Status change alert -->
      <div v-if="statusMessage" class="status-alert" :class="statusMessage.type">
        {{ statusMessage.text }}
      </div>
    </template>

    <template v-else-if="loading">
      <div class="loading-state">
        <AppSpinner :size="32" />
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, watch, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import AppModal from '../ui/AppModal.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import { useTasksStore, type Task } from '../../stores/tasks'
import { useSwal } from '../../composables/useSwal'

const props = defineProps<{ taskId: number | null }>()
defineEmits(['close'])

const store = useTasksStore()
const { statuses } = storeToRefs(store)
const { showChange, error: swalError } = useSwal()

const task = ref<Task | null>(null)
const loading = ref(false)
const statusMessage = ref<{ text: string; type: string } | null>(null)

const editFields = reactive({
  status_name: '',
  priority_name: '',
  assigned_to_name: '',
  start_date: '',
  due_date: '',
})

const priorities = ['Low', 'Normal', 'High', 'Urgent', 'Immediate']
const members = ref<string[]>([])

watch(() => props.taskId, async (id) => {
  if (!id) {
    task.value = null
    return
  }
  loading.value = true
  task.value = await store.fetchTask(id)
  if (task.value) {
    editFields.status_name = task.value.status_name
    editFields.priority_name = task.value.priority_name
    editFields.assigned_to_name = task.value.assigned_to_name || ''
    editFields.start_date = task.value.start_date || ''
    editFields.due_date = task.value.due_date || ''
  }
  loading.value = false
})

async function saveField(field: string) {
  if (!task.value) return
  const oldValue = (task.value as any)[field] || '—'
  const newValue = (editFields as any)[field] || '—'

  try {
    await store.updateTask(task.value.external_id, { [field]: newValue === '—' ? null : newValue })
    // SweetAlert2 notification per ТЗ: "Старое значение → Новое значение"
    showChange(String(oldValue), String(newValue))
  } catch {
    swalError('Ошибка сохранения')
  }
}

function formatHours(h: number): string {
  if (!h) return '—'
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  return `${hours}:${mins.toString().padStart(2, '0')} ч.`
}
</script>

<style scoped>
.task-header {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-light);
}

.header-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 140px;
}

.field-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.field-select {
  padding: 6px 10px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.field-select:focus {
  border-color: var(--accent);
}

.task-meta {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-light);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-dim);
}

.meta-label {
  color: var(--text-muted);
  font-size: 12px;
}

.date-input {
  padding: 4px 8px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 6px;
  color: var(--text);
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
}

.description-content {
  font-size: 13px;
  color: var(--text-dim);
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
}

.description-content img {
  max-width: 100%;
  border-radius: 8px;
}

.status-alert {
  margin-top: 16px;
  padding: 8px 12px;
  font-size: 12px;
  border-radius: 8px;
}

.status-alert.success {
  background: var(--success)22;
  color: var(--success);
}

.status-alert.error {
  background: var(--danger)22;
  color: var(--danger);
}

.loading-state {
  display: flex;
  justify-content: center;
  padding: 40px;
}
</style>
