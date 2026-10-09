<template>
  <div class="contract-row" :class="{ overdue: contract.is_overdue, 'fully-paid': contract.is_fully_paid }" :data-testid="'contract-' + contract.id">
    <div class="row-main" @click="expanded = !expanded">
      <ChevronDown :size="14" class="chevron" :class="{ collapsed: !expanded }" />
      <span class="type-badge" :class="contract.contract_type" :title="$t('payments.contract_types.' + contract.contract_type)">
        {{ $t('payments.contract_types_short.' + contract.contract_type) }}
      </span>
      <span class="contract-name" :title="contract.name">{{ contract.name }}</span>
      <span class="amount" :title="$t('payments.contract.amount_with_vat')">{{ formatMoney(contract.total_amount) }}</span>
      <span class="invoiced" :title="$t('payments.stats.invoiced')">{{ formatMoney(contract.invoiced_amount) }}</span>
      <span class="debt" :class="{ 'has-debt': contract.debt_amount > 0 }" :title="$t('payments.stats.debt')">
        {{ formatMoney(contract.debt_amount) }}
      </span>
      <span class="end-date" v-if="contract.end_date">
        <span v-if="contract.is_overdue" class="overdue-icon" :title="$t('payments.contract.overdue')"><AlertCircle :size="12" /></span>
        {{ formatDate(contract.end_date) }}
      </span>

      <div class="row-actions" @click.stop>
        <button class="action-btn" @click="$emit('issue-invoice')" :title="$t('payments.issue_invoice')" data-testid="contract-issue-invoice">
          <Receipt :size="14" />
        </button>
        <button class="action-btn" @click="$emit('edit')" :title="$t('common.edit')">
          <Pencil :size="14" />
        </button>
        <button class="action-btn danger-btn" @click="$emit('delete')" :title="$t('common.delete')">
          <Trash2 :size="14" />
        </button>
      </div>
    </div>

    <div v-if="expanded" class="row-details">
      <InvoiceTable :contract-id="contract.id" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown, AlertCircle, Receipt, Pencil, Trash2 } from 'lucide-vue-next'
import InvoiceTable from './InvoiceTable.vue'
import { formatMoney, formatDate } from '../../utils/format'
import type { Contract } from '../../stores/payments'

defineProps<{ contract: Contract }>()
defineEmits(['edit', 'delete', 'issue-invoice'])

// The invoices of the contract are shown when it is opened
const expanded = ref(false)
</script>

<style scoped>
.contract-row {
  border-bottom: 1px solid var(--border-light);
}

.contract-row:last-child {
  border-bottom: none;
}

.contract-row.overdue {
  border-left: 3px solid var(--warning);
}

.contract-row.fully-paid {
  background: var(--success)08;
}

.row-main {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  cursor: pointer;
  transition: background 0.15s;
}

.row-main:hover {
  background: var(--bg-hover);
}

.chevron {
  color: var(--text-faint);
  transition: transform 0.2s;
  flex-shrink: 0;
}

.chevron.collapsed {
  transform: rotate(-90deg);
}

.type-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 20px;
  font-size: 10px;
  font-weight: 600;
  border-radius: 9999px;
  flex-shrink: 0;
}

.type-badge.service {
  background: var(--accent-bg);
  color: var(--accent);
}

.type-badge.onetime {
  background: var(--orange)22;
  color: var(--orange);
}

.contract-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 120px;
}

.amount,
.invoiced,
.debt {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  min-width: 100px;
  text-align: right;
}

.amount { color: var(--text-dim); }
.invoiced { color: var(--text-muted); }
.debt.has-debt { color: var(--danger); font-weight: 500; }

.end-date {
  font-size: 12px;
  color: var(--text-faint);
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 80px;
}

.overdue-icon {
  color: var(--warning);
}

.row-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
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

.row-details {
  padding: 0 16px 12px 40px;
}

@media (max-width: 768px) {
  .row-main {
    flex-wrap: wrap;
    gap: 6px;
  }
  .amount, .invoiced, .debt {
    min-width: auto;
  }
}
</style>
