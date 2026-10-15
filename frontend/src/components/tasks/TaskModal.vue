<template>
  <AppModal
    :model-value="!!taskId"
    :title="task ? `#${task.external_id} ${task.subject}` : ''"
    width="80vw"
    expandable
    :page="isMobile"
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

    <div v-else-if="task" ref="cardEl" class="task-card carded" :class="isMobile ? 'mobile' : 'split'" @click="onCardClick">
      <!-- Телефон: проект и название над сводкой (на компьютере название — в заголовке окна) -->
      <template v-if="isMobile">
        <div class="m-project">{{ task.project_name }}</div>
        <div class="m-subject">{{ task.subject }}</div>
      </template>

      <!-- Слева — вкладки и разделы; справа — сводка и свойства. На телефоне
           обёртки не влияют на раскладку: всё идёт одной колонкой -->
      <div class="tc-main">
        <nav class="m-tabs">
          <button v-for="tab in mobileTabs" :key="tab.key" type="button" :class="{ on: mobileTab === tab.key }" :data-testid="'task-tab-' + tab.key" @click="setMobileTab(tab.key)">
            {{ $t(tab.label) }}<i v-if="tab.count">{{ tab.count }}</i>
          </button>
        </nav>


      <!-- Описание -->
      <section v-show="mobileTab === 'over'" class="section">
        <h4 class="section-title">{{ $t('tasks.modal.description') }}</h4>
        <div v-if="detailsLoading" class="section-loading"><AppSpinner :size="20" /></div>
        <div v-else-if="descriptionHtml" class="markup" v-html="descriptionHtml"></div>
        <div v-else class="empty">{{ $t('tasks.modal.description_empty') }}</div>
      </section>

      <!-- Файлы -->
      <section v-show="mobileTab === 'files'" class="section">
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

      <!-- Связанные задачи -->
      <section v-if="related.length" v-show="mobileTab === 'over'" class="section" data-testid="task-related">
        <h4 class="section-title">
          {{ $t('tasks.related.title') }}
          <span class="section-count">{{ related.length }}</span>
        </h4>
        <div class="related">
          <component
            :is="r.accessible ? 'button' : 'div'"
            v-for="r in related"
            :key="r.relation + r.id"
            class="related-row"
            :class="{ locked: !r.accessible }"
            :type="r.accessible ? 'button' : undefined"
            @click="r.accessible && openRelated(r.id)"
          >
            <span class="related-type">{{ relationName(r.relation) }}</span>
            <span class="related-id">#{{ r.id }}</span>
            <span class="related-subject">{{ r.accessible ? r.subject : $t('tasks.related.hidden') }}</span>
            <span v-if="r.accessible && r.status" class="related-status" :style="statusBadge(r.status)">{{ r.status }}</span>
          </component>
        </div>
      </section>

      <!-- Комментарии или вся история задачи; на телефоне — вкладки «Обсуждение» и «История» -->
      <section v-show="mobileTab === 'talk' || mobileTab === 'history'" class="section">
        <div v-if="detailsLoading" class="section-loading"><AppSpinner :size="20" /></div>
        <template v-else-if="historyMode === 'history'">
          <div v-if="history.length" class="comments">
            <div v-for="e in history" :key="e.id" class="comment">
              <span class="avatar">{{ initials(e.author) }}</span>
              <div class="comment-body">
                <div class="comment-header">
                  <span class="comment-author">{{ e.author }}</span>
                  <span class="comment-date">{{ formatDate(e.created_on) }}</span>
                </div>
                <ul v-if="e.changes.length" class="changes">
                  <li v-for="(change, i) in e.changes" :key="i">
                    <span class="change-field">{{ fieldName(change.field) }}:</span>
                    <template v-if="change.field === 'description'">{{ $t('tasks.history.changed') }}</template>
                    <template v-else>
                      <span class="change-old">{{ change.old || '—' }}</span>
                      <span class="change-arrow">→</span>
                      <span class="change-new">{{ change.new || '—' }}</span>
                    </template>
                  </li>
                </ul>
                <div v-if="e.html" class="markup comment-text" v-html="e.html"></div>
              </div>
            </div>
          </div>
          <div v-else class="empty">{{ $t('tasks.history.empty') }}</div>
        </template>
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

      <!-- Новый комментарий -->
      <section v-if="auth.canWrite" v-show="mobileTab === 'talk'" class="section new-comment">
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

      <aside class="tc-side">
        <div class="tc-summary">
        <div class="m-chips">
          <!-- Чип — это и есть выбор: поверх него прозрачный список, телефон показывает свой выбор значения -->
          <label class="m-chip" :style="chipColors(statusColor(editFields.status_name))">
            <span class="m-dot"></span>{{ editFields.status_name }} ▾
            <select v-model="editFields.status_name" :disabled="!auth.canWrite" :aria-label="$t('tasks.table.status')" @change="saveField('status_name')">
              <option v-for="s in statuses" :key="s.id" :value="s.name">{{ s.name }}</option>
            </select>
          </label>
          <label v-if="editFields.priority_name" class="m-chip m-chip-priority">
            {{ editFields.priority_name }} ▾
            <select v-model="editFields.priority_name" :disabled="!auth.canWrite" :aria-label="$t('tasks.table.priority')" @change="saveField('priority_name')">
              <option v-for="p in priorityNames" :key="p" :value="p">{{ p }}</option>
            </select>
          </label>
        </div>

        <div class="m-kpi">
          <label class="m-box" :class="dueState.kind">
            <span class="m-label">{{ $t(dueState.label) }}</span>
            <b class="m-big">{{ dueState.value }}</b>
            <span class="m-small">{{ task.due_date ? formatDay(task.due_date) : $t('tasks.mobile.card.no_due') }}</span>
            <input type="date" v-model="editFields.due_date" :disabled="!auth.canWrite" :aria-label="$t('tasks.modal.deadline')" class="m-overlay" @change="saveField('due_date')" />
          </label>
          <div class="m-box m-hours">
            <svg viewBox="0 0 36 36" class="m-ring" :class="{ over: hoursPct > 100 }">
              <circle cx="18" cy="18" r="15" class="m-ring-track" />
              <circle cx="18" cy="18" r="15" class="m-ring-fill" :stroke-dasharray="`${Math.min(hoursPct, 100) * 0.9425} 94.25`" />
            </svg>
            <div>
              <span class="m-label">{{ $t('tasks.mobile.card.hours') }}</span>
              <b class="m-big">{{ formatHours(task.spent_hours) }}<small v-if="task.estimated_hours">/{{ formatHours(task.estimated_hours) }}</small></b>
              <span v-if="task.bug_fix_hours" class="m-small">{{ $t('tasks.mobile.card.bug_fix', { h: formatHours(task.bug_fix_hours) }) }}</span>
            </div>
          </div>
        </div>

        <label class="m-assignee">
          <span class="m-avatar" :style="{ background: avatarColor(editFields.assigned_to_name) }">
            <img v-if="avatarSrc(task.assigned_to_id, editFields.assigned_to_name)" :src="avatarSrc(task.assigned_to_id, editFields.assigned_to_name)" alt="" />
            <template v-else>{{ initials(editFields.assigned_to_name || '?') }}</template>
          </span>
          <span class="m-assignee-name">{{ editFields.assigned_to_name || $t('tasks.modal.no_assignee') }}</span>
          <span class="m-chev">›</span>
          <select v-model="editFields.assigned_to_name" :disabled="!auth.canWrite" :aria-label="$t('tasks.table.assignee')" @change="saveField('assigned_to_name')">
            <option value="">{{ $t('tasks.modal.no_assignee') }}</option>
            <option v-for="m in memberNames" :key="m" :value="m">{{ m }}</option>
          </select>
        </label>

        </div>

      <!-- Параметры -->
      <section v-show="!isMobile || mobileTab === 'over'" class="params">
        <div class="params-col">
          <div v-if="!isMobile" class="param">
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
        </div>

        <div class="params-col">
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
        </div>
      </section>
      </aside>
    </div>

    <!-- Картинки задачи на весь экран -->
    <AppImageViewer v-model:index="viewerIndex" :images="viewerImages" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, reactive } from 'vue'
import { formatDate as formatCalendarDate, formatDateTime } from '../../utils/format'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { FileText } from 'lucide-vue-next'
import AppModal from '../ui/AppModal.vue'
import AppImageViewer, { type ViewerImage } from '../ui/AppImageViewer.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import AppButton from '../ui/AppButton.vue'
import { useTasksStore, type Task } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'
import { useSwal } from '../../composables/useSwal'
import { renderRedmineMarkup } from '../../utils/redmineMarkup'
import { statusColor } from '../../utils/statusColor'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { useMemberAvatars } from '../../composables/useMemberAvatars'
import { avatarColor } from '../../utils/avatar'

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

interface HistoryEntry extends Comment {
  changes: { field: string; old: string; new: string }[]
}

interface RelatedIssue {
  relation: string
  id: number
  subject: string
  status: string
  accessible: boolean
}

const props = defineProps<{ taskId: number | null }>()
defineEmits(['close'])

const store = useTasksStore()
const settingsStore = useSettingsStore()
const { settings } = storeToRefs(settingsStore)
const { statuses, priorities, members, projectCategories } = storeToRefs(store)
const { showChange, error: swalError, toast } = useSwal()
const auth = useAuthStore()
// Имя "t" занято локальной переменной в saveField
const i18n = useI18n()

// Показанная задача: та, для которой открыли карточку, или связанная, на
// которую пользователь перешёл из неё.
const currentId = ref<number | null>(props.taskId)
watch(() => props.taskId, id => { currentId.value = id })
// Другая задача открывается с вкладки «Обзор»
watch(currentId, () => setMobileTab('over'))

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

// Часть карточки, которая читается из Redmine напрямую
const description = ref('')
const attachments = ref<Attachment[]>([])
const rawComments = ref<Comment[]>([])
const rawHistory = ref<HistoryEntry[]>([])
const related = ref<RelatedIssue[]>([])
const historyMode = ref<'comments' | 'history'>('comments')

// Просмотр картинок: нажатие на картинку в описании, комментарии или на
// превью файла открывает её на весь экран; листаются все картинки задачи
const cardEl = ref<HTMLElement | null>(null)
const viewerImages = ref<ViewerImage[]>([])
const viewerIndex = ref<number | null>(null)

function onCardClick(e: MouseEvent) {
  const img = (e.target as HTMLElement).closest('img')
  if (!img || !cardEl.value || !(img.closest('.markup') || img.classList.contains('file-thumb'))) return
  e.preventDefault()
  const all = [...cardEl.value.querySelectorAll<HTMLImageElement>('.markup img, img.file-thumb')]
  const seen = new Set<string>()
  const list: ViewerImage[] = []
  for (const el of all) {
    if (!el.src || seen.has(el.src)) continue
    seen.add(el.src)
    list.push({ src: el.src, alt: el.alt || '' })
  }
  viewerImages.value = list
  viewerIndex.value = Math.max(0, list.findIndex(item => item.src === img.src))
}

// Телефон: карточка — отдельный экран с вкладками
const { isMobile } = useMobileTasks()
const { avatarSrc } = useMemberAvatars()
const mobileTab = ref<'over' | 'talk' | 'files' | 'history'>('over')
const mobileTabs = computed(() => [
  { key: 'over' as const, label: isMobile.value ? 'tasks.mobile.card.tab_over' : 'tasks.modal.description', count: 0 },
  { key: 'talk' as const, label: 'tasks.modal.comments', count: comments.value.length },
  { key: 'files' as const, label: 'tasks.modal.files', count: attachments.value.length },
  { key: 'history' as const, label: 'tasks.history.title', count: history.value.length },
])

// Вкладки «Обсуждение» и «История» показывают тот же раздел в разных режимах
function setMobileTab(key: 'over' | 'talk' | 'files' | 'history') {
  mobileTab.value = key
  if (key === 'talk') historyMode.value = 'comments'
  if (key === 'history') historyMode.value = 'history'
}

// Сколько осталось до срока: дни, «сегодня» или сколько дней просрочено
const dueState = computed(() => {
  const due = task.value?.due_date?.slice(0, 10)
  if (!due) return { kind: 'none', label: 'tasks.mobile.card.due_left', value: '—' }
  const today = new Date()
  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
  const days = Math.round((Date.parse(due) - Date.parse(todayStr)) / 86_400_000)
  if (days < 0) return { kind: 'overdue', label: 'tasks.mobile.card.overdue', value: i18n.t('tasks.mobile.card.days', { n: -days }) }
  if (days === 0) return { kind: 'today', label: 'tasks.mobile.card.due_left', value: i18n.t('tasks.mobile.card.today') }
  return { kind: days <= 3 ? 'soon' : 'ok', label: 'tasks.mobile.card.due_left', value: i18n.t('tasks.mobile.card.days', { n: days }) }
})

// Доля потраченных часов от плана, в процентах
const hoursPct = computed(() => {
  const est = Number(task.value?.estimated_hours) || 0
  return est > 0 ? Math.round(((Number(task.value?.spent_hours) || 0) / est) * 100) : 0
})

// Цвета чипа статуса — от цвета самого статуса
function chipColors(color: string) {
  return { color, background: `color-mix(in srgb, ${color} 18%, transparent)` }
}
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

// Все категории проекта задачи; текущую по-прежнему можно выбрать.
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

const history = computed(() =>
  rawHistory.value.map(e => ({ ...e, html: e.text ? renderRedmineMarkup(e.text, markupOptions()) : '' }))
)

// Название изменённого поля; поле без перевода показывается так, как его
// называет источник.
function fieldName(field: string): string {
  const key = `tasks.history.fields.${field.startsWith('cf_') ? 'cf' : field}`
  return i18n.te(key) ? i18n.t(key) : field
}

function statusBadge(status: string) {
  const color = statusColor(status)
  return { color, background: color + '22' }
}

function relationName(relation: string): string {
  const key = `tasks.related.types.${relation}`
  return i18n.te(key) ? i18n.t(key) : relation
}

function openRelated(id: number) {
  currentId.value = id
}

function attachmentUrl(id: number): string {
  return `/api/tasks/${task.value?.external_id}/attachments/${id}`
}

function isImage(a: Attachment): boolean {
  return /^image\/(png|jpe?g|gif|webp|bmp)$/i.test(a.content_type)
}

function taskLink(id: number): string {
  return redmineUrl.value ? `${redmineUrl.value}/issues/${id}` : '#'
}

// quiet: перечитать в фоне, без индикаторов загрузки
async function loadDetails(id: number, quiet = false) {
  if (!quiet) detailsLoading.value = true
  detailsError.value = ''
  try {
    const { data } = await axios.get(`/api/tasks/${id}/details`)
    if (task.value?.external_id !== id) return
    description.value = data.description || ''
    attachments.value = data.attachments || []
    rawComments.value = data.comments || []
    rawHistory.value = data.history || []
    related.value = data.related || []
  } catch {
    if (quiet) return
    if (task.value?.external_id !== id) return
    // Запасной вариант — описание из локальной копии
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
    // Новый комментарий — тоже часть истории
    loadDetails(id, true)
    toast(i18n.t('tasks.messages.comment_added'), 'success')
  } catch {
    toast(i18n.t('tasks.messages.comment_error'), 'error')
  } finally {
    sendingComment.value = false
  }
}

watch(currentId, async (id) => {
  task.value = null
  description.value = ''
  attachments.value = []
  rawComments.value = []
  rawHistory.value = []
  related.value = []
  historyMode.value = 'comments'
  newComment.value = ''
  if (!id) return

  loading.value = true
  // Справочников может не быть, если карточка открыта не с вкладки «Задачи»
  if (!statuses.value.length) store.fetchStatuses()
  if (!priorities.value.length) store.fetchPriorities()
  store.fetchMembers()
  if (!redmineUrl.value) settingsStore.fetchSettings()

  const loaded = await store.fetchTask(id)
  if (currentId.value !== id) return
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
    // Обычное текстовое поле: принимаются и "1,5", и "1.5"
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
    showChange(String(old ?? '—'), String(value ?? '—'), i18n.t(fieldLabels[field]), task.value.external_id)
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

// Срок и дата старта — календарные даты: берётся только день, без пояса
function formatDay(d?: string | null): string {
  return formatCalendarDate(d ? d.slice(0, 10) : null)
}

// Время комментария или файла — в поясе пользователя
function formatDate(d?: string): string {
  return formatDateTime(d, { empty: '' })
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

/* Параметры */
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
  /* Редактируемые поля всегда в рамке, значения только для чтения — обычный текст */
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

.field-input:focus-visible {
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

/* Разделы */
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

/* Разметка Redmine */
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

.markup :deep(img),
.file-thumb {
  cursor: zoom-in;
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
  border-radius: 8px;
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

/* Файлы */
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
  border-radius: 12px;
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
  border-radius: 8px;
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

/* Комментарии */
.comments {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Комментарии / вся история */
.mode-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font: inherit;
  text-transform: inherit;
  letter-spacing: inherit;
  color: var(--text-muted);
  cursor: pointer;
}

.mode-pill:hover {
  color: var(--text);
}

.mode-pill.active {
  background: var(--surface-2);
  color: var(--text-bright);
}

.changes {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
  color: var(--text);
}

.change-field {
  color: var(--text-muted);
  margin-right: 4px;
}

.change-old {
  color: var(--text-muted);
  text-decoration: line-through;
}

.change-arrow {
  margin: 0 6px;
  color: var(--text-faint);
}

/* Связанные задачи */
.related {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.related-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  font-size: 13px;
  color: var(--text);
  text-align: left;
}

button.related-row {
  cursor: pointer;
}

button.related-row:hover {
  border-color: var(--accent);
}

.related-row.locked {
  color: var(--text-muted);
  background: transparent;
}

.related-type {
  flex-shrink: 0;
  min-width: 110px;
  font-size: 12px;
  color: var(--text-muted);
}

.related-id {
  flex-shrink: 0;
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}

.related-row.locked .related-id {
  color: var(--text-muted);
}

.related-subject {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.related-status {
  flex-shrink: 0;
  padding: 2px 8px;
  font-size: 11px;
  border-radius: 9999px;
  background: var(--tag-bg);
  color: var(--text-muted);
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
  border-radius: 0 12px 12px 12px;
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

/* Новый комментарий */
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
  border-radius: 12px;
  color: var(--text);
  resize: vertical;
  outline: none;
}

.comment-input:focus-visible {
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
    grid-template-columns: minmax(0, 1fr);
  }
  .params-col + .params-col {
    border-top: 1px solid var(--border-light);
  }
}

/* На телефоне подписи параметров уже, значение не выходит за рамку блока */
@media (max-width: 480px) {
  .params {
    padding: 12px;
  }
  .param {
    grid-template-columns: 96px minmax(0, 1fr);
  }
}
/* ── Телефон: карточка-экран с вкладками ── */
.m-project {
  font-size: 12px;
  color: var(--text-muted);
}

.m-subject {
  margin: 2px 0 10px;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.3;
  color: var(--text-bright);
  overflow-wrap: anywhere;
}

.m-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.m-chip,
.m-assignee,
.m-box {
  position: relative;
}

.m-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.m-chip-priority {
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 18%, transparent);
}

.m-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

/* Прозрачный список поверх чипа или строки: нажатие открывает выбор телефона */
.m-chip select,
.m-assignee select,
.m-overlay {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  font-size: 16px;
  cursor: pointer;
}

.m-kpi {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 12px;
}

.m-box {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.m-hours {
  flex-direction: row;
  align-items: center;
  gap: 10px;
}

.m-hours > div {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.m-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.m-big {
  font-size: 22px;
  font-weight: 700;
  line-height: 1.15;
  color: var(--text-bright);
  white-space: nowrap;
}

.m-big small {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
}

.m-small {
  font-size: 12px;
  color: var(--text-muted);
}

.m-box.overdue .m-big { color: var(--danger); }
.m-box.today .m-big,
.m-box.soon .m-big { color: var(--warning); }

.m-ring {
  width: 46px;
  height: 46px;
  flex: none;
}

.m-ring circle {
  fill: none;
  stroke-width: 4;
}

.m-ring-track {
  stroke: var(--surface-3);
}

.m-ring-fill {
  stroke: var(--accent);
  stroke-linecap: round;
  transform: rotate(-90deg);
  transform-origin: 18px 18px;
}

.m-ring.over .m-ring-fill {
  stroke: var(--danger);
}

.m-assignee {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0 12px;
  padding: 9px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.m-avatar {
  width: 28px;
  height: 28px;
  flex: none;
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

.m-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.m-assignee-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-bright);
}

.m-chev {
  color: var(--text-faint);
}

.m-tabs {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  gap: 2px;
  margin: 0 -16px 12px;
  padding: 0 12px;
  overflow-x: auto;
  scrollbar-width: none;
  background: var(--surface-1);
  border-bottom: 1px solid var(--border);
}

.m-tabs button {
  flex: none;
  padding: 10px 8px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.m-tabs button.on {
  color: var(--text-bright);
  border-bottom-color: var(--accent);
}

.m-tabs i {
  margin-left: 4px;
  font-style: normal;
  font-weight: 500;
  color: var(--text-faint);
}

/* Свойства на «Обзоре» — список в карточке: название слева, значение справа */
.task-card.carded .params {
  display: block;
  padding: 0;
  overflow: hidden;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.task-card.carded .params-col {
  display: contents;
}

.task-card.carded .param {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 50px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border-light);
}

.task-card.carded .param:last-child {
  border-bottom: 1px solid var(--border-light);
}

.task-card.carded .params-col:last-child .param:last-child {
  border-bottom: none;
}

.task-card.carded .param-label {
  flex: 1;
  font-size: 14px;
  color: var(--text-muted);
}

.task-card.carded .param-value {
  padding: 0;
  font-size: 15px;
  color: var(--text-bright);
  text-align: right;
}

.task-card.carded .param-control {
  justify-content: flex-end;
  gap: 4px;
}

/* Поля без рамок: значение справа, как обычный текст */
.task-card.carded .field-input {
  width: auto;
  max-width: 60vw;
  padding: 6px 0;
  border: 0;
  background: none;
  font-size: 16px;
  text-align: right;
  text-align-last: right;
  appearance: none;
  -webkit-appearance: none;
}

.task-card.carded .num-input {
  width: 64px;
}

/* У изменяемых значений справа «›» */
.task-card.carded .param:has(select:not(:disabled))::after,
.task-card.carded .param:has(input:not(:disabled))::after {
  content: '›';
  font-size: 18px;
  color: var(--text-faint);
}

/* ── Компьютер: две колонки — вкладки слева, сводка и свойства справа ── */
.task-card.split {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  align-items: start;
  gap: 24px;
}

.task-card.split .tc-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.task-card.split .m-tabs {
  position: static;
  margin: 0;
  padding: 0;
}

.task-card.split .tc-side {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-card.split .tc-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-card.split .m-kpi {
  margin: 0;
}

.task-card.split .m-assignee {
  margin: 0;
}

.task-card.split .field-input {
  max-width: 180px;
}

/* ── Телефон: обёртки не участвуют в раскладке, порядок — как раньше ── */
.task-card.mobile .tc-main,
.task-card.mobile .tc-side,
.task-card.mobile .tc-summary {
  display: contents;
}

.task-card.mobile .m-chips,
.task-card.mobile .m-kpi,
.task-card.mobile .m-assignee { order: 1; }
.task-card.mobile .m-tabs { order: 2; }
.task-card.mobile .params { order: 3; }
.task-card.mobile .section { order: 4; }

/* Окно уже — колонки друг под другом */
@media (max-width: 900px) {
  .task-card.split {
    grid-template-columns: minmax(0, 1fr);
  }
  .task-card.split .tc-side {
    position: static;
    order: -1;
  }
}
</style>

