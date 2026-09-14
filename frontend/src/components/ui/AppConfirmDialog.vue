<template>
  <AppModal
    :model-value="modelValue"
    :title="title"
    width="400px"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <p class="confirm-text">{{ message || $t('common.confirm_delete_text') }}</p>
    <template #footer>
      <AppButton variant="ghost" @click="$emit('update:modelValue', false)">
        {{ $t('common.cancel') }}
      </AppButton>
      <AppButton variant="danger" @click="$emit('confirm')">
        {{ $t('common.delete') }}
      </AppButton>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import AppModal from './AppModal.vue'
import AppButton from './AppButton.vue'

defineProps<{
  modelValue: boolean
  title: string
  message?: string
}>()

defineEmits(['update:modelValue', 'confirm'])
</script>

<style scoped>
.confirm-text {
  font-size: 13px;
  color: var(--text-dim);
  line-height: 1.5;
}
</style>
