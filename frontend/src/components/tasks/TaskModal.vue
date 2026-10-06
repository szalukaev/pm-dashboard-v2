<template>
  <AppModal
    :model-value="!!taskId"
    :title="task ? `#${task.external_id} ${task.subject}` : ''"
    width="720px"
    @update:model-value="!$event && $emit('close')"
  >
    <template #title>
      <template v-if="task">
        <a :href="taskLink(task.external_id)" target="_blank" class="modal-task-link" @click.stop>#{{ task.external_id }}</a>
        {{ task.subject }}
      </template>
    </template>
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
            <option value="">Без ответственного</option>
            <option v-for="m in memberNames" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>
      </div>

      <div class="task-meta">
        <div class="meta-item">
          <span class="meta-label">Проект:</span>
          <span>{{ task.project_name }}</span>
        </div>
        <div class="meta-item">
          <span class="meta-label">Категория:</span>
          <select v-model="editFields.category_name" class="field-select" @change="saveField('category_name')">
            <option value="">—</option>
            <option v-for="c in taskCategories" :key="c" :value="c">{{ c }}</option>
          </select>
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
      <!-- Comments section -->
      <div class="comments-section">
        <h4 class="comments-title">Комментарии</h4>
        <div class="comments-list">
          <div v-if="comments.length === 0" class="no-comments">Нет комментариев</div>
          <div v-for="c in comments" :key="c.id" class="comment-item">
            <div class="comment-header">
              <span class="comment-author">{{ c.author }}</span>
              <span class="comment-date">{{ formatDate(c.created_on) }}</span>
            </div>
            <div class="comment-text">{{ c.text }}</div>
          </div>
        </div>
        <div class="comment-form">
          <textarea
            v-model="newComment"
            class="comment-input"
            placeholder="Добавить комментарий..."
            rows="3"
          ></textarea>
          <AppButton
            variant="primary"
            size="sm"
            :disabled="!newComment.trim()"
            :loading="sendingComment"
            @click="submitComment"
          >
            Сохранить
          </AppButton>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, reactive, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import AppModal from '../ui/AppModal.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import AppButton from '../ui/AppButton.vue'
import { useTasksStore, type Task } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'
import axios from 'axios'
import { useSwal } from '../../composables/useSwal'

const props = defineProps<{ taskId: number | null }>()
defineEmits(['close'])

const store = useTasksStore()
const settingsStore = useSettingsStore()
const { settings } = storeToRefs(settingsStore)
const { tasks } = storeToRefs(store)
const { statuses } = storeToRefs(store)
const { showChange, error: swalError, toast } = useSwal()

const task = ref<Task | null>(null)
const loading = ref(false)
const statusMessage = ref<{ text: string; type: string } | null>(null)

const editFields = reactive({
  status_name: '',
  priority_name: '',
  assigned_to_name: '',
  category_name: '',
  start_date: '',
  due_date: '',
})

const priorities = computed(() => store.priorities.map((p: any) => p.name))
const members = ref<{ id: number; name: string }[]>([])
const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  const all = members.value
  if (selectedTeam.length === 0) return all.map(m => m.name).sort()
  return all.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

const comments = ref<any[]>([])
const newComment = ref('')
const sendingComment = ref(false)

function taskLink(id: number): string {
  const base = settings.value.data_source_url || settings.value.redmine_url || ''
  if (base) {
    return base.replace(/\/$/, '') + '/issues/' + id
  }
  return '#'
}

function formatDate(d?: string): string {
  if (!d) return ''
  return new Date(d).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

async function loadComments(id: number) {
  try {
    const { data } = await axios.get(`/api/tasks/${id}/comments`)
    comments.value = data.comments || []
  } catch {
    comments.value = []
  }
}

async function submitComment() {
  if (!newComment.value.trim() || !task.value) return
  sendingComment.value = true
  try {
    await axios.post(`/api/tasks/${task.value.external_id}/comments`, { text: newComment.value.trim() })
    newComment.value = ''
    await loadComments(task.value.external_id)
    toast('Комментарий добавлен', 'success')
  } catch {
    toast('Ошибка добавления комментария', 'error')
  } finally {
    sendingComment.value = false
  }
}

const taskCategories = computed(() => {
  if (!task.value) return []
  const cats = new Set<string>()
  for (const t of tasks.value) {
    if (t.project_id === task.value.project_id && t.category_name) {
      cats.add(t.category_name)
    }
  }
  return [...cats].sort()
})

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
    editFields.category_name = task.value.category_name || ''
    loadComments(task.value.external_id)
    editFields.start_date = (task.value.start_date || '').slice(0, 10)
    editFields.due_date = (task.value.due_date || '').slice(0, 10)
  }
  loading.value = false
}, { immediate: true })

// Load team members for assignee dropdown
onMounted(async () => {
  try {
    const { data } = await axios.get('/api/tasks/members')
    members.value = data.members || []
  } catch {}
})

async function saveField(field: string) {
  if (!task.value) return
  const oldValue = (task.value as any)[field] || '—'
  const newValue = (editFields as any)[field] || '—'

  try {
    const result = await store.updateTask(task.value.external_id, { [field]: newValue === '—' ? null : newValue }, true)
    // Toast notification: "Старое значение → Новое значение"
    const fieldLabels: Record<string, string> = {
      status_name: 'Статус',
      priority_name: 'Приоритет',
      assigned_to_name: 'Исполнитель',
      category_name: 'Категория',
      start_date: 'Дата начала',
      due_date: 'Дедлайн',
      estimated_hours: 'Оценка',
    }
    showChange(String(oldValue), String(newValue), `#${task.value.external_id} — ${fieldLabels[field] || field}`)
    // Check Redmine sync result
    if (result && result.redmine_ok === false) {
      toast('Ошибка синхронизации с Redmine', 'error')
    }
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

.modal-task-link {
  color: var(--accent);
  text-decoration: none;
  margin-right: 4px;
}

.modal-task-link:hover {
  text-decoration: underline;
}

.comments-section {
  margin-top: 20px;
  border-top: 1px solid var(--hairline);
  padding-top: 16px;
}

.comments-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
}

.comments-list {
  max-height: 250px;
  overflow-y: auto;
  margin-bottom: 12px;
}

.no-comments {
  font-size: 12px;
  color: var(--text-faintest);
  padding: 8px 0;
}

.comment-item {
  padding: 10px 0;
  border-bottom: 1px solid var(--border-light);
}

.comment-item:last-child {
  border-bottom: none;
}

.comment-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.comment-author {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-bright);
}

.comment-date {
  font-size: 11px;
  color: var(--text-faintest);
}

.comment-text {
  font-size: 12px;
  color: var(--text-dim);
  line-height: 1.5;
  white-space: pre-wrap;
}

.comment-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.comment-input {
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  padding: 10px;
  font-size: 12px;
  resize: vertical;
  min-height: 60px;
  outline: none;
  font-family: inherit;
}

.comment-input:focus {
  border-color: var(--accent);
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
