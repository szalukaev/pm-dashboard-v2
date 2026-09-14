<template>
  <AppModal
    :model-value="visible"
    :title="$t('payments.invoice.pay_form_title')"
    width="360px"
    @update:model-value="!$event && $emit('close')"
  >
    <div class="pay-form">
      <div class="form-field">
        <label>{{ $t('payments.invoice.amount_label') }}</label>
        <input v-model.number="amount" type="number" min="0" step="0.01" class="form-input" />
      </div>
      <div class="form-field">
        <label>{{ $t('payments.invoice.pay_date_label') }}</label>
        <input v-model="paidAt" type="date" class="form-input" />
      </div>
    </div>

    <template #footer>
      <AppButton variant="ghost" @click="$emit('close')">{{ $t('common.cancel') }}</AppButton>
      <AppButton variant="primary" @click="submit">{{ $t('common.confirm') }}</AppButton>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import AppModal from '../ui/AppModal.vue'
import AppButton from '../ui/AppButton.vue'

const props = defineProps<{
  visible: boolean
  defaultAmount: number
}>()

const emit = defineEmits(['close', 'submit'])

const amount = ref(0)
const paidAt = ref(new Date().toISOString().slice(0, 10))

watch(() => props.visible, (v) => {
  if (v) {
    amount.value = props.defaultAmount
    paidAt.value = new Date().toISOString().slice(0, 10)
  }
})

function submit() {
  emit('submit', { amount: amount.value, paid_at: paidAt.value })
}
</script>

<style scoped>
.pay-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-input:focus {
  border-color: var(--accent);
}
</style>
