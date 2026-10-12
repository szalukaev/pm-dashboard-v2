<template>
  <span
    ref="icon"
    class="info-hint"
    tabindex="0"
    data-testid="info-hint"
    @mouseenter="show"
    @mouseleave="shown = false"
    @focus="show"
    @blur="shown = false"
  >
    <Info :size="15" />
    <!-- Lives in the body so that no widget or window edge cuts it -->
    <Teleport to="body">
      <div v-if="shown" class="info-bubble" :style="{ left: x + 'px', top: y + 'px' }">{{ text }}</div>
    </Teleport>
  </span>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Info } from 'lucide-vue-next'

// text: what the bubble says; line breaks are kept
defineProps<{ text: string }>()

const BUBBLE_WIDTH = 360
const EDGE = 8

const icon = ref<HTMLElement | null>(null)
const shown = ref(false)
const x = ref(0)
const y = ref(0)

// Under the icon, moved left when it would not fit in the window
function show() {
  if (!icon.value) return
  const rect = icon.value.getBoundingClientRect()
  const width = Math.min(BUBBLE_WIDTH, window.innerWidth - 2 * EDGE)
  x.value = Math.max(EDGE, Math.min(rect.left, window.innerWidth - width - EDGE))
  y.value = rect.bottom + 6
  shown.value = true
}
</script>

<style scoped>
.info-hint {
  display: inline-flex;
  flex-shrink: 0;
  color: var(--text-faint);
  cursor: help;
  outline: none;
}

.info-hint:hover,
.info-hint:focus {
  color: var(--accent);
}

.info-bubble {
  position: fixed;
  z-index: var(--z-toast);
  width: 360px;
  max-width: calc(100vw - 16px);
  box-sizing: border-box;
  padding: 10px 12px;
  background: var(--surface-3);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  box-shadow: var(--shadow);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0;
  color: var(--text-dim);
  white-space: pre-line;
  pointer-events: none;
}
</style>
