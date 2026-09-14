import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

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
  invoiced_amount: number
  total_paid: number
  debt_amount: number
  is_closed: boolean
  is_overdue: boolean
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

  const filterType = ref('')
  const showClosed = ref(false)
  const searchQuery = ref('')

  async function fetchAll() {
    loading.value = true
    error.value = ''
    try {
      await Promise.all([fetchOrganizations(), fetchContracts(), fetchStats()])
    } catch {
      error.value = 'Не удалось загрузить данные'
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

  async function fetchContracts() {
    try {
      const params: Record<string, string> = {}
      if (filterType.value) params.type = filterType.value
      if (showClosed.value) params.show_closed = 'true'
      if (searchQuery.value) params.search = searchQuery.value
      const { data } = await axios.get('/api/contracts', { params })
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
    await fetchAll()
  }

  async function deleteOrganization(id: number) {
    await axios.delete(`/api/organizations/${id}`)
    await fetchAll()
  }

  async function createContract(contract: Partial<Contract>) {
    const { data } = await axios.post('/api/contracts', contract)
    await fetchAll()
    return data
  }

  async function updateContract(id: number, fields: Partial<Contract>) {
    await axios.put(`/api/contracts/${id}`, fields)
    await fetchAll()
  }

  async function deleteContract(id: number) {
    await axios.delete(`/api/contracts/${id}`)
    await fetchAll()
  }

  async function fetchInvoices(contractId: number): Promise<Invoice[]> {
    try {
      const { data } = await axios.get(`/api/contracts/${contractId}/invoices`)
      return data.invoices || []
    } catch { return [] }
  }

  async function createInvoice(contractId: number, items: { name: string; quantity: number; price: number }[]) {
    const { data } = await axios.post(`/api/contracts/${contractId}/invoices`, { items })
    await fetchAll()
    return data
  }

  async function deleteInvoice(contractId: number, invoiceId: number) {
    await axios.delete(`/api/contracts/${contractId}/invoices/${invoiceId}`)
    await fetchAll()
  }

  async function payInvoice(contractId: number, invoiceId: number, amount?: number, paidAt?: string) {
    const body: any = {}
    if (amount) body.amount = amount
    if (paidAt) body.paid_at = paidAt
    const { data } = await axios.post(`/api/contracts/${contractId}/invoices/${invoiceId}/pay`, body)
    await fetchAll()
    return data
  }

  async function exportCSV() {
    const response = await axios.get('/api/contracts/export/csv', { responseType: 'blob' })
    const url = URL.createObjectURL(response.data)
    const a = document.createElement('a')
    a.href = url
    a.download = 'contracts.csv'
    a.click()
    URL.revokeObjectURL(url)
  }

  return {
    organizations, contracts, stats, loading, error,
    filterType, showClosed, searchQuery,
    fetchAll, fetchOrganizations, fetchContracts, fetchStats,
    createOrganization, updateOrganization, deleteOrganization,
    createContract, updateContract, deleteContract,
    fetchInvoices, createInvoice, deleteInvoice, payInvoice,
    exportCSV,
  }
})
