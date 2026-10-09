package handlers

import (
	"time"

	"pm-dashboard/utils"
)

// The arithmetic of the Payments tab, in one place: the list of contracts,
// the statistics and the CSV export must agree with each other.

// vatMultiplier turns an amount without VAT into the amount to pay.
// Two modes exist: "none" and "5" (5 %).
func vatMultiplier(rate string) float64 {
	if rate == "5" {
		return 1.05
	}
	return 1
}

// contractMoney is what the state of a contract is computed from.
type contractMoney struct {
	Type    string  // "service" | "onetime"
	Amount  float64 // the amount of the contract, without VAT
	VatRate string
	// Invoiced: the sum of its invoices, with VAT of each invoice.
	Invoiced float64
	// Paid: everything paid on its invoices.
	Paid    float64
	EndDate *string // YYYY-MM-DD…
}

// Total is the amount of the contract with VAT.
func (m contractMoney) Total() float64 {
	return utils.Round2(m.Amount * vatMultiplier(m.VatRate))
}

// Debt is what is invoiced and not paid yet. What has not been invoiced is
// not a debt.
func (m contractMoney) Debt() float64 {
	debt := utils.Round2(m.Invoiced - m.Paid)
	if debt < 0 {
		return 0
	}
	return debt
}

// FullyPaid: the whole amount of the contract is paid.
func (m contractMoney) FullyPaid() bool {
	return m.Paid > 0 && m.Paid >= m.Total()
}

func (m contractMoney) endsBefore(day string, inclusive bool) bool {
	if m.EndDate == nil || len(*m.EndDate) < 10 {
		return false
	}
	end := (*m.EndDate)[:10]
	return end < day || (inclusive && end == day)
}

// Closed: nothing more is expected of the contract. A one-time contract is
// closed when it is fully paid; a service contract — when its term has come
// and nothing is owed. A service contract whose term is in the future, or
// that has no term, is never closed.
func (m contractMoney) Closed(today string) bool {
	if m.Type == "onetime" {
		return m.FullyPaid()
	}
	return m.endsBefore(today, true) && m.Debt() <= 0
}

// Overdue: the term has passed and the contract is neither fully paid nor
// closed.
func (m contractMoney) Overdue(today string) bool {
	return m.endsBefore(today, false) && !m.FullyPaid() && !m.Closed(today)
}

// Statuses of a contract in the export.
const (
	contractStatusPaid    = "Оплачен"
	contractStatusOverdue = "Просрочен"
	contractStatusActive  = "Активен"
)

// ExportStatus is the status of the contract in the CSV export.
func (m contractMoney) ExportStatus(today string) string {
	switch {
	case m.FullyPaid():
		return contractStatusPaid
	case m.Overdue(today):
		return contractStatusOverdue
	default:
		return contractStatusActive
	}
}

// paymentTotals are the figures of the cards above the list: always over
// every contract of the user, whatever the filters show.
type paymentTotals struct {
	Contracts    int
	TotalAmount  float64 // with VAT
	Invoiced     float64 // with VAT
	Debt         float64
	Paid         float64
	ClosedAmount float64 // the amounts of the contracts that are fully paid or closed
}

func sumContracts(contracts []contractMoney, today string) paymentTotals {
	var t paymentTotals
	for _, c := range contracts {
		t.Contracts++
		t.TotalAmount += c.Total()
		t.Invoiced += c.Invoiced
		t.Debt += c.Debt()
		t.Paid += c.Paid
		if c.FullyPaid() || c.Closed(today) {
			t.ClosedAmount += c.Total()
		}
	}
	t.TotalAmount, t.Invoiced = utils.Round2(t.TotalAmount), utils.Round2(t.Invoiced)
	t.Debt, t.Paid, t.ClosedAmount = utils.Round2(t.Debt), utils.Round2(t.Paid), utils.Round2(t.ClosedAmount)
	return t
}

// appliedPayment is how much of an entered payment is taken: never more
// than what is left to pay on the invoice. ok is false for an amount that
// cannot be a payment, or when nothing is left to pay.
func appliedPayment(entered *float64, remainder float64) (applied float64, clamped, ok bool) {
	remainder = utils.Round2(remainder)
	if remainder <= 0 {
		return 0, false, false
	}
	if entered == nil {
		// No amount given: the whole remainder is paid
		return remainder, false, true
	}
	amount := utils.Round2(*entered)
	if amount <= 0 {
		return 0, false, false
	}
	if amount > remainder {
		return remainder, true, true
	}
	return amount, false, true
}

// invoiceNumber is the number of an invoice: the date it was issued on as
// YYYYMM-DD. It is not a counter.
func invoiceNumber(issuedAt string) string {
	t, err := time.Parse("2006-01-02", firstN(issuedAt, 10))
	if err != nil {
		return ""
	}
	return t.Format("200601-02")
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
