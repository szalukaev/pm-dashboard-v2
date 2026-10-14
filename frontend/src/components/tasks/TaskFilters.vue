<template>
  <!-- Телефон: поиск на виду, остальные фильтры — за иконкой на отдельном экране -->
  <div v-if="isMobile" class="mobile-filters">
    <div class="m-search">
      <Search :size="16" class="m-search-icon" />
      <input
        v-model="searchInput"
        type="search"
        enterkeyhint="search"
        :placeholder="$t('tasks.filters.search_placeholder')"
        data-testid="filter-search"
        @keyup.enter="doSearch"
      />
      <button
        class="m-filter-btn"
        :class="{ active: activeChips.length > 0 }"
        :aria-label="$t('tasks.filters.title')"
        data-testid="filter-open"
        @click="panelOpen = true"
      >
        <Filter :size="18" />
        <span v-if="activeChips.length" class="m-badge">{{ activeChips.length }}</span>
      </button>
    </div>

    <div v-if="activeChips.length" class="m-chips">
      <button
        v-for="chip in activeChips"
        :key="chip.key"
        class="m-chip"
        :data-testid="'filter-chip-' + chip.key"
        @click="chip.clear()"
      >
        {{ chip.label }} <X :size="12" />
      </button>
    </div>

    <Teleport to="body">
      <Transition name="m-panel">
        <div v-if="panelOpen" class="m-panel" data-testid="filter-panel">
          <div class="m-panel-head">
            <button class="m-back" :aria-label="$t('common.back')" @click="panelOpen = false">
              <ChevronLeft :size="22" />
            </button>
            <b>{{ $t('tasks.filters.title') }}</b>
            <button class="m-reset" data-testid="filter-reset" @click="resetAll">{{ $t('tasks.filters.reset') }}</button>
          </div>

          <div class="m-panel-body">
            <div class="m-label">{{ $t('tasks.filters.tasks') }}</div>
            <div class="m-pills">
              <button
                v-for="t in typeOptions"
                :key="t.value"
                :class="{ active: filters.type === t.value }"
                :data-testid="'filter-type-' + t.value"
                @click="setFilter('type', t.value)"
              >{{ $t(t.label) }}</button>
            </div>

            <div class="m-label">{{ $t('tasks.filters.project') }}</div>
            <select v-model="filters.project_id" class="m-select" data-testid="filter-project" @change="onProjectChange">
              <option value="">{{ $t('tasks.filters.all_projects') }}</option>
              <option v-for="p in visibleProjects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>

            <template v-if="categories.length > 0 && filters.project_id">
              <div class="m-label">{{ $t('tasks.filters.category') }}</div>
              <div class="m-pills">
                <button
                  v-for="cat in categories"
                  :key="cat"
                  :class="{ active: filters.category === cat }"
                  :data-testid="'filter-cat-' + cat"
                  @click="setFilter('category', filters.category === cat ? '' : cat)"
                >{{ cat }}</button>
              </div>
            </template>

            <div class="m-label">{{ $t('tasks.filters.grouping') }}</div>
            <div class="m-pills">
              <button :class="{ active: useGrouping && filters.group_by === 'project' }" data-testid="filter-group-project" @click="setGrouping('project')">{{ $t('tasks.filters.group_by_project') }}</button>
              <button :class="{ active: useGrouping && filters.group_by === 'assignee' }" data-testid="filter-group-assignee" @click="setGrouping('assignee')">{{ $t('tasks.filters.group_by_assignee') }}</button>
              <button :class="{ active: !useGrouping }" data-testid="filter-no-group" @click="setGrouping('')">{{ $t('tasks.filters.no_grouping') }}</button>
            </div>
          </div>

          <div class="m-panel-foot">
            <button class="m-show" data-testid="filter-show" @click="panelOpen = false">
              {{ $t('tasks.filters.show_tasks', { n: total }) }}
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>

  <div v-else class="task-filters">
    <!-- Строка 1: статус, проект, поиск, сброс, счётчик -->
    <div class="filter-bar">
      <button
        v-for="t in typeOptions"
        :key="t.value"
        class="pill-btn"
        :class="{ active: filters.type === t.value }"
        @click="setFilter('type', t.value)"
        :data-testid="'filter-type-' + t.value"
      >
        {{ $t(t.label) }}
      </button>

      <select
        v-model="filters.project_id"
        @change="onProjectChange"
        class="field filter-select"
        data-testid="filter-project"
      >
        <option value="">{{ $t('tasks.filters.all_projects') }}</option>
        <option v-for="p in visibleProjects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>

      <input
        v-model="searchInput"
        type="text"
        :placeholder="$t('tasks.filters.search_placeholder')"
        class="field search-input"
        data-testid="filter-search"
        @keyup.enter="doSearch"
      />

      <button class="reset-btn" data-testid="filter-reset" @click="resetAll">
        <RotateCcw :size="14" /> {{ $t('tasks.filters.reset') }}
      </button>

      <span class="found">{{ $t('tasks.filters.found') }}: <strong>{{ total }}</strong></span>
    </div>

    <!-- Строка 2: группировка, категории -->
    <div class="filter-row">
      <div class="filter-group">
        <span class="filter-label">{{ $t('tasks.filters.grouping') }}:</span>
        <button
          class="pill-btn compact"
          :class="{ active: useGrouping && filters.group_by === 'project' }"
          @click="setGrouping('project')"
          data-testid="filter-group-project"
        >
          {{ $t('tasks.filters.group_by_project') }}
        </button>
        <button
          class="pill-btn compact"
          :class="{ active: useGrouping && filters.group_by === 'assignee' }"
          @click="setGrouping('assignee')"
          data-testid="filter-group-assignee"
        >
          {{ $t('tasks.filters.group_by_assignee') }}
        </button>
        <button
          class="pill-btn compact"
          :class="{ active: !useGrouping }"
          @click="setGrouping('')"
          data-testid="filter-no-group"
        >
          {{ $t('common.all') }}
        </button>
      </div>

      <AppColumnSettings class="columns-gear" :table="TASKS_TABLE" :columns="TASK_COLUMNS" />

      <div class="filter-group" v-if="categories.length > 0 && filters.project_id">
        <span class="filter-label">{{ $t('tasks.filters.category') }}:</span>
        <button
          v-for="cat in categories"
          :key="cat"
          class="pill-btn compact"
          :class="{ active: filters.category === cat }"
          @click="setFilter('category', filters.category === cat ? '' : cat)"
          :data-testid="'filter-cat-' + cat"
        >
          {{ cat }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import AppColumnSettings from '../ui/AppColumnSettings.vue'
import { TASK_COLUMNS, TASKS_TABLE } from './taskColumns'
import { ref, watch, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { RotateCcw, Search, Filter, X, ChevronLeft } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { useTasksStore } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'

const store = useTasksStore()
const settingsStore = useSettingsStore()
const { filters, projects, categories, useGrouping, total } = storeToRefs(store)
const { settings } = storeToRefs(settingsStore)
const { t } = useI18n()
const { isMobile } = useMobileTasks()

// Экран фильтров на телефоне
const panelOpen = ref(false)

// Показываются только проекты, выбранные в настройках пользователя (если выбраны)
const visibleProjects = computed(() => {
  const selected = settings.value.selected_projects || []
  if (selected.length === 0) return projects.value
  return projects.value.filter(p => selected.includes(p.id))
})

const searchInput = ref(filters.value.search)
// Сохранённые фильтры восстанавливаются после создания этого компонента.
watch(() => filters.value.search, (val) => { searchInput.value = val })

// Настройки нужны, чтобы взять из них selected_projects для отбора
settingsStore.fetchSettings()

const typeOptions = [
  { value: 'open', label: 'tasks.filters.open' },
  { value: 'testing', label: 'tasks.filters.testing' },
  { value: 'closed', label: 'tasks.filters.closed' },
  { value: 'all', label: 'tasks.filters.all' },
]

// Включённые фильтры, которые отличаются от значений по умолчанию, —
// чипы под строкой поиска на телефоне; нажатие на чип снимает фильтр
const activeChips = computed(() => {
  const chips: { key: string; label: string; clear: () => void }[] = []
  const f = filters.value
  if (f.type !== 'open') {
    const option = typeOptions.find(o => o.value === f.type)
    chips.push({ key: 'type', label: option ? t(option.label) : f.type, clear: () => setFilter('type', 'open') })
  }
  if (f.project_id) {
    const project = projects.value.find(p => String(p.id) === String(f.project_id))
    chips.push({
      key: 'project',
      label: project?.name || String(f.project_id),
      clear: () => { f.project_id = ''; onProjectChange() },
    })
  }
  if (f.category) {
    chips.push({ key: 'category', label: f.category, clear: () => setFilter('category', '') })
  }
  if (f.search) {
    chips.push({ key: 'search', label: '«' + f.search + '»', clear: () => { searchInput.value = '' } })
  }
  return chips
})

function setFilter(key: string, value: string) {
  store.setFilter(key as any, value)
  store.fetchTasks()
}

function setGrouping(mode: string) {
  store.setGrouping(mode)
  store.fetchTasks()
}

function onProjectChange() {
  store.setFilter('category', '')
  store.fetchCategories(filters.value.project_id || undefined)
  store.fetchTasks()
}

function doSearch() {
  store.setFilter('search', searchInput.value)
  store.fetchTasks()
}

function resetAll() {
  searchInput.value = ''
  store.resetFilters()
}

watch(searchInput, (val) => {
  if (val === '' && filters.value.search) {
    store.setFilter('search', '')
    store.fetchTasks()
  }
})
</script>

<style scoped>
.task-filters {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 20px;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

/* Настройка колонок стоит у правого края строки */
.columns-gear {
  margin-left: auto;
  order: 1;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.filter-label {
  font-size: 11px;
  color: var(--text-muted);
  margin-right: 2px;
}

.pill-btn {
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 9999px;
  border: 1px solid var(--hairline);
  background: var(--surface-1);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

/* Не "sm": элементы с классами-брейкпоинтами Quasar скрывает на широких экранах */
.pill-btn.compact {
  padding: 4px 8px;
  font-size: 11px;
}

.pill-btn:hover {
  border-color: var(--accent);
  color: var(--text-bright);
}

.pill-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.field {
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  min-width: 200px;
  transition: border-color 0.1s;
}

/* Подсказка «Поиск по названию или номеру…» должна помещаться целиком */
.search-input {
  min-width: 240px;
}

.field:focus-visible {
  outline: none;
  border-color: var(--accent);
}

.filter-select {
  margin-left: 12px;
  cursor: pointer;
}

.reset-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  color: var(--text-muted);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}

.reset-btn:hover {
  border-color: var(--text-faint);
  color: var(--text-dim);
}

.found {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-faint);
}

.found strong {
  font-weight: 500;
  color: var(--text-muted);
}

/* Телефон */
.mobile-filters {
  margin-bottom: 12px;
}

.m-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  padding: 0 4px 0 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.m-search:focus-within {
  border-color: var(--accent);
}

.m-search-icon {
  flex: none;
  color: var(--text-muted);
}

.m-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  color: var(--text-bright);
  /* 16px — иначе iOS увеличивает страницу при фокусе */
  font-size: 16px;
  outline: none;
}

.m-filter-btn {
  position: relative;
  flex: none;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--text);
  cursor: pointer;
}

.m-filter-btn.active {
  background: var(--accent);
  color: #fff;
}

.m-badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 17px;
  height: 17px;
  padding: 0 4px;
  display: grid;
  place-items: center;
  border: 2px solid var(--bg-card);
  border-radius: 9px;
  background: var(--danger);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
}

.m-chips {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  overflow-x: auto;
  scrollbar-width: none;
}

.m-chips::-webkit-scrollbar {
  display: none;
}

.m-chip {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 0;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}

.m-panel {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  flex-direction: column;
  background: var(--bg);
}

.m-panel-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: calc(10px + env(safe-area-inset-top)) 12px 10px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border);
}

.m-panel-head b {
  flex: 1;
  font-size: 17px;
  color: var(--text-bright);
}

.m-back,
.m-reset {
  border: 0;
  background: none;
  cursor: pointer;
}

.m-back {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  color: var(--text);
}

.m-reset {
  padding: 8px;
  color: var(--accent);
  font-size: 14px;
  font-weight: 600;
}

.m-panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 16px 16px;
}

.m-label {
  margin: 18px 0 8px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.m-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.m-pills button {
  padding: 8px 13px;
  border: 1px solid var(--border);
  border-radius: 9999px;
  background: var(--bg-card);
  color: var(--text);
  font-size: 14px;
  cursor: pointer;
}

.m-pills button.active {
  border-color: var(--accent);
  background: var(--accent-bg);
  color: var(--accent);
  font-weight: 600;
}

.m-select {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-card);
  color: var(--text-bright);
  font-size: 16px;
}

.m-panel-foot {
  padding: 12px 16px calc(14px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--border);
  background: var(--bg-card);
}

.m-show {
  width: 100%;
  padding: 14px;
  border: 0;
  border-radius: 11px;
  background: var(--accent);
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
}

.m-panel-enter-active,
.m-panel-leave-active {
  transition: transform 0.28s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.m-panel-enter-from,
.m-panel-leave-to {
  transform: translateX(100%);
}

@media (max-width: 768px) {
  .filter-select {
    margin-left: 0;
  }
  .field {
    min-width: 0;
    width: 100%;
  }
  .found {
    margin-left: 0;
  }
}
</style>
