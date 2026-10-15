<template>
  <!-- Сводка — четыре карточки: долг, оплачено, выставлено, не выставлено.
       На телефоне долг во всю ширину, остальное — сеткой 2×2 -->
  <div class="payment-summary" data-testid="payment-stats">
    <div class="kpi kpi-debt">
      <div class="kpi-label">{{ $t('payments.stats.debt') }}</div>
      <div class="kpi-value" :class="{ 'has-debt': debt > 0 }">{{ formatMoney(debt) }}</div>
      <div class="kpi-sub">{{ $t('payments.stats.contracts_count', { n: contracts }) }}</div>
    </div>
    <div class="kpi">
      <div class="kpi-label">{{ $t('payments.stats.paid') }}</div>
      <div class="kpi-value">{{ formatMoney(paid) }}</div>
      <div class="kpi-bar" v-if="totalAmount > 0"><i class="seg-paid" :style="{ width: pct(paid) }"></i></div>
    </div>
    <div class="kpi">
      <div class="kpi-label">{{ $t('payments.stats.invoiced') }}</div>
      <div class="kpi-value">{{ formatMoney(invoiced) }}</div>
      <div class="kpi-bar" v-if="totalAmount > 0"><i class="seg-unpaid" :style="{ width: pct(invoiced) }"></i></div>
    </div>
    <div class="kpi" :title="$t('payments.stats.closed_amount') + ': ' + formatMoney(closedAmount)">
      <div class="kpi-label">{{ $t('payments.stats.not_invoiced') }}</div>
      <div class="kpi-value muted">{{ formatMoney(notInvoiced) }}</div>
      <div class="kpi-sub">{{ $t('payments.stats.of_total', { amount: formatMoney(totalAmount) }) }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PaymentStat } from '../../stores/payments'
import { formatMoney } from '../../utils/format'

const props = defineProps<{ stats: PaymentStat[] }>()

function value(label: string): number {
  return Number(props.stats.find(s => s.label === label)?.value) || 0
}

const contracts = computed(() => value('contracts'))
const totalAmount = computed(() => value('total_amount'))
const invoiced = computed(() => value('invoiced'))
const debt = computed(() => value('debt'))
const paid = computed(() => value('paid'))
const closedAmount = computed(() => value('closed_amount'))
const notInvoiced = computed(() => Math.max(totalAmount.value - invoiced.value, 0))

// Доля от общей суммы договоров, не больше 100 %
function pct(amount: number): string {
  if (!totalAmount.value) return '0%'
  return Math.min(100, (amount / totalAmount.value) * 100) + '%'
}
</script>

<style scoped>
.payment-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.kpi {
  min-width: 0;
  padding: 12px 14px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 10px;
}

.kpi-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.kpi-value {
  margin-top: 2px;
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.4px;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.kpi-value.has-debt {
  color: var(--danger);
}

.kpi-value.muted {
  color: var(--text-muted);
}

.kpi-sub {
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.kpi-bar {
  height: 3px;
  margin-top: 8px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--surface-3);
}

.kpi-bar i {
  display: block;
  height: 100%;
}

.seg-paid {
  background: var(--success);
}

.seg-unpaid {
  background: var(--warning);
}

/* Телефон: долг во всю ширину, остальные три — сеткой */
@media (max-width: 768px) {
  .payment-summary {
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .kpi {
    padding: 10px 12px;
  }

  .kpi-debt {
    grid-column: 1 / -1;
  }

  .kpi-value {
    font-size: 16px;
  }

  .kpi-debt .kpi-value {
    font-size: 24px;
  }
}

/* Узкий телефон: в половину ширины сумма не помещается — каждая карточка в свою строку */
@media (max-width: 480px) {
  .payment-summary {
    grid-template-columns: 1fr;
  }
}
</style>
