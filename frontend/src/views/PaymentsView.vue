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
      />
    </template>

    <!-- Organization -->
    <AppModal :model-value="showOrgForm" :title="$t('payments.org_form.title')" width="480px" @update:model-value="showOrgForm = $event">
      <form class="modal-form" @submit.prevent="submitOrg">
        <div class="form-field">
          <label>{{ $t('payments.org_form.name') }} *</label>
          <input v-model="orgForm.name" type="text" required data-testid="org-name" />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.org_form.address') }}</label>
          <input v-model="orgForm.address" type="text" />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.org_form.inn') }}</label>
          <input v-model="orgForm.inn" type="text" />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.org_form.contact') }}</label>
          <input v-model="orgForm.contact_name" type="text" />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.org_form.phone') }}</label>
          <input v-model="orgForm.contact_phone" type="text" />
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showOrgForm = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" @click="submitOrg" :disabled="!orgForm.name.trim()">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>

    <!-- Contract -->
    <AppModal :model-value="showContractForm" :title="$t('payments.contract_form.title')" width="520px" @update:model-value="showContractForm = $event">
      <form class="modal-form" @submit.prevent="submitContract">
        <div class="form-field">
          <label>{{ $t('payments.contract_form.type') }} *</label>
          <select v-model="contractForm.contract_type" data-testid="contract-type">
            <option value="service">{{ $t('payments.contract_types.service') }}</option>
            <option value="onetime">{{ $t('payments.contract_types.onetime') }}</option>
          </select>
        </div>
        <div class="form-field">
          <label>{{ $t('payments.contract_form.name') }} *</label>
          <input v-model="contractForm.name" type="text" required data-testid="contract-name" />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.contract_form.organization') }}</label>
          <input :value="contractOrgName" type="text" disabled />
        </div>
        <div class="form-field">
          <label>{{ $t('payments.contract_form.address') }}</label>
          <input v-model="contractForm.company_address" type="text" />
        </div>
        <div class="form-row">
          <div class="form-field">
            <label>{{ $t('payments.contract_form.amount') }}</label>
            <input v-model.number="contractForm.amount" type="number" min="0" step="0.01" />
          </div>
          <div class="form-field">
            <label>{{ $t('payments.contract_form.vat') }}</label>
            <select v-model="contractForm.vat_rate">
              <option value="none">{{ $t('payments.contract_form.vat_none') }}</option>
              <option value="5">{{ $t('payments.contract_form.vat_5') }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="form-field">
            <label>{{ $t('payments.contract_form.contact') }} *</label>
            <input v-model="contractForm.contact_name" type="text" required data-testid="contract-contact" />
          </div>
          <div class="form-field">
            <label>{{ $t('payments.contract_form.phone') }}</label>
            <input v-model="contractForm.contact_phone" type="text" />
          </div>
        </div>
        <div class="form-row">
          <!-- A one-time contract has no start date -->
          <div class="form-field" v-if="contractForm.contract_type === 'service'">
            <label>{{ $t('payments.contract_form.start_date') }}</label>
            <input v-model="contractForm.start_date" type="date" />
          </div>
          <div class="form-field">
            <label>{{ $t('payments.contract_form.end_date') }}</label>
            <input v-model="contractForm.end_date" type="date" />
          </div>
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showContractForm = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" test-id="contract-save" @click="submitContract" :disabled="!canSaveContract">{{ $t('common.save') }}</AppButton>
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
      :title="deleteTitle"
      :message="deleteMessage"
      @update:model-value="showDeleteConfirm = $event"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, provide, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { Download } from 'lucide-vue-next'
import { usePaymentsStore, type Organization, type Contract, type Invoice } from '../stores/payments'
import { useSwal } from '../composables/useSwal'
import { formatMoney } from '../utils/format'
import { invoiceActionsKey } from '../components/payments/invoiceActions'
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
const { fetchAll } = store
const { t } = useI18n()
const { toast } = useSwal()

// Whatever fails on this page is told to the user instead of failing quietly
async function guarded(action: () => Promise<unknown>, failure = 'payments.errors.action_failed'): Promise<boolean> {
  try {
    await action()
    return true
  } catch {
    toast(t(failure), 'error')
    return false
  }
}

function exportCSV() {
  guarded(() => store.exportCSV(), 'payments.errors.export_failed')
}

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

// Organization form
const showOrgForm = ref(false)
const editingOrg = ref<Organization | null>(null)
const orgForm = reactive({ name: '', address: '', inn: '', contact_name: '', contact_phone: '' })

function editOrg(org: Organization) {
  editingOrg.value = org
  Object.assign(orgForm, { name: org.name, address: org.address || '', inn: org.inn || '', contact_name: org.contact_name || '', contact_phone: org.contact_phone || '' })
  showOrgForm.value = true
}

async function submitOrg() {
  if (!orgForm.name.trim()) return
  const editing = editingOrg.value
  const saved = await guarded(() => (editing ? store.updateOrganization(editing.id, { ...orgForm }) : store.createOrganization({ ...orgForm })))
  if (!saved) return
  showOrgForm.value = false
  editingOrg.value = null
  Object.assign(orgForm, { name: '', address: '', inn: '', contact_name: '', contact_phone: '' })
}

function deleteOrg(id: number) {
  askDelete(t('payments.delete.org_title'), t('payments.delete.org_text'), () => store.deleteOrganization(id))
}

// Contract form
const showContractForm = ref(false)
const editingContract = ref<Contract | null>(null)
const emptyContract = {
  contract_type: 'service', name: '', organization_id: null as number | null,
  company_address: '', amount: 0, vat_rate: 'none', contact_name: '', contact_phone: '',
  start_date: '', end_date: '',
}
const contractForm = reactive({ ...emptyContract })

const contractOrgName = computed(() =>
  organizations.value.find(o => o.id === contractForm.organization_id)?.name || t('payments.without_org')
)
// The name and the contact person are required
const canSaveContract = computed(() => contractForm.name.trim() !== '' && contractForm.contact_name.trim() !== '')

// A new contract takes the address and the contact of its organization
function startAddContract(orgId: number) {
  const org = organizations.value.find(o => o.id === orgId)
  editingContract.value = null
  Object.assign(contractForm, {
    ...emptyContract,
    organization_id: orgId,
    company_address: org?.address || '',
    contact_name: org?.contact_name || '',
    contact_phone: org?.contact_phone || '',
  })
  showContractForm.value = true
}

function editContract(contract: Contract) {
  editingContract.value = contract
  Object.assign(contractForm, {
    contract_type: contract.contract_type, name: contract.name,
    organization_id: contract.organization_id,
    company_address: contract.company_address || '',
    amount: contract.amount, vat_rate: contract.vat_rate,
    contact_name: contract.contact_name || '', contact_phone: contract.contact_phone || '',
    start_date: (contract.start_date || '').slice(0, 10), end_date: (contract.end_date || '').slice(0, 10),
  })
  showContractForm.value = true
}

async function submitContract() {
  if (!canSaveContract.value) return
  // A date left blank is "not set"; a one-time contract has no start date
  const fields = {
    ...contractForm,
    name: contractForm.name.trim(),
    amount: Number(contractForm.amount) || 0,
    start_date: contractForm.contract_type === 'service' ? contractForm.start_date || null : null,
    end_date: contractForm.end_date || null,
  }
  const editing = editingContract.value
  const saved = await guarded(() => (editing ? store.updateContract(editing.id, fields as any) : store.createContract(fields as any)))
  if (saved) showContractForm.value = false
}

function deleteContract(id: number) {
  askDelete(t('payments.delete.contract_title'), t('payments.delete.contract_text'), () => store.deleteContract(id))
}

// Issuing an invoice
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

// The invoice is saved, and its document is downloaded at once
async function handleCreateInvoice(items: any[]) {
  const contractId = invoiceContractId.value
  let invoiceId = 0
  const created = await guarded(async () => {
    const data = await store.createInvoice(contractId, items)
    invoiceId = data?.id || 0
  })
  if (!created) return
  showInvoiceForm.value = false
  if (invoiceId) await guarded(() => store.downloadInvoice(contractId, invoiceId), 'payments.errors.download_failed')
}

// Paying an invoice
const showPayForm = ref(false)
const payContractId = ref(0)
const payInvoiceId = ref(0)
const payDefaultAmount = ref(0)

async function handlePay(data: { amount: number; paid_at: string }) {
  await guarded(async () => {
    const result = await store.payInvoice(payContractId.value, payInvoiceId.value, data.amount, data.paid_at)
    showPayForm.value = false
    // More than was left to pay: the user is told how much was taken
    if (result?.clamped) {
      toast(t('payments.invoice.clamped', { applied: formatMoney(result.applied), entered: formatMoney(data.amount) }), 'info')
    } else {
      toast(t('payments.invoice.paid_ok'), 'success')
    }
  }, 'payments.errors.pay_failed')
}

// The table of invoices sits deep inside the accordions; what it does with
// an invoice is done here
provide(invoiceActionsKey, {
  pay(contractId: number, invoice: Invoice) {
    payContractId.value = contractId
    payInvoiceId.value = invoice.id
    payDefaultAmount.value = invoice.remainder
    showPayForm.value = true
  },
  download(contractId: number, invoice: Invoice) {
    guarded(() => store.downloadInvoice(contractId, invoice.id), 'payments.errors.download_failed')
  },
  remove(contractId: number, invoice: Invoice) {
    const text = invoice.paid_amount > 0
      ? t('payments.delete.invoice_paid_text', { paid: formatMoney(invoice.paid_amount) })
      : t('payments.delete.invoice_text')
    askDelete(t('payments.delete.invoice_title', { number: invoice.number }), text, () => store.deleteInvoice(contractId, invoice.id))
  },
})

// Delete confirmation
const showDeleteConfirm = ref(false)
const deleteTitle = ref('')
const deleteMessage = ref('')
const deleteAction = ref<() => Promise<unknown>>(() => Promise.resolve())

function askDelete(title: string, message: string, action: () => Promise<unknown>) {
  deleteTitle.value = title
  deleteMessage.value = message
  deleteAction.value = action
  showDeleteConfirm.value = true
}

async function handleDelete() {
  await guarded(deleteAction.value, 'payments.errors.delete_failed')
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
