import type { InjectionKey } from 'vue'
import type { Invoice } from '../../stores/payments'

// What can be done with an invoice. The page provides the actions (it owns
// the forms and the confirmations), the table of invoices deep inside the
// accordions calls them.
export interface InvoiceActions {
  pay(contractId: number, invoice: Invoice): void
  download(contractId: number, invoice: Invoice): void
  remove(contractId: number, invoice: Invoice): void
}

export const invoiceActionsKey: InjectionKey<InvoiceActions> = Symbol('invoiceActions')
