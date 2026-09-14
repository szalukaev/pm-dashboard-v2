<template>
  <div class="invoice-table-wrapper">
    <div v-if="loading" class="loading">
      <AppSpinner :size="16" />
    </div>

    <table v-else-if="invoices.length > 0" class="invoice-table">
      <thead>
        <tr>
          <th>№</th>
          <th>{{ $t('payments.invoice.date') }}</th>
          <th class="num">{{ $t('payments.invoice.amount') }}</th>
          <th class="num">{{ $t('payments.invoice.paid') }}</th>
          <th class="num">{{ $t('payments.invoice.remainder') }}</th>
          <th>{{ $t('payments.invoice.status') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(inv, idx) in invoices" :key="inv.id">
          <td>{{ idx + 1 }}</td>
          <td>{{ formatDate(inv.issued_at) }}</td>
          <td class="num">{{ formatMoney(inv.total) }}</td>
          <td class="num">{{ formatMoney(inv.paid_amount) }}</td>
          <td class="num" :class="{ 'has-remainder': inv.remainder > 0 }">{{ formatMoney(inv.remainder) }}</td>
          <td>
            <span class="status-badge" :class="inv.status">{{ statusLabel(inv.status) }}</span>
          </td>
          <td class="actions-cell">
            <button v-if="inv.status !== 'paid'" class="action-btn" @click="$emit('pay', { contractId, invoiceId: inv.id, remainder: inv.remainder })" title="Оплатить">
              <CreditCard :size="14" />
            </button>
            <button class="action-btn" @click="$emit('download', inv.id)" title="Скачать">
              <Download :size="14" />
            </button>
            <button class="action-btn" @click="copyInvoiceInfo(inv, idx)" title="Копировать">
              <Copy :size="14" />
            </button>
            <button class="action-btn danger-btn" @click="$emit('delete-invoice', inv.id)" title="Удалить">
              <Trash2 :size="14" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-else class="empty-invoices">
      Нет счетов
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { CreditCard, Download, Copy, Trash2 } from 'lucide-vue-next'
import AppSpinner from '../ui/AppSpinner.vue'
import { usePaymentsStore, type Invoice } from '../../stores/payments'
import { formatMoney, formatDate } from '../../utils/format'

const props = defineProps<{ contractId: number }>()
defineEmits(['pay', 'download', 'delete-invoice'])

const store = usePaymentsStore()
const invoices = ref<Invoice[]>([])
const loading = ref(true)

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    unpaid: 'Не оплачен',
    partial: 'Частично',
    paid: 'Оплачен',
  }
  return map[status] || status
}

function copyInvoiceInfo(inv: Invoice, idx: number) {
  const text = `Счёт №${idx + 1} от ${formatDate(inv.issued_at)}\nСумма: ${formatMoney(inv.total)}\nОплачено: ${formatMoney(inv.paid_amount)}\nОстаток: ${formatMoney(inv.remainder)}\nСтатус: ${statusLabel(inv.status)}`
  navigator.clipboard.writeText(text)
}

onMounted(async () => {
  invoices.value = await store.fetchInvoices(props.contractId)
  loading.value = false
})
</script>

<style scoped>
.invoice-table-wrapper {
  margin-top: 8px;
}

.loading {
  display: flex;
  justify-content: center;
  padding: 12px;
}

.invoice-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.invoice-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}

.invoice-table th.num {
  text-align: right;
}

.invoice-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
}

.invoice-table td.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.has-remainder {
  color: var(--warning);
  font-weight: 500;
}

.status-badge {
  display: inline-block;
  padding: 2px 6px;
  font-size: 10px;
  font-weight: 500;
  border-radius: 9999px;
}

.status-badge.unpaid {
  background: var(--danger)22;
  color: var(--danger);
}

.status-badge.partial {
  background: var(--warning)22;
  color: var(--warning);
}

.status-badge.paid {
  background: var(--success)22;
  color: var(--success);
}

.actions-cell {
  display: flex;
  gap: 2px;
  justify-content: flex-end;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.action-btn:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.danger-btn:hover {
  color: var(--danger);
}

.empty-invoices {
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
  padding: 12px;
}
</style>
