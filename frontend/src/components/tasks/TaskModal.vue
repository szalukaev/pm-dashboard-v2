<template>
  <AppModal
    :model-value="!!taskId"
    :title="task ? `#${task.external_id} ${task.subject}` : ''"
    width="70vw"
    @update:model-value="!$event && $emit('close')"
  >
    <template #title>
      <template v-if="task">
        <a :href="taskLink(task.external_id)" target="_blank" class="modal-task-link" @click.stop>#{{ task.external_id }}</a>
        {{ task.subject }}
      </template>
    </template>

    <div v-if="loading && !task" class="loading-state">
      <AppSpinner :size="32" />
    </div>

    <div v-else-if="task" class="task-card">
      <!-- Parameters -->
      <section class="params">
        <div class="params-col">
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.project') }}</span>
            <span class="param-value">{{ task.project_name || '—' }}</span>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.type') }}</span>
            <span class="param-value">{{ task.tracker_name || '—' }}</span>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.estimate') }}</span>
            <div class="param-control">
              <input
                v-model="editFields.estimated_hours"
                type="text"
                inputmode="decimal"
                :disabled="!auth.canWrite" class="field-input num-input"
                placeholder="—"
                @change="saveField('estimated_hours')"
                @keyup.enter="($event.target as HTMLInputElement).blur()"
              />
              <span class="unit">{{ $t('tasks.units.hours') }}</span>
            </div>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.fact') }}</span>
            <span class="param-value num">{{ formatHours(task.spent_hours) }}</span>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.bug_fix') }}</span>
            <span class="param-value num">
              {{ formatHours(task.bug_fix_hours) }}
              <span v-if="bugFixPct" class="param-hint">{{ bugFixPct }}</span>
            </span>
          </div>
        </div>

        <div class="params-col">
          <div class="param">
            <span class="param-label">{{ $t('tasks.table.status') }}</span>
            <select v-model="editFields.status_name" :disabled="!auth.canWrite" class="field-input" @change="saveField('status_name')">
              <option v-for="s in statuses" :key="s.id" :value="s.name">{{ s.name }}</option>
            </select>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.table.priority') }}</span>
            <select v-model="editFields.priority_name" :disabled="!auth.canWrite" class="field-input" @change="saveField('priority_name')">
              <option v-for="p in priorityNames" :key="p" :value="p">{{ p }}</option>
            </select>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.table.assignee') }}</span>
            <select v-model="editFields.assigned_to_name" :disabled="!auth.canWrite" class="field-input" @change="saveField('assigned_to_name')">
              <option value="">{{ $t('tasks.modal.no_assignee') }}</option>
              <option v-for="m in memberNames" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.category') }}</span>
            <select v-model="editFields.category_name" :disabled="!auth.canWrite" class="field-input" @change="saveField('category_name')">
              <option value="">—</option>
              <option v-for="c in taskCategories" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.start_date') }}</span>
            <span class="param-value">{{ formatDay(task.start_date) }}</span>
          </div>
          <div class="param">
            <span class="param-label">{{ $t('tasks.modal.deadline') }}</span>
            <input type="date" v-model="editFields.due_date" :disabled="!auth.canWrite" class="field-input" @change="saveField('due_date')" />
          </div>
        </div>
      </section>

      <!-- Description -->
      <section class="section">
        <h4 class="section-title">{{ $t('tasks.modal.description') }}</h4>
        <div v-if="detailsLoading" class="section-loading"><AppSpinner :size="20" /></div>
        <div v-else-if="descriptionHtml" class="markup" v-html="descriptionHtml"></div>
        <div v-else class="empty">{{ $t('tasks.modal.description_empty') }}</div>
      </section>

      <!-- Files -->
      <section class="section">
        <h4 class="section-title">
          {{ $t('tasks.modal.files') }}
          <span v-if="attachments.length" class="section-count">{{ attachments.length }}</span>
        </h4>
        <div v-if="detailsLoading" class="section-loading"><AppSpinner :size="20" /></div>
        <div v-else-if="attachments.length" class="files">
          <a
            v-for="a in attachments"
            :key="a.id"
            :href="attachmentUrl(a.id)"
            target="_blank"
            rel="noopener"
            class="file"
            :title="a.description || a.filename"
          >
            <img v-if="isImage(a)" :src="attachmentUrl(a.id)" :alt="a.filename" class="file-thumb" loading="lazy" />
            <span v-else class="file-icon"><FileText :size="18" /></span>
            <span class="file-info">
              <span class="file-name">{{ a.filename }}</span>
              <span class="file-meta">{{ formatSize(a.filesize) }} · {{ a.author }} · {{ formatDate(a.created_on) }}</span>
            </span>
          </a>
        </div>
        <div v-else class="empty">{{ $t('tasks.modal.files_empty') }}</div>
      </section>

      <!-- Comments -->
      <section class="section">
        <h4 class="section-title">
          {{ $t('tasks.modal.comments') }}
          <span v-if="comments.length" class="section-count">{{ comments.length }}</span>
        </h4>
        <div v-if="detailsLoading" class="section-loading"><AppSpinner :size="20" /></div>
        <div v-else-if="comments.length" class="comments">
          <div v-for="c in comments" :key="c.id" class="comment">
            <span class="avatar">{{ initials(c.author) }}</span>
            <div class="comment-body">
              <div class="comment-header">
                <span class="comment-author">{{ c.author }}</span>
                <span class="comment-date">{{ formatDate(c.created_on) }}</span>
              </div>
              <div class="markup comment-text" v-html="c.html"></div>
            </div>
          </div>
        </div>
        <div v-else class="empty">{{ $t('tasks.modal.comments_empty') }}</div>
        <div v-if="detailsError" class="empty error-text">{{ detailsError }}</div>
      </section>

      <!-- New comment -->
      <section v-if="auth.canWrite" class="section new-comment">
        <h4 class="section-title">{{ $t('tasks.modal.new_comment') }}</h4>
        <textarea
          v-model="newComment"
          class="comment-input"
          :placeholder="$t('tasks.modal.comment_placeholder')"
          rows="3"
          @keydown.ctrl.enter="submitComment"
          @keydown.meta.enter="submitComment"
        ></textarea>
        <div class="comment-actions">
          <span class="comment-hint">{{ $t('tasks.modal.comment_hint') }}</span>
          <AppButton
            variant="primary"
            size="sm"
            :disabled="!newComment.trim()"
            :loading="sendingComment"
            @click="submitComment"
          >
            {{ $t('common.save') }}
          </AppButton>
        </div>
      </section>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { FileText } from 'lucide-vue-next'
import AppModal from '../ui/AppModal.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import AppButton from '../ui/AppButton.vue'
import { useTasksStore, type Task } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'
import { useSwal } from '../../composables/useSwal'
import { renderRedmineMarkup } from '../../utils/redmineMarkup'

interface Attachment {
  id: number
  filename: string
  filesize: number
  content_type: string
  description: string
  author: string
  created_on: string
}

interface Comment {
  id: number
  author: string
  text: string
  created_on: string
}

const props = defineProps<{ taskId: number | null }>()
defineEmits(['close'])

const store = useTasksStore()
const settingsStore = useSettingsStore()
const { settings } = storeToRefs(settingsStore)
const { statuses, priorities, members, projectCategories } = storeToRefs(store)
const { showChange, error: swalError, toast } = useSwal()
const auth = useAuthStore()
// "t" is taken by a local variable in saveField
const i18n = useI18n()

const task = ref<Task | null>(null)
const loading = ref(false)

const editFields = reactive({
  status_name: '',
  priority_name: '',
  assigned_to_name: '',
  category_name: '',
  due_date: '',
  estimated_hours: '' as string | number,
})

// Live part of the card, read from Redmine
const description = ref('')
const attachments = ref<Attachment[]>([])
const rawComments = ref<Comment[]>([])
const detailsLoading = ref(false)
const detailsError = ref('')

const newComment = ref('')
const sendingComment = ref(false)

const priorityNames = computed(() => priorities.value.map(p => p.name))

const memberNames = computed(() => {
  const selectedTeam = settings.value.selected_team || []
  const all = members.value
  if (selectedTeam.length === 0) return all.map(m => m.name).sort()
  return all.filter(m => selectedTeam.includes(m.id)).map(m => m.name).sort()
})

// All categories of the task's project; the current one stays selectable.
const taskCategories = computed(() => {
  if (!task.value) return []
  const cats = new Set<string>(projectCategories.value[task.value.project_id] || [])
  if (task.value.category_name) cats.add(task.value.category_name)
  return [...cats].sort()
})

const bugFixPct = computed(() => {
  const t = task.value
  if (!t || !t.spent_hours || !t.bug_fix_hours) return ''
  return i18n.t('tasks.modal.of_fact', { pct: (t.bug_fix_hours / t.spent_hours * 100).toFixed(1) })
})

const redmineUrl = computed(() => (settings.value.data_source_url || settings.value.redmine_url || '').replace(/\/$/, ''))

function markupOptions() {
  return { attachments: attachments.value, attachmentUrl, redmineUrl: redmineUrl.value }
}

const descriptionHtml = computed(() => renderRedmineMarkup(description.value, markupOptions()))

const comments = computed(() =>
  rawComments.value.map(c => ({ ...c, html: renderRedmineMarkup(c.text, markupOptions()) }))
)

function attachmentUrl(id: number): string {
  return `/api/tasks/${task.value?.external_id}/attachments/${id}`
}

function isImage(a: Attachment): boolean {
  return /^image\/(png|jpe?g|gif|webp|bmp)$/i.test(a.content_type)
}

function taskLink(id: number): string {
  return redmineUrl.value ? `${redmineUrl.value}/issues/${id}` : '#'
}

async function loadDetails(id: number) {
  detailsLoading.value = true
  detailsError.value = ''
  try {
    const { data } = await axios.get(`/api/tasks/${id}/details`)
    if (task.value?.external_id !== id) return
    description.value = data.description || ''
    attachments.value = data.attachments || []
    rawComments.value = data.comments || []
  } catch {
    if (task.value?.external_id !== id) return
    // Fall back to the cached description
    description.value = task.value?.description || ''
    detailsError.value = i18n.t('tasks.messages.details_error')
  } finally {
    if (task.value?.external_id === id) detailsLoading.value = false
  }
}

async function submitComment() {
  if (!newComment.value.trim() || !task.value || sendingComment.value) return
  sendingComment.value = true
  const id = task.value.external_id
  try {
    await axios.post(`/api/tasks/${id}/comments`, { text: newComment.value.trim() })
    newComment.value = ''
    const { data } = await axios.get(`/api/tasks/${id}/comments`)
    rawComments.value = data.comments || []
    toast(i18n.t('tasks.messages.comment_added'), 'success')
  } catch {
    toast(i18n.t('tasks.messages.comment_error'), 'error')
  } finally {
    sendingComment.value = false
  }
}

watch(() => props.taskId, async (id) => {
  task.value = null
  description.value = ''
  attachments.value = []
  rawComments.value = []
  newComment.value = ''
  if (!id) return

  loading.value = true
  // Reference data may be missing when the card is opened outside the Tasks tab
  if (!statuses.value.length) store.fetchStatuses()
  if (!priorities.value.length) store.fetchPriorities()
  store.fetchMembers()
  if (!redmineUrl.value) settingsStore.fetchSettings()

  const loaded = await store.fetchTask(id)
  if (props.taskId !== id) return
  task.value = loaded
  loading.value = false
  if (!loaded) return

  editFields.status_name = loaded.status_name
  editFields.priority_name = loaded.priority_name
  editFields.assigned_to_name = loaded.assigned_to_name || ''
  editFields.category_name = loaded.category_name || ''
  editFields.due_date = (loaded.due_date || '').slice(0, 10)
  editFields.estimated_hours = loaded.estimated_hours ?? ''
  store.fetchProjectCategories(loaded.project_id)
  loadDetails(id)
}, { immediate: true })

const fieldLabels: Record<string, string> = {
  status_name: 'tasks.fields.status',
  priority_name: 'tasks.fields.priority',
  assigned_to_name: 'tasks.fields.assignee',
  category_name: 'tasks.fields.category',
  due_date: 'tasks.fields.deadline',
  estimated_hours: 'tasks.fields.estimate',
}

async function saveField(field: keyof typeof editFields) {
  if (!task.value) return
  const t = task.value as any
  const raw = editFields[field]
  let value = raw === '' || raw === null ? null : String(raw)
  const old = t[field] ?? null
  if (field === 'estimated_hours' && value !== null) {
    // Plain text field: accept both "1,5" and "1.5"
    value = value.trim().replace(',', '.') || null
    if (value !== null && !(Number(value) >= 0)) {
      editFields[field] = old ?? ''
      toast(i18n.t('tasks.messages.invalid_number'), 'error')
      return
    }
  }
  if (String(old ?? '') === String(value ?? '')) return

  try {
    const result = await store.updateTask(task.value.external_id, { [field]: value }, true)
    if (result && result.redmine_ok === false) {
      toast(i18n.t('tasks.messages.redmine_sync_error') + ': ' + (result.redmine_error || ''), 'error')
      editFields[field] = old ?? ''
      return
    }
    t[field] = field === 'estimated_hours' && value !== null ? Number(value) : value
    showChange(String(old ?? '—'), String(value ?? '—'), `#${task.value.external_id} — ${i18n.t(fieldLabels[field])}`)
  } catch {
    editFields[field] = old ?? ''
    swalError(i18n.t('tasks.messages.save_error'))
  }
}

function formatHours(h: number): string {
  if (!h) return '—'
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  return `${hours}:${mins.toString().padStart(2, '0')} ${i18n.t('tasks.units.hours')}`
}

// Dates follow the interface language
const dateLocale = computed(() => (i18n.locale.value === 'en' ? 'en-GB' : 'ru-RU'))

function formatDay(d?: string | null): string {
  if (!d) return '—'
  return new Date(d.slice(0, 10) + 'T00:00:00').toLocaleDateString(dateLocale.value)
}

function formatDate(d?: string): string {
  if (!d) return ''
  return new Date(d).toLocaleString(dateLocale.value, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} ${i18n.t('tasks.units.b')}`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} ${i18n.t('tasks.units.kb')}`
  return `${(bytes / 1024 / 1024).toFixed(1)} ${i18n.t('tasks.units.mb')}`
}

function initials(name: string): string {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map(p => p[0]).join('').toUpperCase() || '?'
}
</script>

<style scoped>
.task-card {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.modal-task-link {
  color: var(--accent);
  text-decoration: none;
  margin-right: 4px;
}

.modal-task-link:hover {
  text-decoration: underline;
}

/* Parameters */
.params {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 32px;
  padding: 16px 20px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.params-col {
  display: flex;
  flex-direction: column;
}

.param {
  display: grid;
  grid-template-columns: 120px 1fr;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  border-bottom: 1px solid var(--border-light);
}

.param:last-child {
  border-bottom: none;
}

.param-label {
  font-size: 12px;
  color: var(--text-muted);
}

.param-value {
  font-size: 13px;
  color: var(--text-bright);
  padding: 0 10px;
}

.param-value.num {
  font-variant-numeric: tabular-nums;
}

.param-hint {
  margin-left: 6px;
  font-size: 11px;
  color: var(--text-muted);
}

.param-control {
  display: flex;
  align-items: center;
  gap: 6px;
}

.unit {
  font-size: 12px;
  color: var(--text-muted);
}

.field-input {
  width: 100%;
  padding: 6px 10px;
  font-size: 13px;
  font-family: inherit;
  /* Editable fields are always framed, read-only values are plain text */
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text-bright);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.field-input:hover {
  border-color: var(--hairline-strong);
}

.field-input:focus {
  outline: none;
  background: var(--surface-1);
  border-color: var(--accent);
  cursor: text;
}

.field-input option {
  background: var(--surface-1);
  color: var(--text);
}

.num-input {
  width: 100px;
  font-variant-numeric: tabular-nums;
}

/* Sections */
.section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.section-count {
  padding: 1px 7px;
  font-size: 11px;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent);
  letter-spacing: 0;
}

.section-loading {
  display: flex;
  padding: 8px 0;
}

.empty {
  font-size: 13px;
  color: var(--text-faint);
}

.error-text {
  color: var(--danger);
}

/* Redmine markup */
.markup {
  font-size: 14px;
  line-height: 1.65;
  color: var(--text);
  overflow-wrap: anywhere;
}

.markup :deep(p) {
  margin: 0 0 10px;
}

.markup :deep(p:last-child) {
  margin-bottom: 0;
}

.markup :deep(h1),
.markup :deep(h2),
.markup :deep(h3),
.markup :deep(h4) {
  margin: 16px 0 8px;
  color: var(--text-bright);
  font-weight: 600;
  line-height: 1.3;
}

.markup :deep(h1) { font-size: 20px; }
.markup :deep(h2) { font-size: 17px; }
.markup :deep(h3) { font-size: 15px; }
.markup :deep(h4) { font-size: 14px; }

.markup :deep(ul),
.markup :deep(ol) {
  margin: 0 0 10px;
  padding-left: 24px;
}

.markup :deep(li) {
  margin: 2px 0;
}

.markup :deep(a) {
  color: var(--accent);
  text-decoration: none;
}

.markup :deep(a:hover) {
  text-decoration: underline;
}

.markup :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 8px 0;
  border-radius: 8px;
  border: 1px solid var(--hairline);
}

.markup :deep(code) {
  padding: 1px 5px;
  font-size: 12.5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  background: var(--surface-2);
  border-radius: 4px;
}

.markup :deep(pre) {
  margin: 0 0 10px;
  padding: 12px 14px;
  overflow-x: auto;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
}

.markup :deep(pre code) {
  padding: 0;
  background: none;
}

.markup :deep(blockquote) {
  margin: 0 0 10px;
  padding: 4px 14px;
  border-left: 3px solid var(--hairline-strong);
  color: var(--text-muted);
}

.markup :deep(table) {
  margin: 0 0 10px;
  border-collapse: collapse;
  font-size: 13px;
}

.markup :deep(th),
.markup :deep(td) {
  padding: 6px 10px;
  border: 1px solid var(--hairline);
  text-align: left;
}

.markup :deep(th) {
  background: var(--surface-2);
  color: var(--text-bright);
}

.markup :deep(hr) {
  border: none;
  border-top: 1px solid var(--hairline);
  margin: 12px 0;
}

/* Files */
.files {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 8px;
}

.file {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px 8px 8px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 10px;
  text-decoration: none;
  transition: border-color 0.15s;
  min-width: 0;
}

.file:hover {
  border-color: var(--accent);
}

.file-thumb,
.file-icon {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 6px;
}

.file-thumb {
  object-fit: cover;
}

.file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent-bg);
  color: var(--accent);
}

.file-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.file-name {
  font-size: 13px;
  color: var(--text-bright);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-meta {
  font-size: 11px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Comments */
.comments {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.comment {
  display: flex;
  gap: 12px;
}

.avatar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--accent-bg);
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
}

.comment-body {
  flex: 1;
  min-width: 0;
  padding: 10px 14px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 0 10px 10px 10px;
}

.comment-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.comment-author {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.comment-date {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.comment-text {
  font-size: 13px;
}

/* New comment */
.new-comment {
  padding-top: 20px;
  border-top: 1px solid var(--border-light);
}

.comment-input {
  width: 100%;
  min-height: 80px;
  padding: 10px 12px;
  font-size: 13px;
  font-family: inherit;
  line-height: 1.5;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 10px;
  color: var(--text);
  resize: vertical;
  outline: none;
}

.comment-input:focus {
  border-color: var(--accent);
}

.comment-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.comment-hint {
  font-size: 11px;
  color: var(--text-faint);
}

.loading-state {
  display: flex;
  justify-content: center;
  padding: 40px;
}

@media (max-width: 900px) {
  .params {
    grid-template-columns: 1fr;
  }
  .params-col + .params-col {
    border-top: 1px solid var(--border-light);
  }
}
</style>
