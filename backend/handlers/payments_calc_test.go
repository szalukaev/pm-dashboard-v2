package handlers

import "testing"

func strPtr(s string) *string { return &s }

func TestContractMoneyFromTheSpec(t *testing.T) {
	// The example of the specification: a contract for 100 000 with 5 % VAT,
	// 52 500 invoiced, 31 500 paid
	c := contractMoney{Type: "service", Amount: 100000, VatRate: "5", Invoiced: 52500, Paid: 31500, EndDate: strPtr("2025-12-31")}
	if c.Total() != 105000 {
		t.Errorf("total = %v, want 105000", c.Total())
	}
	if c.Debt() != 21000 {
		t.Errorf("debt = %v, want 21000 (invoiced minus paid, not the whole contract)", c.Debt())
	}
	if c.FullyPaid() {
		t.Error("31 500 of 105 000 is not fully paid")
	}
}

func TestContractClosedAndOverdue(t *testing.T) {
	const today = "2026-10-10"
	cases := []struct {
		name            string
		c               contractMoney
		closed, overdue bool
		status          string
	}{
		{"one-time, fully paid",
			contractMoney{Type: "onetime", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 1000}, true, false, contractStatusPaid},
		{"one-time, nothing paid",
			contractMoney{Type: "onetime", Amount: 1000, VatRate: "none", Invoiced: 1000}, false, false, contractStatusActive},
		{"one-time for nothing is not closed by paying nothing",
			contractMoney{Type: "onetime", Amount: 0, VatRate: "none"}, false, false, contractStatusActive},
		{"one-time with VAT: the amount without VAT is not enough",
			contractMoney{Type: "onetime", Amount: 1000, VatRate: "5", Invoiced: 1050, Paid: 1000}, false, false, contractStatusActive},
		{"service, term passed, nothing owed",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 400, Paid: 400, EndDate: strPtr("2026-10-01")}, true, false, contractStatusActive},
		{"service, term is today, nothing owed",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 1000, EndDate: strPtr("2026-10-10")}, true, false, contractStatusPaid},
		{"service, term passed, a debt",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 400, EndDate: strPtr("2026-10-01")}, false, true, contractStatusOverdue},
		{"service, term in the future, nothing owed: not closed",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 1000, EndDate: strPtr("2027-01-01")}, false, false, contractStatusPaid},
		{"service, term in the future, a debt: active",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 500, EndDate: strPtr("2027-01-01")}, false, false, contractStatusActive},
		{"service without a term is never closed",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 1000}, false, false, contractStatusPaid},
		{"the date may come with a time part",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 400, EndDate: strPtr("2026-10-01T00:00:00Z")}, false, true, contractStatusOverdue},
		{"term is today: not overdue yet",
			contractMoney{Type: "service", Amount: 1000, VatRate: "none", Invoiced: 1000, Paid: 400, EndDate: strPtr("2026-10-10")}, false, false, contractStatusActive},
	}
	for _, c := range cases {
		if got := c.c.Closed(today); got != c.closed {
			t.Errorf("%s: closed = %v, want %v", c.name, got, c.closed)
		}
		if got := c.c.Overdue(today); got != c.overdue {
			t.Errorf("%s: overdue = %v, want %v", c.name, got, c.overdue)
		}
		if got := c.c.ExportStatus(today); got != c.status {
			t.Errorf("%s: status = %q, want %q", c.name, got, c.status)
		}
	}
}

func TestSumContracts(t *testing.T) {
	contracts := []contractMoney{
		{Type: "service", Amount: 100000, VatRate: "5", Invoiced: 52500, Paid: 31500, EndDate: strPtr("2027-12-31")},
		{Type: "onetime", Amount: 20000, VatRate: "none", Invoiced: 20000, Paid: 20000},
		{Type: "onetime", Amount: 5000, VatRate: "none"},
	}
	got := sumContracts(contracts, "2026-10-10")
	want := paymentTotals{Contracts: 3, TotalAmount: 130000, Invoiced: 72500, Debt: 21000, Paid: 51500, ClosedAmount: 20000}
	if got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}
}

func TestAppliedPayment(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name      string
		entered   *float64
		remainder float64
		applied   float64
		clamped   bool
		ok        bool
	}{
		{"no amount: the whole remainder", nil, 1050, 1050, false, true},
		{"a part", f(400), 1050, 400, false, true},
		{"exactly the remainder", f(1050), 1050, 1050, false, true},
		{"more than the remainder is cut to it", f(5000), 1050, 1050, true, true},
		{"zero is not a payment", f(0), 1050, 0, false, false},
		{"a negative amount is not a payment", f(-10), 1050, 0, false, false},
		{"nothing left to pay", f(100), 0, 0, false, false},
		{"kopecks are rounded", f(100.004), 1050, 100, false, true},
	}
	for _, c := range cases {
		applied, clamped, ok := appliedPayment(c.entered, c.remainder)
		if applied != c.applied || clamped != c.clamped || ok != c.ok {
			t.Errorf("%s: got (%v, %v, %v), want (%v, %v, %v)", c.name, applied, clamped, ok, c.applied, c.clamped, c.ok)
		}
	}
}

func TestInvoiceNumber(t *testing.T) {
	cases := map[string]string{
		"2025-01-15":           "202501-15",
		"2026-10-09T00:00:00Z": "202610-09",
		"":                     "",
		"not a date":           "",
	}
	for issued, want := range cases {
		if got := invoiceNumber(issued); got != want {
			t.Errorf("invoice issued at %q: number %q, want %q", issued, got, want)
		}
	}
}

func TestContractsCSV(t *testing.T) {
	const today = "2026-10-10"
	contract := func(name, kind string, amount float64, vat string, invoiced, paid float64, end *string) paymentContract {
		c := paymentContract{Name: name, ContractType: kind, Amount: amount, VatRate: vat,
			InvoicedAmount: invoiced, TotalPaid: paid, EndDate: end, CompanyName: strPtr("ООО Ромашка")}
		c.money = contractMoney{Type: kind, Amount: amount, VatRate: vat, Invoiced: invoiced, Paid: paid, EndDate: end}
		c.TotalAmount, c.DebtAmount = c.money.Total(), c.money.Debt()
		return c
	}
	withOrg := contract("Сопровождение", "service", 100000, "5", 52500, 31500, strPtr("2026-09-30T00:00:00Z"))
	withOrg.OrgName = "ООО Рога и Копыта"
	rows := contractsCSV([]paymentContract{
		withOrg,
		contract("Разработка", "onetime", 20000, "none", 20000, 20000, nil),
	}, today)

	wantHeader := []string{"Тип", "Название", "Организация", "Адрес", "Сумма", "НДС", "Выставлено", "Долг",
		"Контакт", "Телефон", "Дата начала", "Срок", "Статус"}
	if len(rows) != 3 || len(rows[0]) != len(wantHeader) {
		t.Fatalf("rows = %v", rows)
	}
	for i, name := range wantHeader {
		if rows[0][i] != name {
			t.Errorf("column %d = %q, want %q", i, rows[0][i], name)
		}
	}
	// Amounts with VAT and a decimal comma; the organization the contract
	// belongs to; an expired contract with a debt is overdue
	want := []string{"Сопровождение", "Сопровождение", "ООО Рога и Копыта", "", "105000,00", "НДС 5%", "52500,00", "21000,00",
		"", "", "", "2026-09-30", "Просрочен"}
	for i, value := range want {
		if rows[1][i] != value {
			t.Errorf("first contract, column %q = %q, want %q", wantHeader[i], rows[1][i], value)
		}
	}
	// Without an organization the name kept in the contract is used; a
	// fully paid contract is "paid"
	if rows[2][2] != "ООО Ромашка" || rows[2][5] != "Без НДС" || rows[2][12] != "Оплачен" {
		t.Errorf("second contract = %v", rows[2])
	}
}
