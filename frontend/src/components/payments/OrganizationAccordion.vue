<template>
  <div class="org-accordion">
    <div
      v-for="org in organizations"
      :key="org.id"
      class="org-group"
    >
      <div class="org-header" @click="toggle(org.id)">
        <ChevronDown :size="16" class="chevron" :class="{ collapsed: !isOpen(org.id) }" />
        <Building2 :size="16" class="org-icon" />
        <span class="org-name">{{ org.name }}</span>
        <span class="org-stat">{{ orgContracts(org.id).length }} дог.</span>
        <span class="org-stat">{{ formatMoney(orgTotal(org.id)) }}</span>

        <div class="org-actions" @click.stop>
          <button class="action-btn" @click="$emit('add-contract', org.id)" title="Добавить договор">
            <Plus :size="14" />
          </button>
          <button class="action-btn" @click="$emit('edit-org', org)" title="Редактировать">
            <Pencil :size="14" />
          </button>
          <button class="action-btn danger-btn" @click="$emit('delete-org', org.id)" title="Удалить">
            <Trash2 :size="14" />
          </button>
        </div>
      </div>

      <div v-if="isOpen(org.id)" class="org-body">
        <ContractRow
          v-for="contract in orgContracts(org.id)"
          :key="contract.id"
          :contract="contract"
          @edit="$emit('edit-contract', contract)"
          @delete="$emit('delete-contract', contract.id)"
          @issue-invoice="$emit('issue-invoice', contract.id)"
          @view-invoices="$emit('view-invoices', contract)"
        />
      </div>
    </div>

    <!-- Contracts without organization -->
    <div v-if="noOrgContracts.length > 0" class="org-group">
      <div class="org-header" @click="toggle(0)">
        <ChevronDown :size="16" class="chevron" :class="{ collapsed: !isOpen(0) }" />
        <Building2 :size="16" class="org-icon" style="opacity: 0.3" />
        <span class="org-name">{{ $t('payments.without_org') }}</span>
        <span class="org-stat">{{ noOrgContracts.length }} дог.</span>
      </div>
      <div v-if="isOpen(0)" class="org-body">
        <ContractRow
          v-for="contract in noOrgContracts"
          :key="contract.id"
          :contract="contract"
          @edit="$emit('edit-contract', contract)"
          @delete="$emit('delete-contract', contract.id)"
          @issue-invoice="$emit('issue-invoice', contract.id)"
          @view-invoices="$emit('view-invoices', contract)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown, Building2, Plus, Pencil, Trash2 } from 'lucide-vue-next'
import ContractRow from './ContractRow.vue'
import { formatMoney } from '../../utils/format'
import type { Organization, Contract } from '../../stores/payments'

const props = defineProps<{
  organizations: Organization[]
  contracts: Contract[]
}>()

defineEmits(['add-contract', 'edit-org', 'delete-org', 'edit-contract', 'delete-contract', 'issue-invoice', 'view-invoices'])

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

const noOrgContracts = computed(() => props.contracts.filter(c => !c.organization_id))

function orgTotal(orgId: number) {
  return orgContracts(orgId).reduce((sum, c) => sum + c.amount * (c.vat_rate === '5' ? 1.05 : 1), 0)
}
</script>

<style scoped>
.org-accordion {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.org-group {
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
}

.org-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  background: var(--surface-1);
  cursor: pointer;
  transition: background 0.15s;
  user-select: none;
}

.org-header:hover {
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

.org-icon {
  color: var(--accent);
  flex-shrink: 0;
}

.org-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.org-stat {
  font-size: 12px;
  color: var(--text-muted);
  margin-left: 8px;
}

.org-actions {
  display: flex;
  gap: 4px;
  margin-left: auto;
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

.org-body {
  border-top: 1px solid var(--border-light);
}
</style>
