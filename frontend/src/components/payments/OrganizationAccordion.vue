<template>
  <!-- Организации — сеткой карточек. Свёрнутая карточка показывает только итоги;
       по клику она растягивается на всю ширину и показывает договоры -->
  <div class="org-grid">
    <div
      v-for="org in sortedOrganizations"
      :key="org.id"
      class="org-card"
      :class="{ open: isOpen(org.id) }"
      :data-testid="'org-' + org.id"
    >
      <div class="org-header" @click="toggle(org.id)">
        <!-- Подсказка: что за число в кружке и как оно посчитано; нажатие на
             кружок не сворачивает и не разворачивает карточку -->
        <AppHint class="ring-hint" :text="ringHint(org.id)" @click.stop>
        <svg class="ring" viewBox="0 0 36 36">
          <circle cx="18" cy="18" r="15" class="ring-track" />
          <circle
            cx="18" cy="18" r="15"
            class="ring-fill"
            :class="{ 'has-debt': orgSum(org.id, 'debt_amount') > 0 }"
            :stroke-dasharray="ringDash(orgPct(org.id))"
          />
          <text x="18" y="21.5" text-anchor="middle">{{ orgPct(org.id) }}</text>
        </svg>
        </AppHint>
        <div class="org-title">
          <div class="org-name">{{ org.name }}</div>
          <div class="org-meta">
            <span v-if="org.inn">{{ $t('payments.org.inn', { inn: org.inn }) }} · </span>{{ $t('payments.org.contracts', { count: orgContracts(org.id).length }) }}<span v-if="org.owner"> · {{ org.owner }}</span>
          </div>
        </div>

        <div v-if="isOpen(org.id)" class="org-actions" @click.stop>
          <button v-if="auth.canPayment('contract_create')" class="action-btn" @click="$emit('add-contract', org.id)" :title="$t('payments.add_contract')" data-testid="org-add-contract">
            <Plus :size="14" />
          </button>
          <button v-if="auth.canPayment('org_edit')" class="action-btn" @click="$emit('edit-org', org)" :title="$t('common.edit')">
            <Pencil :size="14" />
          </button>
          <button v-if="auth.canPayment('org_delete')" class="action-btn danger-btn" @click="$emit('delete-org', org.id)" :title="$t('common.delete')">
            <Trash2 :size="14" />
          </button>
        </div>
        <span v-else class="org-debt-short" :class="{ 'has-debt': orgSum(org.id, 'debt_amount') > 0 }">
          {{ orgSum(org.id, 'debt_amount') > 0 ? '−' + formatMoney(orgSum(org.id, 'debt_amount')) : $t('payments.org.no_debt') }}
        </span>
        <ChevronRight :size="16" class="chevron" />
      </div>

      <!-- Суммы с подписями, каждая своей строкой: большие числа не помещаются в одну -->
      <div v-if="!isOpen(org.id)" class="org-foot">
        <div class="foot-row"><span>{{ $t('payments.org.total_label') }}</span><b>{{ formatMoney(orgSum(org.id, 'total_amount')) }}</b></div>
        <div class="foot-row"><span>{{ $t('payments.stats.invoiced') }}</span><b>{{ formatMoney(orgSum(org.id, 'invoiced_amount')) }}</b></div>
      </div>

      <div v-else class="org-body">
        <ContractRow
          v-for="contract in orgContracts(org.id)"
          :key="contract.id"
          :contract="contract"
          @edit="$emit('edit-contract', contract)"
          @delete="$emit('delete-contract', contract.id)"
          @issue-invoice="$emit('issue-invoice', contract.id)"
        />
        <div v-if="orgContracts(org.id).length === 0" class="org-empty">{{ $t('payments.org.empty') }}</div>
      </div>
    </div>

    <!-- Договоры без организации -->
    <div v-if="noOrgContracts.length > 0" class="org-card no-org" :class="{ open: isOpen(0) }">
      <div class="org-header" @click="toggle(0)">
        <div class="org-title">
          <div class="org-name">{{ $t('payments.without_org') }}</div>
          <div class="org-meta">{{ $t('payments.org.contracts', { count: noOrgContracts.length }) }}</div>
        </div>
        <ChevronRight :size="16" class="chevron" />
      </div>
      <div v-if="isOpen(0)" class="org-body">
        <ContractRow
          v-for="contract in noOrgContracts"
          :key="contract.id"
          :contract="contract"
          @edit="$emit('edit-contract', contract)"
          @delete="$emit('delete-contract', contract.id)"
          @issue-invoice="$emit('issue-invoice', contract.id)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronRight, Plus, Pencil, Trash2 } from 'lucide-vue-next'
import ContractRow from './ContractRow.vue'
import AppHint from '../ui/AppHint.vue'
import { useI18n } from 'vue-i18n'
import { formatMoney } from '../../utils/format'
import type { Organization, Contract } from '../../stores/payments'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const { t } = useI18n()

const props = defineProps<{
  organizations: Organization[]
  contracts: Contract[]
}>()

defineEmits(['add-contract', 'edit-org', 'delete-org', 'edit-contract', 'delete-contract', 'issue-invoice'])

const openOrgs = ref<Set<number>>(new Set())

function toggle(id: number) {
  if (openOrgs.value.has(id)) {
    openOrgs.value.delete(id)
  } else {
    openOrgs.value.add(id)
  }
}

function isOpen(id: number) { return openOrgs.value.has(id) }

function orgContracts(orgId: number) {
  return props.contracts.filter(c => c.organization_id === orgId)
}

// Сначала организации с долгом — от большего к меньшему, без долга — в конце
// (между собой в прежнем порядке)
const sortedOrganizations = computed(() =>
  props.organizations
    .map((org, index) => ({ org, index, debt: orgSum(org.id, 'debt_amount') }))
    .sort((a, b) => (b.debt - a.debt) || (a.index - b.index))
    .map(item => item.org)
)

const noOrgContracts = computed(() => props.contracts.filter(c => !c.organization_id))

// Цифра организации: сумма по её договорам
function orgSum(orgId: number, field: 'total_amount' | 'invoiced_amount' | 'debt_amount' | 'total_paid') {
  return orgContracts(orgId).reduce((sum, c) => sum + (c[field] || 0), 0)
}

// Доля оплаченного от суммы договоров организации, целым процентом
function orgPct(orgId: number) {
  const total = orgSum(orgId, 'total_amount')
  if (!total) return 0
  return Math.min(100, Math.round((orgSum(orgId, 'total_paid') / total) * 100))
}

// Подсказка к кружку: доля оплаченного и из чего она посчитана
function ringHint(orgId: number) {
  return t('payments.org.ring_hint', {
    pct: orgPct(orgId),
    paid: formatMoney(orgSum(orgId, 'total_paid')),
    total: formatMoney(orgSum(orgId, 'total_amount')),
  })
}

// Длина дуги кольца: окружность r=15 ≈ 94.25
function ringDash(pct: number) {
  const full = 2 * Math.PI * 15
  return `${(pct / 100) * full} ${full}`
}
</script>

<style scoped>
.org-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 380px), 1fr));
  gap: 10px;
  align-items: start;
}

.org-card {
  min-width: 0;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  transition: border-color 0.15s;
}

.org-card:hover {
  border-color: var(--hairline-strong);
}

/* Раскрытая карточка — на всю ширину сетки */
.org-card.open {
  grid-column: 1 / -1;
  border-color: var(--border-accent);
}

.org-card.no-org {
  border-style: dashed;
}

.org-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 14px 0;
  cursor: pointer;
  user-select: none;
}

.org-card.open .org-header,
.org-card.no-org .org-header {
  padding-bottom: 14px;
}

.ring {
  width: 40px;
  height: 40px;
  flex: none;
}

.ring circle {
  fill: none;
  stroke-width: 4;
}

.ring-track {
  stroke: var(--surface-3);
}

.ring-fill {
  stroke: var(--success);
  stroke-linecap: round;
  transform: rotate(-90deg);
  transform-origin: 18px 18px;
}

.ring-fill.has-debt {
  stroke: var(--warning);
}

.ring text {
  font-size: 9px;
  font-weight: 600;
  fill: var(--text-muted);
}

.org-title {
  flex: 1;
  min-width: 0;
}

.org-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  /* Длинное название — в две строки, дальше многоточие */
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
}

.org-meta {
  margin-top: 1px;
  font-size: 12px;
  color: var(--text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.org-debt-short {
  flex: none;
  font-size: 13px;
  font-weight: 600;
  color: var(--success);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.org-debt-short.has-debt {
  color: var(--danger);
}

.chevron {
  flex: none;
  color: var(--text-faint);
  transition: transform 0.2s;
}

.org-card.open .chevron {
  transform: rotate(90deg);
}

.org-foot {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 14px 14px 66px;
  font-size: 12px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}

.foot-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  white-space: nowrap;
}

.foot-row b {
  font-weight: 500;
  color: var(--text);
}

.ring-hint {
  flex: none;
  display: inline-flex;
}

.org-actions {
  display: flex;
  gap: 4px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
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

.org-body {
  border-top: 1px solid var(--border-light);
}

.org-empty {
  padding: 14px 16px;
  font-size: 12px;
  color: var(--text-faint);
}

/* Телефон: одна колонка, итоги свёрнутой карточки — в шапке */
@media (max-width: 768px) {
  .org-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .org-header {
    gap: 10px;
    padding: 12px;
  }

  .ring {
    width: 36px;
    height: 36px;
  }

  .org-foot {
    display: none;
  }
}

/* Узкий телефон: рядом с суммой названию не хватает места — сумма уходит строкой ниже */
@media (max-width: 480px) {
  .org-header {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    column-gap: 12px;
    row-gap: 4px;
    padding-bottom: 14px;
  }

  .ring-hint {
    grid-column: 1;
    grid-row: 1 / 3;
    align-self: center;
  }

  .org-title {
    grid-column: 2;
    grid-row: 1;
  }

  .org-debt-short,
  .org-actions {
    grid-column: 2;
    grid-row: 2;
    justify-self: start;
  }

  .chevron {
    grid-column: 3;
    grid-row: 1 / 3;
    align-self: center;
  }
}
</style>
