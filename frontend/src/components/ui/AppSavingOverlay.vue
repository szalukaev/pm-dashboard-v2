<template>
  <!-- Закрывает форму на время сохранения. Обычный режим — обёртка вокруг
       формы; cover — только сама заслонка, родитель должен быть position: relative -->
  <div v-if="!cover || active" :class="cover ? 'saving-cover' : 'saving-wrap'">
    <slot v-if="!cover" />
    <div v-if="active" class="saving-overlay" role="status" aria-live="polite" data-testid="saving-overlay">
      <AppSpinner :size="26" />
      <p class="saving-text">{{ text || $t('common.saving') }}</p>
      <div v-if="progress" class="saving-bar"><span></span></div>
      <p v-if="hint || long" class="saving-hint">{{ hint || $t('common.saving_long') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import AppSpinner from './AppSpinner.vue'

// Через сколько сохранение считается долгим и появляется просьба подождать
const LONG_AFTER_MS = 5000

// text: надпись вместо «Сохранение…»; hint: пояснение, видимое сразу;
// progress: бегущая полоса для долгих действий; guard: не даёт закрыть или
// обновить страницу, пока заслонка активна
const props = defineProps<{
  active: boolean
  cover?: boolean
  text?: string
  hint?: string
  progress?: boolean
  guard?: boolean
}>()

const long = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

function warnBeforeLeave(e: BeforeUnloadEvent) {
  e.preventDefault()
  e.returnValue = ''
}

function stop() {
  if (timer) clearTimeout(timer)
  timer = null
  long.value = false
  window.removeEventListener('beforeunload', warnBeforeLeave)
}

watch(() => props.active, (active) => {
  stop()
  if (!active) return
  timer = setTimeout(() => { long.value = true }, LONG_AFTER_MS)
  if (props.guard) window.addEventListener('beforeunload', warnBeforeLeave)
}, { immediate: true })

onBeforeUnmount(stop)
</script>

<style scoped>
.saving-wrap {
  position: relative;
}

.saving-cover {
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: inherit;
}

.saving-overlay {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 16px;
  text-align: center;
  background: color-mix(in srgb, var(--surface-1) 78%, transparent);
  border-radius: inherit;
  cursor: progress;
}

.saving-text {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.saving-hint {
  max-width: 420px;
  font-size: 12px;
  color: var(--text-muted);
}

/* Полоса без процентов: сколько займёт действие, заранее неизвестно */
.saving-bar {
  width: 240px;
  max-width: 80%;
  height: 4px;
  overflow: hidden;
  background: var(--hairline);
  border-radius: 9999px;
}

.saving-bar span {
  display: block;
  width: 40%;
  height: 100%;
  background: var(--accent);
  border-radius: 9999px;
  animation: saving-slide 1.4s ease-in-out infinite;
}

@keyframes saving-slide {
  from { transform: translateX(-100%); }
  to { transform: translateX(250%); }
}
</style>
