package handlers

import (
	"time"

	"pm-dashboard/utils"
)

// Арифметика вкладки «Оплаты» в одном месте: список договоров, статистика
// и выгрузка в CSV должны сходиться друг с другом.

// vatMultiplier переводит сумму без НДС в сумму к оплате. Режимов два:
// "none" и "5" (5 %).
func vatMultiplier(rate string) float64 {
	if rate == "5" {
		return 1.05
	}
	return 1
}

// contractMoney — то, из чего считается состояние договора.
type contractMoney struct {
	Type    string  // "service" | "onetime"
	Amount  float64 // сумма договора без НДС
	VatRate string
	// Invoiced: сумма его счетов, с НДС каждого счёта.
	Invoiced float64
	// Paid: всё, что оплачено по его счетам.
	Paid    float64
	EndDate *string // YYYY-MM-DD…
}

// Total — сумма договора с НДС.
func (m contractMoney) Total() float64 {
	return utils.Round2(m.Amount * vatMultiplier(m.VatRate))
}

// Debt — выставленное и ещё не оплаченное. То, на что счёт не выставлен,
// долгом не считается.
func (m contractMoney) Debt() float64 {
	debt := utils.Round2(m.Invoiced - m.Paid)
	if debt < 0 {
		return 0
	}
	return debt
}

// FullyPaid: оплачена вся сумма договора.
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

// Closed: по договору больше ничего не ожидается. Разовый договор закрыт,
// когда полностью оплачен; договор сопровождения — когда его срок наступил
// и долга нет. Договор сопровождения со сроком в будущем или без срока
// закрытым не бывает.
func (m contractMoney) Closed(today string) bool {
	if m.Type == "onetime" {
		return m.FullyPaid()
	}
	return m.endsBefore(today, true) && m.Debt() <= 0
}

// Overdue: срок прошёл, а договор не оплачен полностью и не закрыт.
func (m contractMoney) Overdue(today string) bool {
	return m.endsBefore(today, false) && !m.FullyPaid() && !m.Closed(today)
}

// Статусы договора в выгрузке.
const (
	contractStatusPaid    = "Оплачен"
	contractStatusOverdue = "Просрочен"
	contractStatusActive  = "Активен"
)

// ExportStatus — статус договора в выгрузке CSV.
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

// paymentTotals — цифры карточек над списком: всегда по всем договорам
// пользователя, что бы ни показывали фильтры.
type paymentTotals struct {
	Contracts    int
	TotalAmount  float64 // с НДС
	Invoiced     float64 // с НДС
	Debt         float64
	Paid         float64
	ClosedAmount float64 // суммы договоров, оплаченных полностью или закрытых
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

// appliedPayment — сколько из введённой оплаты принимается: не больше
// остатка по счёту. ok равен false для суммы, которая не может быть
// оплатой, или когда платить уже нечего.
func appliedPayment(entered *float64, remainder float64) (applied float64, clamped, ok bool) {
	remainder = utils.Round2(remainder)
	if remainder <= 0 {
		return 0, false, false
	}
	if entered == nil {
		// Сумма не указана: оплачивается весь остаток
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

// invoiceNumber — номер счёта: дата выставления в виде YYYYMM-DD. Это не
// счётчик.
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
