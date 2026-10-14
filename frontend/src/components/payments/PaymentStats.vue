<template>
  <!-- Сводка в одной карточке: долг крупно, полоса «оплачено / выставлено / не выставлено»
       от общей суммы договоров и подписи с суммами -->
  <div class="payment-summary" data-testid="payment-stats">
    <div class="summary-top">
      <div class="summary-main">
        <div class="summary-label">{{ $t('payments.stats.debt') }}</div>
        <div class="summary-debt" :class="{ 'has-debt': debt > 0 }">{{ formatMoney(debt) }}</div>
      </div>
      <div class="summary-count">
        <b>{{ contracts }}</b>
        <span>{{ $t('payments.stats.contracts') }}</span>
      </div>
    </div>

    <div v-if="totalAmount > 0" class="summary-bar" :title="$t('payments.stats.total_amount') + ': ' + formatMoney(totalAmount)">
      <i class="seg-paid" :style="{ width: pct(paid) }"></i>
      <i class="seg-unpaid" :style="{ width: pct(Math.max(invoiced - paid, 0)) }"></i>
    </div>

    <div class="summary-legend">
      <div class="legend-item">
        <span class="legend-dot seg-paid"></span>
        <span class="legend-label">{{ $t('payments.stats.paid') }}</span>
        <b>{{ formatMoney(paid) }}</b>
      </div>
      <div class="legend-item">
        <span class="legend-dot seg-unpaid"></span>
        <span class="legend-label">{{ $t('payments.stats.invoiced') }}</span>
        <b>{{ formatMoney(invoiced) }}</b>
      </div>
      <div class="legend-item">
        <span class="legend-dot seg-total"></span>
        <span class="legend-label">{{ $t('payments.stats.total_amount') }}</span>
        <b>{{ formatMoney(totalAmount) }}</b>
      </div>
      <div class="legend-item">
        <span class="legend-dot seg-closed"></span>
        <span class="legend-label">{{ $t('payments.stats.closed_amount') }}</span>
        <b>{{ formatMoney(closedAmount) }}</b>
      </div>
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

// Доля от общей суммы договоров, не больше 100 %
function pct(amount: number): string {
  if (!totalAmount.value) return '0%'
  return Math.min(100, (amount / totalAmount.value) * 100) + '%'
}
</script>

<style scoped>
.payment-summary {
  margin-bottom: 20px;
  padding: 16px 18px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.summary-top {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.summary-main {
  min-width: 0;
}

.summary-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.summary-debt {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.6px;
  color: var(--text-bright);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.summary-debt.has-debt {
  color: var(--danger);
}

.summary-count {
  display: flex;
  align-items: baseline;
  gap: 6px;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 12px;
}

.summary-count b {
  font-size: 18px;
  color: var(--text-bright);
}

.summary-bar {
  display: flex;
  height: 8px;
  margin-bottom: 12px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--surface-3);
}

.summary-bar i {
  display: block;
  height: 100%;
}

.seg-paid {
  background: var(--success);
}

.seg-unpaid {
  background: var(--warning);
}

.seg-total {
  background: var(--surface-3);
  box-shadow: inset 0 0 0 1px var(--border);
}

.seg-closed {
  background: var(--text-faint);
}

.summary-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  white-space: nowrap;
}

.legend-dot {
  width: 8px;
  height: 8px;
  flex: none;
  border-radius: 2px;
}

.legend-label {
  color: var(--text-muted);
}

.legend-item b {
  color: var(--text-bright);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

/* Телефон: подписи в две колонки, сумма под названием */
@media (max-width: 768px) {
  .payment-summary {
    padding: 14px;
    margin-bottom: 14px;
  }

  .summary-debt {
    font-size: 24px;
  }

  .summary-legend {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px 12px;
  }

  .legend-item {
    display: grid;
    grid-template-columns: 8px 1fr;
    column-gap: 6px;
    row-gap: 1px;
    white-space: normal;
  }

  .legend-item b {
    grid-column: 2;
    font-size: 13px;
  }
}
</style>
