<template>
  <div ref="root" class="column-settings">
    <button
      type="button"
      class="gear-btn"
      :class="{ active: open }"
      :title="$t('common.columns.title')"
      data-testid="column-settings"
      @click="open = !open"
    >
      <SlidersHorizontal :size="16" />
    </button>

    <div v-if="open" class="panel" data-testid="column-settings-panel">
      <div class="panel-title">{{ $t('common.columns.title') }}</div>
      <ul class="column-list">
        <li
          v-for="(col, index) in ordered"
          :key="col.key"
          class="column-row"
          :class="{ dragging: dragIndex === index, 'drop-here': overIndex === index && dragIndex !== index }"
          draggable="true"
          @dragstart="onDragStart($event, index)"
          @dragover.prevent="overIndex = index"
          @drop.prevent="onDrop(index)"
          @dragend="dragIndex = overIndex = null"
        >
          <GripVertical :size="14" class="grip" />
          <label class="column-label">
            <input
              type="checkbox"
              :checked="visible.has(col.key)"
              :disabled="visible.has(col.key) && visible.size === 1"
              @change="toggle(col.key)"
            />
            {{ $t(col.label) }}
          </label>
        </li>
      </ul>
      <button type="button" class="reset-btn" data-testid="column-settings-reset" @click="reset">
        {{ $t('common.columns.reset') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { SlidersHorizontal, GripVertical } from 'lucide-vue-next'
import { useSettingsStore } from '../../stores/settings'

// table: the name the setup is saved under; columns: every column of the
// table in the standard order, label is a dictionary key.
const props = defineProps<{
  table: string
  columns: { key: string; label: string }[]
}>()

const settingsStore = useSettingsStore()
const open = ref(false)
const root = ref<HTMLElement | null>(null)

const allKeys = computed(() => props.columns.map(c => c.key))
const order = computed(() => settingsStore.columnOrder(props.table, allKeys.value))
const visible = computed(() => new Set(settingsStore.tableColumns(props.table, allKeys.value)))
const ordered = computed(() => order.value.map(key => props.columns.find(c => c.key === key)!))

// Every change is applied and saved at once
function save(nextOrder: string[], nextVisible: Set<string>) {
  settingsStore.setTableColumns(props.table, nextOrder.filter(k => nextVisible.has(k)), nextOrder)
}

function toggle(key: string) {
  const next = new Set(visible.value)
  if (next.has(key)) {
    // A table without columns is of no use: the last one stays
    if (next.size === 1) return
    next.delete(key)
  } else {
    next.add(key)
  }
  save(order.value, next)
}

const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

function onDragStart(e: DragEvent, index: number) {
  dragIndex.value = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}

function onDrop(index: number) {
  const from = dragIndex.value
  dragIndex.value = overIndex.value = null
  if (from === null || from === index) return
  const next = [...order.value]
  const [moved] = next.splice(from, 1)
  next.splice(index, 0, moved)
  save(next, visible.value)
}

function reset() {
  settingsStore.setTableColumns(props.table, allKeys.value, allKeys.value)
}

function onDocumentClick(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}

onMounted(() => {
  settingsStore.ensureLoaded()
  document.addEventListener('click', onDocumentClick)
})
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<style scoped>
.column-settings {
  position: relative;
  display: inline-flex;
}

.gear-btn {
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

.gear-btn:hover,
.gear-btn.active {
  border-color: var(--accent);
  color: var(--text-bright);
}

.panel {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: var(--z-dropdown);
  width: 240px;
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

.column-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 360px;
  overflow-y: auto;
}

.column-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 4px;
  border-radius: 8px;
  border-top: 2px solid transparent;
  font-size: 13px;
  color: var(--text);
}

.column-row:hover {
  background: var(--bg-hover);
}

.column-row.dragging {
  opacity: 0.4;
}

.column-row.drop-here {
  border-top-color: var(--accent);
}

.grip {
  flex-shrink: 0;
  color: var(--text-faint);
  cursor: grab;
}

.column-label {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  cursor: pointer;
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

.reset-btn:hover {
  border-color: var(--accent);
}
</style>
