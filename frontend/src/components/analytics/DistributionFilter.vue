<template>
  <div ref="root" class="dist-filter">
    <button
      type="button"
      class="filter-btn"
      :class="{ active: open || hidden.length > 0 }"
      :title="$t('analytics.distribution_filter.title')"
      data-testid="distribution-filter"
      @click="open = !open"
    >
      <Filter :size="16" />
    </button>

    <div v-if="open" class="panel" data-testid="distribution-filter-panel">
      <div class="panel-title">{{ $t('analytics.distribution_filter.title') }}</div>
      <div v-if="projects.length === 0" class="panel-empty">{{ $t('common.empty') }}</div>
      <ul class="project-list">
        <li v-for="p in projects" :key="p.id">
          <label class="project-row">
            <input type="checkbox" :checked="!hidden.includes(p.id)" @change="toggle(p.id)" />
            <span class="project-name" :title="p.name">{{ p.name }}</span>
          </label>
        </li>
      </ul>
      <button type="button" class="reset-btn" :disabled="hidden.length === 0" @click="$emit('update:hidden', [])">
        {{ $t('analytics.distribution_filter.show_all') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Filter } from 'lucide-vue-next'

// projects: the projects of the widget; hidden: ids of the ones the user
// switched off. A project that appears later is shown until switched off.
const props = defineProps<{
  projects: { id: number; name: string }[]
  hidden: number[]
}>()

const emit = defineEmits<{
  (e: 'update:hidden', ids: number[]): void
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

function toggle(id: number) {
  emit('update:hidden', props.hidden.includes(id) ? props.hidden.filter(h => h !== id) : [...props.hidden, id])
}

function onDocumentClick(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<style scoped>
.dist-filter {
  position: relative;
  display: inline-flex;
}

.filter-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--text-muted);
  cursor: pointer;
}

.filter-btn:hover,
.filter-btn.active {
  border-color: var(--accent);
  color: var(--text-bright);
}

.panel {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: var(--z-dropdown);
  width: 280px;
  padding: 12px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  box-shadow: var(--shadow);
}

.panel-title {
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.panel-empty {
  padding: 8px 0;
  font-size: 13px;
  color: var(--text-muted);
}

.project-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 320px;
  overflow-y: auto;
}

.project-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 4px;
  border-radius: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}

.project-row:hover {
  background: var(--bg-hover);
}

.project-name {
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.reset-btn {
  width: 100%;
  margin-top: 10px;
  padding: 6px 10px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
}

.reset-btn:hover:not(:disabled) {
  border-color: var(--accent);
}

.reset-btn:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
