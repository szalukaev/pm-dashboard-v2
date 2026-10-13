<template>
  <div class="payment-stats">
    <AppStatCard
      v-for="stat in stats"
      :key="stat.label"
      :label="$t('payments.stats.' + stat.label)"
      :value="formatValue(stat)"
      :variant="(stat.variant as 'danger' | 'success' | 'warning' | 'info' | undefined)"
    />
  </div>
</template>

<script setup lang="ts">
import AppStatCard from '../ui/AppStatCard.vue'
import type { PaymentStat } from '../../stores/payments'
import { formatMoney } from '../../utils/format'

defineProps<{ stats: PaymentStat[] }>()

function formatValue(stat: PaymentStat): string {
  if (['total_amount', 'invoiced', 'debt', 'paid', 'closed_amount'].includes(stat.label)) {
    return formatMoney(stat.value)
  }
  return String(stat.value)
}
</script>

<style scoped>
.payment-stats {
  display: grid;
  /* Карточки делят всю ширину и на узком экране переносятся поровну. Меньше
     260 px нельзя: девятизначная сумма с копейками упирается в рамку */
  grid-template-columns: repeat(auto-fit, minmax(min(260px, 100%), 1fr));
  gap: 12px;
  margin-bottom: 24px;
}
</style>
