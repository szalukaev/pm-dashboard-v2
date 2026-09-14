<template>
  <div class="payments-view">
    <div class="view-header">
      <h1 class="page-title">{{ $t('payments.title') }}</h1>
      <div class="header-actions">
        <AppButton variant="ghost" @click="exportCSV" data-testid="export-csv">
          <Download :size="14" /> {{ $t('payments.export_csv') }}
        </AppButton>
        <AppButton variant="primary" @click="showOrgForm = true" data-testid="add-org">
          + {{ $t('payments.add_organization') }}
        </AppButton>
      </div>
    </div>

    <!-- Filters -->
    <div class="filter-row">
      <div class="filter-group">
        <button
          v-for="t in filterOptions"
          :key="t.value"
          class="pill-btn"
          :class="{ active: filterType === t.value }"
          @click="setFilter(t.value)"
        >
          {{ $t(t.label) }}
        </button>
      </div>
      <label class="toggle-label">
        <input type="checkbox" v-model="showClosedVal" @change="toggleClosed" />
        {{ $t('payments.filters.show_closed') }}
      </label>
      <input
        v-model="searchVal"
        type="text"
        :placeholder="$t('payments.filters.search_placeholder')"
        class="search-input"
        @keyup.enter="doSearch"
      />
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading"><AppSpinner :size="32" /></div>

    <!-- Error -->
    <AppEmptyState v-else-if="error" state="error" @retry="fetchAll" />

    <!-- Content -->
    <template v-else>
      <PaymentStats :stats="stats" />
      <OrganizationAccordion
        :organizations="organizations"
        :contracts="contracts"
        @add-contract="startAddContract"
        @edit-org="editOrg"
        @delete-org="deleteOrg"
        @edit-contract="editContract"
        @delete-contract="deleteContract"
        @issue-invoice="issueInvoice"
        @view-invoices="viewInvoices"
      />
    </template>

    <!-- Org Form Modal -->
    <AppModal :model-value="showOrgForm" title="Организация" width="480px" @update:model-value="showOrgForm = $event">
      <form class="modal-form" @submit.prevent="submitOrg">
        <div class="form-field">
          <label>Название *</label>
          <input v-model="orgForm.name" type="text" required />
        </div>
        <div class="form-field">
          <label>Адрес</label>
          <input v-model="orgForm.address" type="text" />
        </div>
        <div class="form-field">
          <label>ИНН</label>
          <input v-model="orgForm.inn" type="text" />
        </div>
        <div class="form-field">
          <label>Контактное лицо</label>
          <input v-model="orgForm.contact_name" type="text" />
        </div>
        <div class="form-field">
          <label>Телефон</label>
          <input v-model="orgForm.contact_phone" type="text" />
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showOrgForm = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" @click="submitOrg" :disabled="!orgForm.name">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>

    <!-- Contract Form Modal -->
    <AppModal :model-value="showContractForm" title="Договор" width="520px" @update:model-value="showContractForm = $event">
      <form class="modal-form" @submit.prevent="submitContract">
        <div class="form-field">
          <label>Тип *</label>
          <select v-model="contractForm.contract_type">
            <option value="service">Сопровождение</option>
            <option value="onetime">Разовый</option>
          </select>
        </div>
        <div class="form-field">
          <label>Название *</label>
          <input v-model="contractForm.name" type="text" required />
        </div>
        <div class="form-field">
          <label>Сумма</label>
          <input v-model.number="contractForm.amount" type="number" min="0" step="0.01" />
        </div>
        <div class="form-field">
          <label>НДС</label>
          <select v-model="contractForm.vat_rate">
            <option value="none">Без НДС</option>
            <option value="5">НДС 5%</option>
          </select>
        </div>
        <div class="form-field">
          <label>Контактное лицо</label>
          <input v-model="contractForm.contact_name" type="text" />
        </div>
        <div class="form-row" v-if="contractForm.contract_type === 'service'">
          <div class="form-field">
            <label>Дата начала</label>
            <input v-model="contractForm.start_date" type="date" />
          </div>
          <div class="form-field">
            <label>Срок</label>
            <input v-model="contractForm.end_date" type="date" />
          </div>
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showContractForm = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" @click="submitContract" :disabled="!contractForm.name">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>

    <!-- Invoice Form -->
    <InvoiceForm
      :visible="showInvoiceForm"
      :contract-name="invoiceContractName"
      :contract-amount="invoiceContractAmount"
      @close="showInvoiceForm = false"
      @submit="handleCreateInvoice"
    />

    <!-- Pay Form -->
    <PayForm
      :visible="showPayForm"
      :default-amount="payDefaultAmount"
      @close="showPayForm = false"
      @submit="handlePay"
    />

    <!-- Delete Confirm -->
    <AppConfirmDialog
      :model-value="showDeleteConfirm"
      :title="'Удалить ' + deleteTarget + '?'"
      @update:model-value="showDeleteConfirm = $event"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Download } from 'lucide-vue-next'
import { usePaymentsStore, type Organization, type Contract } from '../stores/payments'
import PaymentStats from '../components/payments/PaymentStats.vue'
import OrganizationAccordion from '../components/payments/OrganizationAccordion.vue'
import InvoiceForm from '../components/payments/InvoiceForm.vue'
import PayForm from '../components/payments/PayForm.vue'
import AppButton from '../components/ui/AppButton.vue'
import AppModal from '../components/ui/AppModal.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppSpinner from '../components/ui/AppSpinner.vue'
import AppConfirmDialog from '../components/ui/AppConfirmDialog.vue'

const store = usePaymentsStore()
const { organizations, contracts, stats, loading, error, filterType } = storeToRefs(store)
const { fetchAll, exportCSV } = store

// Filters
const filterOptions = [
  { value: '', label: 'payments.filters.all' },
  { value: 'service', label: 'payments.filters.service' },
  { value: 'onetime', label: 'payments.filters.onetime' },
]
const showClosedVal = ref(false)
const searchVal = ref('')

function setFilter(type: string) {
  store.filterType = type
  store.fetchContracts()
}

function toggleClosed() {
  store.showClosed = showClosedVal.value
  store.fetchContracts()
}

function doSearch() {
  store.searchQuery = searchVal.value
  store.fetchContracts()
}

// Org form
const showOrgForm = ref(false)
const editingOrg = ref<Organization | null>(null)
const orgForm = reactive({ name: '', address: '', inn: '', contact_name: '', contact_phone: '' })

function editOrg(org: Organization) {
  editingOrg.value = org
  Object.assign(orgForm, { name: org.name, address: org.address || '', inn: org.inn || '', contact_name: org.contact_name || '', contact_phone: org.contact_phone || '' })
  showOrgForm.value = true
}

async function submitOrg() {
  if (editingOrg.value) {
    await store.updateOrganization(editingOrg.value.id, { ...orgForm })
  } else {
    await store.createOrganization({ ...orgForm })
  }
  showOrgForm.value = false
  editingOrg.value = null
  Object.assign(orgForm, { name: '', address: '', inn: '', contact_name: '', contact_phone: '' })
}

async function deleteOrg(id: number) {
  deleteTarget.value = 'организацию'
  deleteAction.value = () => store.deleteOrganization(id)
  showDeleteConfirm.value = true
}

// Contract form
const showContractForm = ref(false)
const editingContract = ref<Contract | null>(null)
const contractForm = reactive({
  contract_type: 'service', name: '', organization_id: null as number | null,
  amount: 0, vat_rate: 'none', contact_name: '', start_date: '', end_date: '',
})

function startAddContract(orgId: number) {
  editingContract.value = null
  Object.assign(contractForm, { contract_type: 'service', name: '', organization_id: orgId, amount: 0, vat_rate: 'none', contact_name: '', start_date: '', end_date: '' })
  showContractForm.value = true
}

function editContract(contract: Contract) {
  editingContract.value = contract
  Object.assign(contractForm, {
    contract_type: contract.contract_type, name: contract.name,
    organization_id: contract.organization_id, amount: contract.amount,
    vat_rate: contract.vat_rate, contact_name: contract.contact_name || '',
    start_date: contract.start_date || '', end_date: contract.end_date || '',
  })
  showContractForm.value = true
}

async function submitContract() {
  if (editingContract.value) {
    await store.updateContract(editingContract.value.id, { ...contractForm } as any)
  } else {
    await store.createContract({ ...contractForm } as any)
  }
  showContractForm.value = false
}

async function deleteContract(id: number) {
  deleteTarget.value = 'договор'
  deleteAction.value = () => store.deleteContract(id)
  showDeleteConfirm.value = true
}

// Invoice
const showInvoiceForm = ref(false)
const invoiceContractId = ref(0)
const invoiceContractName = ref('')
const invoiceContractAmount = ref(0)

function issueInvoice(contractId: number) {
  const contract = contracts.value.find(c => c.id === contractId)
  invoiceContractId.value = contractId
  invoiceContractName.value = contract?.name || ''
  invoiceContractAmount.value = contract?.amount || 0
  showInvoiceForm.value = true
}

async function handleCreateInvoice(items: any[]) {
  await store.createInvoice(invoiceContractId.value, items)
  showInvoiceForm.value = false
}

// Pay
const showPayForm = ref(false)
const payContractId = ref(0)
const payInvoiceId = ref(0)
const payDefaultAmount = ref(0)

function viewInvoices(contract: Contract) {
  // The InvoiceTable handles this internally now
}

function handlePayFromTable(payload: any) {
  payContractId.value = payload.contractId
  payInvoiceId.value = payload.invoiceId
  payDefaultAmount.value = payload.remainder
  showPayForm.value = true
}

async function handlePay(data: { amount: number; paid_at: string }) {
  await store.payInvoice(payContractId.value, payInvoiceId.value, data.amount, data.paid_at)
  showPayForm.value = false
}

// Delete confirm
const showDeleteConfirm = ref(false)
const deleteTarget = ref('')
const deleteAction = ref<() => Promise<void>>(() => Promise.resolve())

async function handleDelete() {
  await deleteAction.value()
  showDeleteConfirm.value = false
}

onMounted(() => fetchAll())
</script>

<style scoped>
.view-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  gap: 4px;
}

.pill-btn {
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 9999px;
  background: transparent;
  border: 1px solid var(--hairline);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.pill-btn:hover {
  border-color: var(--text-faint);
  color: var(--text-bright);
}

.pill-btn.active {
  background: var(--accent-bg);
  border-color: var(--accent);
  color: var(--accent);
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
}

.search-input {
  padding: 6px 12px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  min-width: 200px;
  margin-left: auto;
}

.loading {
  display: flex;
  justify-content: center;
  padding: 60px;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 4px;
}

.form-field input,
.form-field select {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-field input:focus,
.form-field select:focus {
  border-color: var(--accent);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
</style>
