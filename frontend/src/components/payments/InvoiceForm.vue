<template>
  <AppModal
    :model-value="visible"
    :title="$t('payments.invoice.form_title')"
    width="600px"
    @update:model-value="!$event && $emit('close')"
  >
    <div class="invoice-form">
      <table class="items-table">
        <thead>
          <tr>
            <th>{{ $t('payments.invoice.service_name') }}</th>
            <th class="num">{{ $t('payments.invoice.quantity') }}</th>
            <th class="num">{{ $t('payments.invoice.price') }}</th>
            <th class="num">Сумма</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, idx) in items" :key="idx">
            <td><input v-model="item.name" type="text" class="cell-input" /></td>
            <td class="num"><input v-model.number="item.quantity" type="number" min="0" step="0.01" class="cell-input num-input" /></td>
            <td class="num"><input v-model.number="item.price" type="number" min="0" step="0.01" class="cell-input num-input" /></td>
            <td class="num total-cell">{{ formatMoney(item.quantity * item.price) }}</td>
            <td>
              <button v-if="items.length > 1" class="remove-btn" @click="removeItem(idx)">
                <X :size="14" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <button class="add-row-btn" @click="addItem">
        <Plus :size="14" /> {{ $t('payments.invoice.add_row') }}
      </button>

      <div class="total-section">
        <span class="total-label">{{ $t('payments.invoice.total') }}:</span>
        <span class="total-value">{{ formatMoney(total) }}</span>
      </div>
    </div>

    <template #footer>
      <AppButton variant="ghost" @click="$emit('close')">{{ $t('common.cancel') }}</AppButton>
      <AppButton variant="primary" @click="submit" :disabled="items.length === 0">
        {{ $t('common.create') }}
      </AppButton>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Plus, X } from 'lucide-vue-next'
import AppModal from '../ui/AppModal.vue'
import AppButton from '../ui/AppButton.vue'
import { formatMoney } from '../../utils/format'

const props = defineProps<{
  visible: boolean
  contractName: string
  contractAmount: number
}>()

const emit = defineEmits(['close', 'submit'])

interface InvoiceItem {
  name: string
  quantity: number
  price: number
}

const items = ref<InvoiceItem[]>([{ name: '', quantity: 1, price: 0 }])

watch(() => props.visible, (v) => {
  if (v) {
    items.value = [{ name: props.contractName || 'Услуга', quantity: 1, price: props.contractAmount || 0 }]
  }
})

const total = computed(() => items.value.reduce((s, i) => s + i.quantity * i.price, 0))

function addItem() {
  items.value.push({ name: '', quantity: 1, price: 0 })
}

function removeItem(idx: number) {
  items.value.splice(idx, 1)
}

function submit() {
  emit('submit', items.value.filter(i => i.name && i.price > 0))
}
</script>

<style scoped>
.items-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 12px;
}

.items-table th {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 8px 6px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}

.items-table th.num {
  text-align: right;
}

.items-table td {
  padding: 4px 6px;
  border-bottom: 1px solid var(--border-light);
}

.cell-input {
  width: 100%;
  padding: 6px 8px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 6px;
  color: var(--text);
}

.cell-input:focus {
  border-color: var(--accent);
}

.num-input {
  text-align: right;
  width: 100px;
}

.total-cell {
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  color: var(--text-bright);
}

.remove-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  color: var(--text-muted);
}

.remove-btn:hover {
  color: var(--danger);
  background: var(--danger)22;
}

.add-row-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--accent);
  border: 1px dashed var(--hairline);
  border-radius: 6px;
  width: 100%;
  justify-content: center;
  transition: all 0.15s;
}

.add-row-btn:hover {
  border-color: var(--accent);
  background: var(--accent-bg);
}

.total-section {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--border-light);
}

.total-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.total-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
}
</style>
