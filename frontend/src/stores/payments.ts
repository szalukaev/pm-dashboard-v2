import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import i18n from '../i18n'

export interface Organization {
  id: number
  name: string
  address: string | null
  inn: string | null
  contact_name: string | null
  contact_phone: string | null
}

export interface Contract {
  id: number
  organization_id: number | null
  contract_type: string
  name: string
  company_name: string | null
  company_address: string | null
  amount: number
  vat_rate: string
  contact_name: string | null
  contact_phone: string | null
  start_date: string | null
  end_date: string | null
  org_name: string
  // The amount of the contract with VAT
  total_amount: number
  // The sum of the invoices, with VAT
  invoiced_amount: number
  total_paid: number
  // Invoiced and not paid yet
  debt_amount: number
  is_closed: boolean
  is_overdue: boolean
  is_fully_paid: boolean
}

export interface Invoice {
  id: number
  amount: number
  vat_rate: string
  issued_at: string
  paid_amount: number
  status: string
  total: number
  remainder: number
  // The date of issue as YYYYMM-DD
  number: string
  // The date of the last payment
  paid_at: string | null
}

// What the server answers to a payment: applied may be less than what was
// entered when it was more than the remainder (clamped).
export interface PaymentResult {
  paid_amount: number
  status: string
  applied: number
  clamped: boolean
}

export interface PaymentStat {
  label: string
  value: number
  variant?: string
}

export const usePaymentsStore = defineStore('payments', () => {
  const organizations = ref<Organization[]>([])
  const contracts = ref<Contract[]>([])
  const stats = ref<PaymentStat[]>([])
  const loading = ref(false)
  const error = ref('')

  // Grows whenever invoices change, so that open invoice tables re-read theirs
  const invoicesVersion = ref(0)

  const filterType = ref('')
  const showClosed = ref(false)
  const searchQuery = ref('')

  async function fetchAll() {
    loading.value = true
    error.value = ''
    try {
      await Promise.all([fetchOrganizations(), fetchContracts(), fetchStats()])
    } catch {
      error.value = i18n.global.t('common.error')
    } finally {
      loading.value = false
    }
  }

  async function fetchOrganizations() {
    try {
      const { data } = await axios.get('/api/organizations')
      organizations.value = data.organizations || []
    } catch {}
  }

  function contractParams(): Record<string, string> {
    const params: Record<string, string> = {}
    if (filterType.value) params.type = filterType.value
    if (showClosed.value) params.show_closed = 'true'
    if (searchQuery.value) params.search = searchQuery.value
    return params
  }

  async function fetchContracts() {
    try {
      const { data } = await axios.get('/api/contracts', { params: contractParams() })
      contracts.value = data.contracts || []
    } catch {}
  }

  async function fetchStats() {
    try {
      const { data } = await axios.get('/api/payments/stats')
      stats.value = data.stats || []
    } catch {}
  }

  async function createOrganization(org: Partial<Organization>) {
    const { data } = await axios.post('/api/organizations', org)
    await fetchOrganizations()
    return data
  }

  async function updateOrganization(id: number, fields: Partial<Organization>) {
    await axios.put(`/api/organizations/${id}`, fields)
    await refresh()
  }

  async function deleteOrganization(id: number) {
    await axios.delete(`/api/organizations/${id}`)
    await refresh()
  }

  async function createContract(contract: Partial<Contract>) {
    const { data } = await axios.post('/api/contracts', contract)
    await refresh()
    return data
  }

  async function updateContract(id: number, fields: Partial<Contract>) {
    await axios.put(`/api/contracts/${id}`, fields)
    await refresh()
  }

  async function deleteContract(id: number) {
    await axios.delete(`/api/contracts/${id}`)
    await refresh()
  }

  // One page of the invoices of a contract and how many there are in all
  async function fetchInvoices(contractId: number, limit: number, offset: number): Promise<{ invoices: Invoice[]; total: number }> {
    try {
      const { data } = await axios.get(`/api/contracts/${contractId}/invoices`, { params: { limit, offset } })
      return { invoices: data.invoices || [], total: data.total || 0 }
    } catch { return { invoices: [], total: 0 } }
  }

  async function createInvoice(contractId: number, items: { name: string; quantity: number; price: number }[]) {
    const { data } = await axios.post(`/api/contracts/${contractId}/invoices`, { items })
    invoicesVersion.value++
    await refresh()
    return data
  }

  async function deleteInvoice(contractId: number, invoiceId: number) {
    await axios.delete(`/api/contracts/${contractId}/invoices/${invoiceId}`)
    invoicesVersion.value++
    await refresh()
  }

  async function payInvoice(contractId: number, invoiceId: number, amount?: number, paidAt?: string): Promise<PaymentResult> {
    const body: any = {}
    if (amount) body.amount = amount
    if (paidAt) body.paid_at = paidAt
    const { data } = await axios.post(`/api/contracts/${contractId}/invoices/${invoiceId}/pay`, body)
    invoicesVersion.value++
    await refresh()
    return data
  }

  // Re-reads the figures quietly: the page stays in place, opened
  // organizations and contracts stay open
  async function refresh() {
    await Promise.all([fetchOrganizations(), fetchContracts(), fetchStats()])
  }

  function saveBlob(blob: Blob, name: string) {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  }

  // The export holds what the list shows: the same filters apply
  async function exportCSV() {
    const response = await axios.get('/api/contracts/export/csv', { params: contractParams(), responseType: 'blob' })
    saveBlob(response.data, 'contracts.csv')
  }

  // Downloads the .docx of an invoice under the name the server gives it
  async function downloadInvoice(contractId: number, invoiceId: number) {
    const response = await axios.get(`/api/contracts/${contractId}/invoices/${invoiceId}/download`, { responseType: 'blob' })
    const header = String(response.headers['content-disposition'] || '')
    const encoded = /filename\*=UTF-8''([^;]+)/i.exec(header)
    saveBlob(response.data, encoded ? decodeURIComponent(encoded[1]) : 'invoice.docx')
  }

  return {
    organizations, contracts, stats, loading, error,
    filterType, showClosed, searchQuery,
    fetchAll, fetchOrganizations, fetchContracts, fetchStats,
    createOrganization, updateOrganization, deleteOrganization,
    createContract, updateContract, deleteContract,
    fetchInvoices, createInvoice, deleteInvoice, payInvoice,
    exportCSV, downloadInvoice, invoicesVersion, refresh,
  }
})
