<template>
  <AppModal
    :model-value="visible"
    :title="sourceNumber ? $t('payments.invoice.copy_form_title', { number: sourceNumber }) : $t('payments.invoice.form_title')"
    width="1200px"
    @update:model-value="!$event && $emit('close')"
  >
    <div class="invoice-form">
      <table class="items-table">
        <!-- Ширины задаются один раз, чтобы заголовок стоял над своими значениями -->
        <colgroup>
          <col />
          <col class="col-quantity" />
          <col class="col-money" />
          <col class="col-money" />
          <col class="col-remove" />
        </colgroup>
        <thead>
          <tr>
            <th>{{ $t('payments.invoice.service_name') }}</th>
            <th class="num">{{ $t('payments.invoice.quantity') }}</th>
            <th class="num">{{ $t('payments.invoice.price') }}</th>
            <th class="num">{{ $t('payments.invoice.amount') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, idx) in items" :key="idx">
            <td><input v-model="item.name" type="text" class="cell-input" :maxlength="MAX_NAME" data-testid="invoice-item-name" /></td>
            <td><input v-model.number="item.quantity" type="number" min="0" step="0.01" class="cell-input num-input" data-testid="invoice-item-quantity" /></td>
            <td><AppMoneyInput v-model="item.price" class="cell-input num-input" data-testid="invoice-item-price" /></td>
            <td class="total-cell">{{ formatMoney((item.quantity || 0) * (item.price || 0)) }}</td>
            <td class="remove-cell">
              <button v-if="items.length > 1" class="remove-btn" :title="$t('common.delete')" @click="removeItem(idx)">
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
import AppMoneyInput from '../ui/AppMoneyInput.vue'
import { formatMoney } from '../../utils/format'
import { useI18n } from 'vue-i18n'

interface InvoiceItem {
  name: string
  quantity: number
  price: number
}

const props = defineProps<{
  visible: boolean
  contractName: string
  contractAmount: number
  // Позиции, с которых начинать: позиции копируемого счёта. Без них форма
  // начинается с одной позиции договора.
  initialItems?: InvoiceItem[]
  // Номер копируемого счёта — для заголовка
  sourceNumber?: string
}>()

const emit = defineEmits(['close', 'submit'])
const { t } = useI18n()

// Сервер принимает название позиции не длиннее стольких символов
const MAX_NAME = 2000

const items = ref<InvoiceItem[]>([{ name: '', quantity: 1, price: 0 }])

watch(() => props.visible, (v) => {
  if (v) {
    items.value = props.initialItems?.length
      ? props.initialItems.map(i => ({ name: i.name, quantity: Number(i.quantity) || 0, price: Number(i.price) || 0 }))
      : [{ name: props.contractName || t('payments.invoice.default_item'), quantity: 1, price: props.contractAmount || 0 }]
  }
})

const total = computed(() => items.value.reduce((s, i) => s + (i.quantity || 0) * (i.price || 0), 0))

function addItem() {
  items.value.push({ name: '', quantity: 1, price: 0 })
}

function removeItem(idx: number) {
  items.value.splice(idx, 1)
}

function submit() {
  emit('submit', items.value
    .filter(i => i.name.trim() && i.price > 0)
    .map(i => ({ name: i.name.trim(), quantity: Number(i.quantity) || 0, price: i.price })))
}
</script>

<style scoped>
/* Просторная форма: счёт из десятка позиций виден без прокрутки */
.invoice-form {
  display: flex;
  flex-direction: column;
  min-height: 55vh;
}

.total-section {
  margin-top: auto;
}

.items-table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  margin-bottom: 12px;
}

.col-quantity {
  width: 130px;
}

.col-money {
  width: 190px;
}

.col-remove {
  width: 40px;
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
  /* Над последней цифрой значений: отступ и рамка поля */
  padding-right: 15px;
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
  border-radius: 8px;
  color: var(--text);
}

.cell-input:focus-visible {
  border-color: var(--accent);
}

/* Число занимает свою колонку и заканчивается там же, где её заголовок */
.num-input {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.remove-cell {
  text-align: center;
}

/* Сумма заканчивается под последней цифрой полей слева от неё */
.items-table td.total-cell {
  padding-right: 15px;
  text-align: right;
  white-space: nowrap;
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
  border-radius: 8px;
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
  border-radius: 8px;
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
