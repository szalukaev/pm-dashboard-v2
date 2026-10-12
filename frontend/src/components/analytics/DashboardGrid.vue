<template>
  <div class="dashboard">
    <div class="dash-toolbar">
      <div class="dash-toolbar-title"><slot name="title" /></div>

      <div v-if="addable.length > 0" ref="addRoot" class="dash-add">
        <button type="button" class="dash-tool" data-testid="dashboard-add" @click="addOpen = !addOpen">
          <Plus :size="14" />
          {{ $t('analytics.dashboard.add_widget') }}
        </button>
        <ul v-if="addOpen" class="dash-add-list" data-testid="dashboard-add-list">
          <li v-for="def in addable" :key="def.id">
            <button type="button" class="dash-add-item" @click="show(def.id)">{{ $t(def.title) }}</button>
          </li>
        </ul>
      </div>
      <button type="button" class="dash-tool" data-testid="dashboard-reset" @click="reset">
        <RotateCcw :size="14" />
        {{ $t('analytics.dashboard.reset') }}
      </button>
    </div>

    <div v-if="shown.length === 0" class="dash-empty">{{ $t('analytics.dashboard.empty') }}</div>

    <div class="dash-grid">
      <DashboardWidget
        v-for="w in shown"
        :id="w.id"
        :key="w.id"
        :title="$t(titles[w.id])"
        :hint="hints[w.id] ? $t(hints[w.id]) : undefined"
        :cols="w.cols"
        :collapsed="w.collapsed === true"
        :movable="!narrow"
        :dragging="dragId === w.id"
        :drop-side="over && over.id === w.id ? over.side : null"
        @resize="update(w.id, { cols: $event })"
        @collapse="update(w.id, { collapsed: $event })"
        @hide="hide(w.id)"
        @drag-start="dragId = w.id"
        @drag-end="dragId = over = null"
        @over="onOver($event, w)"
        @drop-on="onDrop($event)"
      >
        <template #actions><slot :name="w.id + '-actions'" /></template>
        <slot :name="w.id" />
      </DashboardWidget>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Plus, RotateCcw } from 'lucide-vue-next'
import { useSettingsStore } from '../../stores/settings'
import { placedWidgets, hiddenWidgets } from '../../utils/dashboardLayout'
import type { WidgetDef, PlacedWidget } from '../../utils/dashboardLayout'
import DashboardWidget from './DashboardWidget.vue'

// widgets: every widget of the tab in the standard order; available: the ones
// the user may see. A widget the administrator hid is not shown and cannot be
// added, whatever the user's own layout says; its place in the layout is kept
// for the day it is allowed again.
// Slots: <id> — the content of a widget, <id>-actions — its extra buttons in
// the header, title — the heading of the page.
const props = defineProps<{
  widgets: WidgetDef[]
  available: string[]
}>()

defineSlots<Record<string, () => any>>()

type Side = 'left' | 'right' | 'top' | 'bottom'

// Below this width every widget takes the whole row and stays where it is
const NARROW_QUERY = '(max-width: 767px)'

const settingsStore = useSettingsStore()

const saved = computed(() => settingsStore.settings.dashboard_layout)
const placed = computed(() => placedWidgets(saved.value, props.widgets))
const hidden = computed(() => hiddenWidgets(saved.value, props.widgets))

const shown = computed(() => placed.value.filter(w => props.available.includes(w.id)))
const addable = computed(() => props.widgets.filter(d => hidden.value.includes(d.id) && props.available.includes(d.id)))
const titles = computed(() => Object.fromEntries(props.widgets.map(d => [d.id, d.title])) as Record<string, string>)
const hints = computed(() => Object.fromEntries(props.widgets.map(d => [d.id, d.hint || ''])) as Record<string, string>)

function save(widgets: PlacedWidget[], hiddenIds: string[]) {
  settingsStore.setDashboardLayout({ widgets, hidden_widgets: hiddenIds })
}

function update(id: string, change: Partial<PlacedWidget>) {
  save(placed.value.map(w => (w.id === id ? { ...w, ...change } : w)), hidden.value)
}

function hide(id: string) {
  save(placed.value.filter(w => w.id !== id), [...hidden.value, id])
}

// A returned widget goes back to its standard place
function show(id: string) {
  addOpen.value = false
  const layout = { widgets: placed.value, hidden_widgets: hidden.value.filter(h => h !== id) }
  save(placedWidgets(layout, props.widgets), layout.hidden_widgets)
}

function reset() {
  addOpen.value = false
  settingsStore.setDashboardLayout({})
}

// Reordering by dragging a widget by its handle
const dragId = ref<string | null>(null)
const over = ref<{ id: string; side: Side } | null>(null)

function onOver(e: DragEvent, target: PlacedWidget) {
  // Something else is dragged inside a widget, e.g. a column of its table
  if (dragId.value === null) return
  e.preventDefault()
  if (target.id === dragId.value) {
    over.value = null
    return
  }
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  // A widget of the whole row has neighbours only above and below
  const side: Side = target.cols === 12
    ? (e.clientY < rect.top + rect.height / 2 ? 'top' : 'bottom')
    : (e.clientX < rect.left + rect.width / 2 ? 'left' : 'right')
  const current = over.value
  if (!current || current.id !== target.id || current.side !== side) over.value = { id: target.id, side }
}

function onDrop(e: DragEvent) {
  const from = dragId.value
  const to = over.value
  dragId.value = over.value = null
  if (from === null) return
  e.preventDefault()
  if (!to || to.id === from) return

  const moved = placed.value.find(w => w.id === from)
  if (!moved) return
  const rest = placed.value.filter(w => w.id !== from)
  const index = rest.findIndex(w => w.id === to.id)
  if (index === -1) return
  rest.splice(to.side === 'left' || to.side === 'top' ? index : index + 1, 0, moved)
  save(rest, hidden.value)
}

const addOpen = ref(false)
const addRoot = ref<HTMLElement | null>(null)

function onDocumentClick(e: MouseEvent) {
  if (addOpen.value && addRoot.value && !addRoot.value.contains(e.target as Node)) addOpen.value = false
}

const narrow = ref(false)
let narrowQuery: MediaQueryList | null = null

function onNarrowChange() {
  narrow.value = narrowQuery?.matches === true
}

onMounted(() => {
  settingsStore.ensureLoaded()
  document.addEventListener('click', onDocumentClick)
  narrowQuery = window.matchMedia(NARROW_QUERY)
  narrowQuery.addEventListener('change', onNarrowChange)
  onNarrowChange()
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick)
  narrowQuery?.removeEventListener('change', onNarrowChange)
})
</script>

<style scoped>
.dash-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 24px;
}

.dash-toolbar-title {
  flex: 1;
  min-width: 0;
}

.dash-tool {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}

.dash-tool:hover {
  border-color: var(--accent);
  color: var(--text-bright);
}

.dash-add {
  position: relative;
}

.dash-add-list {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: var(--z-dropdown);
  min-width: 240px;
  margin: 0;
  padding: 6px;
  list-style: none;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  box-shadow: var(--shadow);
}

.dash-add-item {
  width: 100%;
  padding: 7px 10px;
  border: none;
  border-radius: 8px;
  background: none;
  font-size: 13px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.dash-add-item:hover {
  background: var(--bg-hover);
}

.dash-empty {
  padding: 48px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}

.dash-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 16px;
  align-items: start;
}
</style>
