<template>
  <!-- Просмотр картинки на весь экран: стрелки и смахивание листают картинки,
       крестик, Esc или нажатие на фон закрывают -->
  <Teleport to="body">
    <Transition name="viewer">
      <div
        v-if="current"
        class="viewer"
        role="dialog"
        :aria-label="current.alt || $t('common.image')"
        data-testid="image-viewer"
        @click.self="close"
        @pointerdown="onDown"
        @pointerup="onUp"
      >
        <img :src="current.src" :alt="current.alt" class="viewer-img" @click.stop />

        <div class="viewer-bar">
          <span v-if="images.length > 1" class="viewer-count">{{ (index ?? 0) + 1 }} / {{ images.length }}</span>
          <span class="viewer-name">{{ current.alt }}</span>
          <a :href="current.src" target="_blank" rel="noopener" class="viewer-btn" :title="$t('common.open_original')" @click.stop>
            <ExternalLink :size="18" />
          </a>
          <button type="button" class="viewer-btn" :aria-label="$t('common.close')" data-testid="image-viewer-close" @click.stop="close">
            <X :size="20" />
          </button>
        </div>

        <template v-if="images.length > 1">
          <button type="button" class="viewer-nav prev" :aria-label="$t('common.previous')" @click.stop="step(-1)">
            <ChevronLeft :size="28" />
          </button>
          <button type="button" class="viewer-nav next" :aria-label="$t('common.next')" @click.stop="step(1)">
            <ChevronRight :size="28" />
          </button>
        </template>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { X, ChevronLeft, ChevronRight, ExternalLink } from 'lucide-vue-next'

export interface ViewerImage {
  src: string
  alt: string
}

const props = defineProps<{ images: ViewerImage[]; index: number | null }>()
const emit = defineEmits<{ 'update:index': [value: number | null] }>()

const current = computed(() => (props.index === null ? null : props.images[props.index] || null))

function close() {
  emit('update:index', null)
}

// Листание по кругу
function step(delta: number) {
  if (props.index === null || props.images.length < 2) return
  emit('update:index', (props.index + delta + props.images.length) % props.images.length)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
  else if (e.key === 'ArrowLeft') step(-1)
  else if (e.key === 'ArrowRight') step(1)
  else return
  // Esc не должен закрыть и карточку задачи под просмотром
  e.stopPropagation()
  e.preventDefault()
}

// Смахивание вбок — следующая или предыдущая картинка
let x0 = 0
let y0 = 0
function onDown(e: PointerEvent) {
  x0 = e.clientX
  y0 = e.clientY
}
function onUp(e: PointerEvent) {
  const dx = e.clientX - x0
  if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(e.clientY - y0)) step(dx < 0 ? 1 : -1)
}

watch(current, open => {
  if (open) window.addEventListener('keydown', onKey, true)
  else window.removeEventListener('keydown', onKey, true)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKey, true))
</script>

<style scoped>
.viewer {
  position: fixed;
  inset: 0;
  z-index: calc(var(--z-modal, 10000) + 10);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: calc(56px + env(safe-area-inset-top)) 16px calc(24px + env(safe-area-inset-bottom));
  background: rgba(0, 0, 0, 0.92);
  /* Картинку можно увеличить двумя пальцами */
  touch-action: pinch-zoom;
}

.viewer-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 6px;
  user-select: none;
  -webkit-user-drag: none;
}

.viewer-bar {
  position: absolute;
  top: env(safe-area-inset-top);
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px 10px 16px;
  color: #fff;
}

.viewer-count {
  flex: none;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  opacity: 0.8;
}

.viewer-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
  opacity: 0.8;
}

.viewer-btn,
.viewer-nav {
  display: grid;
  place-items: center;
  border: 0;
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  cursor: pointer;
}

.viewer-btn {
  width: 40px;
  height: 40px;
  flex: none;
  border-radius: 50%;
}

.viewer-btn:hover,
.viewer-nav:hover {
  background: rgba(255, 255, 255, 0.22);
}

.viewer-nav {
  position: absolute;
  top: 50%;
  width: 48px;
  height: 48px;
  margin-top: -24px;
  border-radius: 50%;
}

.viewer-nav.prev { left: 16px; }
.viewer-nav.next { right: 16px; }

/* На телефоне листают смахиванием — стрелки не загораживают картинку */
@media (max-width: 768px) {
  .viewer-nav {
    display: none;
  }
}

.viewer-enter-active,
.viewer-leave-active {
  transition: opacity 0.2s;
}

.viewer-enter-from,
.viewer-leave-to {
  opacity: 0;
}
</style>
