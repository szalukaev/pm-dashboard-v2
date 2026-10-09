<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="modal-overlay" @click.self="close">
        <div class="modal-content" :class="{ fullscreen }" :style="fullscreen ? undefined : { width: width || '520px' }">
          <div class="modal-header">
            <h3><slot name="title">{{ title }}</slot></h3>
            <button
              v-if="expandable"
              class="modal-close modal-expand"
              :title="$t(fullscreen ? 'common.windowed' : 'common.fullscreen')"
              data-testid="modal-expand"
              @click="fullscreen = !fullscreen"
            >
              <Minimize2 v-if="fullscreen" :size="16" />
              <Maximize2 v-else :size="16" />
            </button>
            <button class="modal-close" data-testid="modal-close" @click="close">
              <X :size="18" />
            </button>
          </div>
          <div class="modal-body">
            <slot />
          </div>
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { X, Maximize2, Minimize2 } from 'lucide-vue-next'

// expandable: the window can be switched to the full screen and back; the
// choice is kept while the page is open.
defineProps<{
  modelValue: boolean
  title: string
  width?: string
  expandable?: boolean
}>()

const fullscreen = ref(false)

const emit = defineEmits(['update:modelValue', 'close'])

function close() {
  emit('update:modelValue', false)
  emit('close')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--modal-overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal);
}

.modal-content {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  box-shadow: var(--shadow);
  max-height: 85vh;
  max-width: calc(100vw - 32px);
  display: flex;
  flex-direction: column;
}

.modal-content.fullscreen {
  width: 100vw;
  height: 100vh;
  max-width: none;
  max-height: none;
  border: none;
  border-radius: 0;
}

.modal-header h3 {
  flex: 1;
  min-width: 0;
}

.modal-expand {
  margin-left: 8px;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border-light);
}

.modal-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.2px;
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.modal-close:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px 20px;
  border-top: 1px solid var(--border-light);
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.15s;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
