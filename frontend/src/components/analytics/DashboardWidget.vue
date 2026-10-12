<template>
  <section
    ref="root"
    class="dash-widget"
    :class="[dropSide ? 'drop-' + dropSide : '', { dragging, collapsed }]"
    :style="{ gridColumn: movable ? 'span ' + cols : '1 / -1' }"
    :data-testid="'widget-' + id"
    @dragover="$emit('over', $event)"
    @drop="$emit('drop-on', $event)"
  >
    <header class="dash-head">
      <span
        v-if="movable"
        class="dash-handle"
        draggable="true"
        :title="$t('analytics.dashboard.drag')"
        data-testid="widget-handle"
        @dragstart="onDragStart"
        @dragend="$emit('drag-end')"
      >
        <Menu :size="16" />
      </span>
      <h3 class="dash-title">{{ title }}</h3>

      <div class="dash-actions">
        <slot name="actions" />
        <select
          v-if="movable"
          class="dash-size"
          :value="cols"
          :title="$t('analytics.dashboard.width')"
          data-testid="widget-size"
          @change="$emit('resize', Number(($event.target as HTMLSelectElement).value))"
        >
          <option v-for="w in WIDGET_WIDTHS" :key="w" :value="w">{{ $t('analytics.dashboard.widths.w' + w) }}</option>
        </select>
        <button
          type="button"
          class="dash-btn"
          :title="$t(collapsed ? 'analytics.dashboard.expand' : 'analytics.dashboard.collapse')"
          data-testid="widget-collapse"
          @click="$emit('collapse', !collapsed)"
        >
          <ChevronDown v-if="collapsed" :size="16" />
          <ChevronUp v-else :size="16" />
        </button>
        <button
          type="button"
          class="dash-btn"
          :title="$t('analytics.dashboard.hide')"
          data-testid="widget-hide"
          @click="$emit('hide')"
        >
          <X :size="16" />
        </button>
      </div>
    </header>

    <div v-if="!collapsed" class="dash-body">
      <slot />
    </div>

    <span
      v-if="movable && !collapsed"
      class="dash-resize"
      :title="$t('analytics.dashboard.resize')"
      data-testid="widget-resize"
      @pointerdown="onResizeStart"
      @pointermove="onResizeMove"
      @pointerup="onResizeEnd"
      @pointercancel="onResizeEnd"
    ></span>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Menu, ChevronDown, ChevronUp, X } from 'lucide-vue-next'
import { WIDGET_WIDTHS, nearestWidth } from '../../utils/dashboardLayout'

// movable: the widget can be dragged and resized (not on a narrow screen,
// where every widget takes the whole width). dropSide: where the dragged
// widget would land relative to this one.
const props = defineProps<{
  id: string
  title: string
  cols: number
  collapsed: boolean
  movable: boolean
  dragging: boolean
  dropSide: 'left' | 'right' | 'top' | 'bottom' | null
}>()

const emit = defineEmits<{
  (e: 'resize', cols: number): void
  (e: 'collapse', collapsed: boolean): void
  (e: 'hide'): void
  (e: 'drag-start'): void
  (e: 'drag-end'): void
  (e: 'over', event: DragEvent): void
  (e: 'drop-on', event: DragEvent): void
}>()

const root = ref<HTMLElement | null>(null)

// The handle is what is grabbed, the whole widget is what is seen moving
function onDragStart(e: DragEvent) {
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', props.id)
    if (root.value) e.dataTransfer.setDragImage(root.value, 16, 16)
  }
  emit('drag-start')
}

const resizing = ref(false)

function onResizeStart(e: PointerEvent) {
  resizing.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  e.preventDefault()
}

// The width follows the pointer by the steps of the grid
function onResizeMove(e: PointerEvent) {
  const grid = root.value?.parentElement
  if (!resizing.value || !root.value || !grid || grid.clientWidth === 0) return
  const width = e.clientX - root.value.getBoundingClientRect().left
  const cols = nearestWidth((width / grid.clientWidth) * 12)
  if (cols !== props.cols) emit('resize', cols)
}

function onResizeEnd(e: PointerEvent) {
  if (!resizing.value) return
  resizing.value = false
  ;(e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId)
}
</script>

<style scoped>
.dash-widget {
  position: relative;
  min-width: 0;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  background: var(--bg);
}

.dash-widget.dragging {
  opacity: 0.4;
}

/* Where the dragged widget lands */
.dash-widget.drop-left { box-shadow: -4px 0 0 var(--accent); }
.dash-widget.drop-right { box-shadow: 4px 0 0 var(--accent); }
.dash-widget.drop-top { box-shadow: 0 -4px 0 var(--accent); }
.dash-widget.drop-bottom { box-shadow: 0 4px 0 var(--accent); }

.dash-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--hairline);
}

.dash-widget.collapsed .dash-head {
  border-bottom: none;
}

.dash-handle {
  display: flex;
  flex-shrink: 0;
  color: var(--text-faint);
  cursor: grab;
}

.dash-handle:hover {
  color: var(--text-bright);
}

.dash-title {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dash-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.dash-size {
  height: 28px;
  padding: 0 6px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
}

.dash-btn {
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

.dash-size:hover,
.dash-btn:hover {
  border-color: var(--accent);
  color: var(--text-bright);
}

/* The content adapts to the width of the widget, not of the window */
.dash-body {
  padding: 16px;
  container-type: inline-size;
}

.dash-resize {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 16px;
  height: 16px;
  border-right: 2px solid var(--text-faint);
  border-bottom: 2px solid var(--text-faint);
  border-bottom-right-radius: 12px;
  cursor: nwse-resize;
  touch-action: none;
  opacity: 0.5;
}

.dash-resize:hover {
  border-color: var(--accent);
  opacity: 1;
}
</style>
