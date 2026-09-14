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
  if (['total_amount', 'invoiced', 'debt', 'paid'].includes(stat.label)) {
    return formatMoney(stat.value)
  }
  return String(stat.value)
}
</script>

<style scoped>
.payment-stats {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}
</style>
