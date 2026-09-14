<template>
  <div class="empty-state" data-testid="empty-state">
    <component :is="iconComponent" :size="iconSize" class="empty-icon" />
    <p class="empty-text">{{ message }}</p>
    <AppButton v-if="showRetry" variant="ghost" @click="$emit('retry')" data-testid="retry-button">
      {{ $t('common.retry') }}
    </AppButton>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Inbox, AlertCircle, Loader2 } from 'lucide-vue-next'
import AppButton from './AppButton.vue'

const props = withDefaults(defineProps<{
  state: 'empty' | 'loading' | 'error'
  iconSize?: number
}>(), {
  iconSize: 40
})

defineEmits(['retry'])

const iconComponent = computed(() => {
  switch (props.state) {
    case 'empty': return Inbox
    case 'loading': return Loader2
    case 'error': return AlertCircle
  }
})

const message = computed(() => {
  switch (props.state) {
    case 'empty': return 'Пока ничего нет'
    case 'loading': return 'Загрузка…'
    case 'error': return 'Не удалось загрузить данные'
  }
})

const showRetry = computed(() => props.state === 'error')
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  gap: 12px;
}

.empty-icon {
  color: var(--text-faint);
}

.empty-text {
  font-size: 13px;
  color: var(--text-muted);
  text-align: center;
}
</style>
