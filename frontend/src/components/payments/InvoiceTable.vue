<template>
  <div class="invoice-table-wrapper">
    <div v-if="loading" class="loading">
      <AppSpinner :size="16" />
    </div>

    <table v-else-if="invoices.length > 0" class="invoice-table">
      <thead>
        <tr>
          <th>{{ $t('payments.invoice.number') }}</th>
          <th>{{ $t('payments.invoice.date') }}</th>
          <th class="num">{{ $t('payments.invoice.amount') }}</th>
          <th class="num">{{ $t('payments.invoice.paid') }}</th>
          <th class="num">{{ $t('payments.invoice.remainder') }}</th>
          <th>{{ $t('payments.invoice.pay_date') }}</th>
          <th>{{ $t('payments.invoice.status') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="inv in invoices" :key="inv.id" :data-testid="'invoice-' + inv.id">
          <td class="number">{{ inv.number || '—' }}</td>
          <td>{{ formatDate(inv.issued_at) }}</td>
          <td class="num">{{ formatMoney(inv.total) }}</td>
          <td class="num">{{ formatMoney(inv.paid_amount) }}</td>
          <td class="num" :class="{ 'has-remainder': inv.remainder > 0 }">{{ formatMoney(inv.remainder) }}</td>
          <td>{{ formatDate(inv.paid_at) }}</td>
          <td>
            <span class="status-badge" :class="inv.status">{{ statusLabel(inv.status) }}</span>
          </td>
          <td class="actions-cell">
            <button v-if="inv.status !== 'paid'" class="action-btn" :title="$t('payments.invoice.pay')" data-testid="invoice-pay" @click="actions?.pay(contractId, inv)">
              <CreditCard :size="14" />
            </button>
            <button class="action-btn" :title="$t('payments.invoice.download')" data-testid="invoice-download" @click="actions?.download(contractId, inv)">
              <Download :size="14" />
            </button>
            <button class="action-btn" :title="$t('payments.invoice.copy')" data-testid="invoice-copy" @click="copyInvoiceInfo(inv)">
              <Copy :size="14" />
            </button>
            <button class="action-btn danger-btn" :title="$t('common.delete')" data-testid="invoice-delete" @click="actions?.remove(contractId, inv)">
              <Trash2 :size="14" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <AppPagination
      v-if="!loading"
      :total="total"
      :limit="pageSize"
      :offset="offset"
      @update:offset="setPage"
      @update:limit="setPageSize"
    />

    <div v-if="!loading && invoices.length === 0" class="empty-invoices">
      {{ $t('payments.invoice.empty') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, inject, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CreditCard, Download, Copy, Trash2 } from 'lucide-vue-next'
import AppSpinner from '../ui/AppSpinner.vue'
import AppPagination from '../ui/AppPagination.vue'
import { useSettingsStore } from '../../stores/settings'
import { usePaymentsStore, type Invoice } from '../../stores/payments'
import { formatMoney, formatDate } from '../../utils/format'
import { useSwal } from '../../composables/useSwal'
import { invoiceActionsKey } from './invoiceActions'

const props = defineProps<{ contractId: number }>()

const { t, te } = useI18n()
const { toast } = useSwal()
// Paying, downloading and deleting belong to the page
const actions = inject(invoiceActionsKey)

const store = usePaymentsStore()
const settingsStore = useSettingsStore()
const PAGE_TABLE = 'payments_table'
const pageSize = computed(() => settingsStore.pageSize(PAGE_TABLE))
const invoices = ref<Invoice[]>([])
const total = ref(0)
const offset = ref(0)
const loading = ref(true)

async function load() {
  const page = await store.fetchInvoices(props.contractId, pageSize.value, offset.value)
  invoices.value = page.invoices
  total.value = page.total
}

async function setPage(value: number) {
  offset.value = value
  await load()
}

async function setPageSize(size: number) {
  await settingsStore.setPageSize(PAGE_TABLE, size)
  offset.value = 0
  await load()
}

function statusLabel(status: string): string {
  const key = `payments.statuses.${status}`
  return te(key) ? t(key) : status
}

async function copyInvoiceInfo(inv: Invoice) {
  const text = t('payments.invoice.copy_text', {
    number: inv.number,
    date: formatDate(inv.issued_at),
    amount: formatMoney(inv.total),
    paid: formatMoney(inv.paid_amount),
    remainder: formatMoney(inv.remainder),
    status: statusLabel(inv.status),
  })
  try {
    await navigator.clipboard.writeText(text)
    toast(t('common.copied'), 'success')
  } catch {
    toast(t('payments.invoice.copy_error'), 'error')
  }
}

// An invoice was issued, paid or deleted somewhere on the page
watch(() => store.invoicesVersion, async () => {
  await load()
  // The last invoice of the last page was deleted: show the page before it
  if (invoices.value.length === 0 && total.value > 0) {
    offset.value = Math.floor((total.value - 1) / pageSize.value) * pageSize.value
    await load()
  }
})

onMounted(async () => {
  await settingsStore.ensureLoaded()
  await load()
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
