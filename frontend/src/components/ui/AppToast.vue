<template>
  <Teleport to="body">
    <div
      class="toast-stack"
      :class="{ open }"
      :style="{ height: open ? `${stackHeight}px` : undefined }"
      @mouseenter="expand"
      @mouseleave="collapse"
    >
      <TransitionGroup name="toast">
        <div
          v-for="(toast, i) in toasts"
          :key="toast.id"
          :ref="el => setRef(toast.id, el as HTMLElement | null)"
          class="toast"
          :class="toast.type"
          :style="itemStyle(i, toast.id)"
          role="status"
          @pointerdown="startDrag($event, toast.id)"
          @pointermove="moveDrag"
          @pointerup="endDrag"
          @pointercancel="cancelDrag"
        >
          <CheckCircle2 v-if="toast.type === 'success'" class="toast-icon" :size="16" />
          <AlertCircle v-else-if="toast.type === 'error'" class="toast-icon" :size="16" />
          <Info v-else class="toast-icon" :size="16" />

          <div class="toast-body">
            <template v-if="toast.change">
              <div v-if="toast.change.field || toast.change.taskId" class="toast-header">
                <b v-if="toast.change.field">{{ toast.change.field }}</b>
                <template v-if="toast.change.field && toast.change.taskId"> · </template>
                <template v-if="toast.change.taskId">#{{ toast.change.taskId }}</template>
              </div>
              <div class="toast-change">
                <span class="toast-old">{{ toast.change.oldValue }}</span>
                <span class="toast-arrow">→</span>
                <span class="toast-new">{{ toast.change.newValue }}</span>
              </div>
            </template>
            <template v-else>{{ toast.message }}</template>
            <button v-if="toast.action" type="button" class="toast-action" data-testid="toast-action" @click="toast.action.run()">
              {{ toast.action.label }}
            </button>
          </div>

          <button class="toast-close" :aria-label="t('common.close')" @click="dismiss(toast.id)">
            <X :size="13" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle2, AlertCircle, Info, X } from 'lucide-vue-next'
import { useToast } from '../../composables/useToast'

const { t } = useI18n()
const { toasts, dismiss, pauseAll, resumeAll } = useToast()

// Сколько карточек видно в свёрнутой стопке
const VISIBLE_COLLAPSED = 3
const GAP = 8

const open = ref(false)
const offsets = ref<number[]>([])
const stackHeight = ref(0)
const elements = new Map<number, HTMLElement>()

function setRef(id: number, el: HTMLElement | null) {
  if (el) elements.set(id, el)
  else elements.delete(id)
}

// Раскрытая стопка: карточки идут столбиком, у каждой своя высота
function measure() {
  let y = 0
  offsets.value = toasts.value.map(toast => {
    const top = y
    y += (elements.get(toast.id)?.offsetHeight ?? 0) + GAP
    return top
  })
  stackHeight.value = y
}

function itemStyle(i: number, id: number) {
  const z = { zIndex: toasts.value.length - i }
  // Тост, который тянут вверх, едет за пальцем и становится прозрачнее
  if (drag.id === id && drag.dy < 0) {
    const base = open.value ? offsets.value[i] ?? 0 : i * 9
    return { ...z, transform: `translateY(${base + drag.dy}px)`, opacity: Math.max(0, 1 + drag.dy / 120), transition: 'none' }
  }
  if (open.value) return { ...z, transform: `translateY(${offsets.value[i] ?? 0}px)` }
  // Свёрнутая: каждая следующая чуть ниже и мельче, лишние скрыты
  return {
    ...z,
    transform: `translateY(${i * 9}px) scale(${1 - i * 0.05})`,
    opacity: i < VISIBLE_COLLAPSED ? 1 : 0,
  }
}

// Смахивание вверх: больше 40 px или резкий рывок — тост закрыт, иначе он
// возвращается на место. Пока тост тянут, таймеры стоят
const SWIPE_DISTANCE = 40
const SWIPE_SPEED = 0.5 // px/мс
const drag = reactive({ id: -1, y0: 0, dy: 0, t0: 0 })

function startDrag(e: PointerEvent, id: number) {
  // Кнопки внутри тоста нажимаются как обычно
  if ((e.target as HTMLElement).closest('button')) return
  drag.id = id
  drag.y0 = e.clientY
  drag.dy = 0
  drag.t0 = Date.now()
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  pauseAll()
}

function moveDrag(e: PointerEvent) {
  if (drag.id < 0) return
  drag.dy = Math.min(0, e.clientY - drag.y0)
}

function endDrag() {
  if (drag.id < 0) return
  const speed = -drag.dy / Math.max(1, Date.now() - drag.t0)
  const id = drag.id
  const swiped = drag.dy < -SWIPE_DISTANCE || speed > SWIPE_SPEED
  cancelDrag()
  if (swiped) dismiss(id)
}

function cancelDrag() {
  drag.id = -1
  drag.dy = 0
  if (!open.value) resumeAll()
}

function expand() {
  measure()
  open.value = true
  pauseAll()
}

function collapse() {
  open.value = false
  resumeAll()
}

watch(() => toasts.value.length, async () => {
  if (toasts.value.length === 0) open.value = false
  if (open.value) {
    await nextTick()
    measure()
  }
})
</script>

<style scoped>
.toast-action {
  display: block;
  margin-top: 8px;
  padding: 6px 12px;
  border: 0;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.toast {
  /* Тост тянут пальцем вверх — страница при этом не прокручивается */
  touch-action: none;
  user-select: none;
}

.toast-stack {
  position: fixed;
  top: 16px;
  right: 16px;
  width: 360px;
  max-width: calc(100vw - 32px);
  z-index: var(--z-toast);
}

/* Телефон: стопка привязана к обоим краям видимой области и не уходит
   за экран, даже если страница шире экрана; сверху — отступ под вырез */
@media (max-width: 768px) {
  .toast-stack {
    top: calc(8px + env(safe-area-inset-top));
    left: 8px;
    right: 8px;
    width: auto;
    max-width: none;
  }
}

.toast {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  box-shadow: var(--shadow);
  color: var(--text-bright);
  font-size: 13px;
  line-height: 1.4;
  transform-origin: top center;
  transition: transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1), opacity 0.3s;
}

.toast-icon {
  flex: none;
  margin-top: 1px;
}

.success .toast-icon { color: var(--success); }
.error .toast-icon { color: var(--danger); }
.info .toast-icon { color: var(--accent); }

.toast-body {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.toast-header {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 3px;
}

.toast-header b {
  color: var(--text);
  font-weight: 500;
}

.toast-old {
  color: var(--text-muted);
  text-decoration: line-through;
}

.toast-arrow {
  color: var(--text-muted);
  margin: 0 6px;
}

.toast-new {
  color: var(--success);
  font-weight: 600;
}

.toast-close {
  flex: none;
  display: flex;
  padding: 2px;
  border: 0;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
}

.toast:hover .toast-close,
.toast-close:focus-visible {
  opacity: 1;
}

.toast-close:hover {
  color: var(--text-bright);
}

.toast-enter-from {
  opacity: 0 !important;
  transform: translateY(-10px) scale(0.98) !important;
}

.toast-leave-to {
  opacity: 0 !important;
  transform: translateX(24px) !important;
}

@media (prefers-reduced-motion: reduce) {
  .toast { transition: opacity 0.2s; }
}
</style>
