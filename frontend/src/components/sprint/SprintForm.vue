<template>
  <AppModal
    :model-value="visible"
    :title="sprint ? $t('sprint.edit_sprint') : $t('sprint.new_sprint')"
    width="520px"
    @update:model-value="!$event && $emit('close')"
  >
    <template v-if="sprint" #title>
      {{ $t('sprint.edit_sprint') }}
      <SprintStateBadge :sprint="sprint" class="title-state" />
    </template>
    <!-- Телефон: поля строками в блоках, как в настройках телефона; значение справа -->
    <form v-if="isMobile" class="ios-form" data-testid="sprint-form-mobile" @submit.prevent="submit">
      <div class="ios-title">{{ $t('sprint.form.group_sprint') }}</div>
      <div class="ios-group">
        <label class="ios-cell">
          <span class="ios-name">{{ $t('sprint.form.name') }}</span>
          <input v-model="form.name" type="text" required :placeholder="$t('sprint.form.name_placeholder')" />
        </label>
        <label class="ios-cell ios-pick">
          <span class="ios-name">{{ $t('sprint.form.kind') }}</span>
          <select v-model="form.is_shared" :disabled="!canChangeKind">
            <option :value="false">{{ $t('sprint.kind.personal') }}</option>
            <option :value="true">{{ $t('sprint.kind.shared') }}</option>
          </select>
        </label>
      </div>
      <div class="ios-footnote">{{ $t(form.is_shared ? 'sprint.kind.shared_hint' : 'sprint.kind.personal_hint') }}</div>

      <div class="ios-title">{{ $t('sprint.form.group_dates') }}</div>
      <div class="ios-group">
        <label class="ios-cell ios-pick">
          <span class="ios-name">{{ $t('sprint.form.start_date') }}</span>
          <input v-model="form.start_date" type="date" required />
        </label>
        <label class="ios-cell ios-pick">
          <span class="ios-name">{{ $t('sprint.form.due_date') }}</span>
          <input v-model="form.due_date" type="date" :min="form.start_date" />
        </label>
      </div>
      <div v-if="!datesInOrder" class="ios-footnote ios-error">{{ $t('sprint.form.due_before_start') }}</div>

      <div class="ios-title">{{ $t('sprint.form.group_tasks') }}</div>
      <div class="ios-group">
        <label class="ios-cell ios-pick">
          <span class="ios-name">{{ $t('sprint.form.project') }}</span>
          <select v-model="form.project_name" @change="onProjectChange">
            <option value="">—</option>
            <option v-for="name in projectOptions" :key="name" :value="name">{{ name }}</option>
          </select>
        </label>
        <label class="ios-cell ios-pick" :class="{ disabled: !projectId }">
          <span class="ios-name">{{ $t('sprint.form.category') }}</span>
          <select v-model="form.category_name" :disabled="!projectId">
            <option value="">{{ projectId ? '—' : $t('sprint.form.category_pick_project') }}</option>
            <option v-for="c in categoryOptions" :key="c" :value="c">{{ c }}</option>
          </select>
        </label>
        <label class="ios-cell">
          <span class="ios-name">{{ $t('sprint.form.auto_fill') }}</span>
          <input v-model="form.auto_fill_category" type="checkbox" class="ios-switch" />
        </label>
      </div>

      <div class="ios-title">{{ $t('sprint.form.description') }}</div>
      <div class="ios-group">
        <textarea v-model="form.description" rows="3" class="ios-text" :placeholder="$t('sprint.form.description_placeholder')"></textarea>
      </div>
    </form>

    <form v-else class="sprint-form" @submit.prevent="submit">
      <div class="form-field">
        <label>{{ $t('sprint.form.name') }} *</label>
        <input v-model="form.name" type="text" required />
      </div>

      <!-- Вид спринта меняет только его создатель -->
      <div class="form-field">
        <label>{{ $t('sprint.form.kind') }}</label>
        <div class="kind-switch">
          <label class="kind-option">
            <input v-model="form.is_shared" type="radio" :value="false" :disabled="!canChangeKind" />
            {{ $t('sprint.kind.personal') }}
          </label>
          <label class="kind-option">
            <input v-model="form.is_shared" type="radio" :value="true" :disabled="!canChangeKind" />
            {{ $t('sprint.kind.shared') }}
          </label>
        </div>
        <div class="kind-hint">{{ $t(form.is_shared ? 'sprint.kind.shared_hint' : 'sprint.kind.personal_hint') }}</div>
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.project') }}</label>
        <select v-model="form.project_name" @change="onProjectChange">
          <option value="">—</option>
          <option v-for="name in projectOptions" :key="name" :value="name">{{ name }}</option>
        </select>
      </div>

      <div class="form-field">
        <label>{{ $t('sprint.form.category') }}</label>
        <!-- Категории выбранного проекта -->
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
          <input v-model="form.due_date" type="date" :min="form.start_date" />
        </div>
      </div>
      <div v-if="!datesInOrder" class="date-error">{{ $t('sprint.form.due_before_start') }}</div>

      <div class="form-field">
        <label>{{ $t('sprint.form.description') }}</label>
        <textarea v-model="form.description" rows="3"></textarea>
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
import SprintStateBadge from './SprintStateBadge.vue'
import { useTasksStore } from '../../stores/tasks'
import { useMobileTasks } from '../../composables/useMobileTasks'

const tasksStore = useTasksStore()
const { isMobile } = useMobileTasks()

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
  is_shared: false,
})

// Новый спринт — свой; у чужого общего спринта вид не меняется
const canChangeKind = computed(() => !props.sprint || props.sprint.is_own)

// В списке — проекты, выбранные пользователем в настройках. Проект, уже
// заданный у спринта, остаётся в списке, даже если его больше нет среди них.
const projectOptions = computed(() => {
  const names = props.projects.map(p => p.name)
  return form.project_name && !names.includes(form.project_name) ? [form.project_name, ...names] : names
})

const projectId = computed(() => props.projects.find(p => p.name === form.project_name)?.id)

// Сохранённая ранее категория остаётся в списке, даже если в проекте её уже нет
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

// Заполняется при каждом открытии окна: новый спринт начинается с пустой формы
watch([() => props.visible, () => props.sprint], ([visible, s]) => {
  if (!visible) return
  if (s) {
    form.name = s.name
    form.project_name = s.project_name || ''
    form.category_name = s.category_name || ''
    form.auto_fill_category = s.auto_fill_category
    // Сервер присылает дату со временем; в поле идёт только дата
    form.start_date = (s.start_date || '').slice(0, 10)
    form.due_date = (s.due_date || '').slice(0, 10)
    form.description = s.description || ''
    form.is_shared = s.is_shared
  } else {
    form.is_shared = false
    form.name = ''
    form.project_name = ''
    form.category_name = ''
    form.auto_fill_category = false
    form.start_date = ''
    form.due_date = ''
    form.description = ''
  }
  loadCategories()
}, { immediate: true })

// Срок не раньше начала; без срока сравнивать не с чем
const datesInOrder = computed(() => !form.start_date || !form.due_date || form.due_date >= form.start_date)

// Название и дата начала обязательны
const canSubmit = computed(() => form.name.trim() !== '' && form.start_date !== '' && datesInOrder.value)

function submit() {
  if (!canSubmit.value) return
  // Незаполненное поле — «не задано», а не пустая строка
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
.title-state {
  margin-left: 8px;
}

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

.form-field input:focus-visible,
.form-field select:focus-visible,
.form-field textarea:focus-visible {
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

.date-error {
  margin-top: -8px;
  font-size: 12px;
  color: var(--danger);
}

.checkbox-field label {
  display: flex;
  align-items: center;
  gap: 8px;
  text-transform: none;
  font-size: 13px;
  cursor: pointer;
}

.kind-switch {
  display: flex;
  gap: 20px;
}

.form-field .kind-option {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 0;
  text-transform: none;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}

.kind-hint {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--text-muted);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

/* Поле даты на iOS шире половины экрана — иначе оно распирает форму */
.form-row .form-field {
  min-width: 0;
}

@media (max-width: 640px) {
  .form-row {
    grid-template-columns: 1fr;
  }

  /* 16px — меньше iOS увеличивает страницу при фокусе в поле */
  .form-field input[type="text"],
  .form-field input[type="date"],
  .form-field select,
  .form-field textarea {
    font-size: 16px;
  }
}

/* Телефон: форма в стиле настроек телефона */
.ios-title {
  margin: 0 4px 7px;
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ios-group {
  margin-bottom: 18px;
  overflow: hidden;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 13px;
}

.ios-footnote {
  margin: -11px 6px 18px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--text-muted);
}

.ios-error {
  color: var(--danger);
}

.ios-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  font-size: 16px;
  color: var(--text-bright);
  cursor: pointer;
}

.ios-cell:last-child {
  border-bottom: 0;
}

.ios-cell.disabled {
  opacity: 0.55;
}

.ios-name {
  flex: none;
  max-width: 60%;
}

/* Значение справа, без рамки; 16px — меньше iOS увеличивает страницу при фокусе */
.ios-cell input[type="text"],
.ios-cell input[type="date"],
.ios-cell select {
  flex: 1;
  min-width: 0;
  padding: 12px 0;
  border: 0;
  background: none;
  color: var(--text-muted);
  font-size: 16px;
  text-align: right;
  text-align-last: right;
  appearance: none;
  -webkit-appearance: none;
  outline: none;
}

.ios-cell input[type="text"] {
  color: var(--text-bright);
}

.ios-cell input[type="date"]::-webkit-date-and-time-value {
  text-align: right;
}

/* Стрелка у строк с выбором */
.ios-pick::after {
  content: '›';
  flex: none;
  margin-left: -4px;
  font-size: 20px;
  color: var(--text-faint);
}

/* Переключатель вместо галочки */
.ios-switch {
  appearance: none;
  -webkit-appearance: none;
  position: relative;
  flex: none;
  width: 46px;
  height: 28px;
  margin-left: auto;
  border-radius: 14px;
  background: var(--surface-4);
  transition: background 0.2s;
  cursor: pointer;
}

.ios-switch::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  transition: transform 0.2s;
}

.ios-switch:checked {
  background: var(--accent);
}

.ios-switch:checked::after {
  transform: translateX(18px);
}

.ios-text {
  display: block;
  width: 100%;
  padding: 12px 14px;
  border: 0;
  background: none;
  color: var(--text-bright);
  font-size: 16px;
  resize: vertical;
  outline: none;
}
</style>
