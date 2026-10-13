import type { InjectionKey } from 'vue'
import type { Invoice } from '../../stores/payments'

// Что можно сделать со счётом. Действия предоставляет страница (формы и
// подтверждения — её), а вызывает их таблица счетов глубоко внутри
// раскрывающихся блоков.
export interface InvoiceActions {
  pay(contractId: number, invoice: Invoice): void
  download(contractId: number, invoice: Invoice): void
  // Открывает форму нового счёта, заполненную позициями этого
  copy(contractId: number, invoice: Invoice): void
  remove(contractId: number, invoice: Invoice): void
}

export const invoiceActionsKey: InjectionKey<InvoiceActions> = Symbol('invoiceActions')
