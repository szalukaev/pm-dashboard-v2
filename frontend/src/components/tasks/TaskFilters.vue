<template>
  <div class="task-filters">
    <!-- Row 1: status, project, search, reset, counter -->
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

    <!-- Row 2: grouping, categories -->
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
import { ref, watch, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { RotateCcw } from 'lucide-vue-next'
import { useTasksStore } from '../../stores/tasks'
import { useSettingsStore } from '../../stores/settings'

const store = useTasksStore()
const settingsStore = useSettingsStore()
const { filters, projects, categories, useGrouping, total } = storeToRefs(store)
const { settings } = storeToRefs(settingsStore)

// Show only projects selected in user settings (if any selected)
const visibleProjects = computed(() => {
  const selected = settings.value.selected_projects || []
  if (selected.length === 0) return projects.value
  return projects.value.filter(p => selected.includes(p.id))
})

const searchInput = ref(filters.value.search)
// Saved filters are restored after this component is created.
watch(() => filters.value.search, (val) => { searchInput.value = val })

// Load settings to get selected_projects for filtering
settingsStore.fetchSettings()

const typeOptions = [
  { value: 'open', label: 'tasks.filters.open' },
  { value: 'testing', label: 'tasks.filters.testing' },
  { value: 'closed', label: 'tasks.filters.closed' },
  { value: 'all', label: 'tasks.filters.all' },
]

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

/* Not "sm": Quasar hides elements with breakpoint class names on wide screens */
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

.field:focus {
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
