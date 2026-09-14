<template>
  <div class="task-filters">
    <!-- Row 1: Type buttons, Project, Search -->
    <div class="filter-row">
      <div class="filter-group">
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
      </div>

      <div class="filter-group">
        <select
          v-model="filters.project_id"
          @change="onProjectChange"
          class="filter-select"
          data-testid="filter-project"
        >
          <option value="">{{ $t('common.all') }}</option>
          <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </div>

      <div class="filter-group search-group">
        <input
          v-model="searchInput"
          type="text"
          :placeholder="$t('tasks.filters.search_placeholder')"
          class="search-input"
          data-testid="filter-search"
          @keyup.enter="doSearch"
        />
      </div>
    </div>

    <!-- Row 2: Grouping, Categories -->
    <div class="filter-row">
      <div class="filter-group">
        <button
          class="pill-btn"
          :class="{ active: useGrouping && filters.group_by === 'project' }"
          @click="setGrouping('project')"
          data-testid="filter-group-project"
        >
          {{ $t('tasks.filters.group_by_project') }}
        </button>
        <button
          class="pill-btn"
          :class="{ active: useGrouping && filters.group_by === 'assignee' }"
          @click="setGrouping('assignee')"
          data-testid="filter-group-assignee"
        >
          {{ $t('tasks.filters.group_by_assignee') }}
        </button>
        <button
          class="pill-btn"
          :class="{ active: !useGrouping }"
          @click="setGrouping('')"
          data-testid="filter-no-group"
        >
          {{ $t('common.all') }}
        </button>
      </div>

      <div class="filter-group" v-if="categories.length > 0">
        <button
          v-for="cat in categories"
          :key="cat"
          class="pill-btn"
          :class="{ active: filters.category === cat }"
          @click="setFilter('category', filters.category === cat ? '' : cat)"
          data-testid="'filter-cat-' + cat"
        >
          {{ cat }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useTasksStore } from '../../stores/tasks'

const store = useTasksStore()
const { filters, projects, categories, useGrouping } = storeToRefs(store)

const searchInput = ref(filters.value.search)

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
  if (mode === '') {
    useGrouping.value = false
  } else {
    useGrouping.value = true
    store.setFilter('group_by', mode)
  }
  store.fetchTasks()
}

function onProjectChange() {
  store.fetchTasks()
}

function doSearch() {
  store.setFilter('search', searchInput.value)
  store.fetchTasks()
}

watch(searchInput, (val) => {
  if (val === '') {
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

.filter-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.search-group {
  flex: 1;
  min-width: 200px;
}

.pill-btn {
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 9999px;
  background: transparent;
  border: 1px solid var(--hairline);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

.pill-btn:hover {
  border-color: var(--text-faint);
  color: var(--text-bright);
}

.pill-btn.active {
  background: var(--accent-bg);
  border-color: var(--accent);
  color: var(--accent);
}

.filter-select {
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  cursor: pointer;
  min-width: 150px;
}

.filter-select:focus {
  border-color: var(--accent);
}

.search-input {
  width: 100%;
  padding: 6px 12px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.search-input:focus {
  border-color: var(--accent);
}

@media (max-width: 768px) {
  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }
  .search-group {
    width: 100%;
  }
}
</style>
