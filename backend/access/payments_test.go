package access

import "testing"

func TestCanPayment(t *testing.T) {
	// Без настройки разрешены все действия
	open := &Scope{}
	for _, a := range PaymentActions {
		if !open.CanPayment(a) {
			t.Errorf("no setting: %q must be allowed", a)
		}
	}

	viewOnly := []string{PaymentDownload}
	limited := &Scope{PaymentActions: &viewOnly}
	if !limited.CanPayment(PaymentDownload) || limited.CanPayment(PaymentContractDelete) {
		t.Error("only downloading must be allowed")
	}

	// Администратора настройка не ограничивает
	admin := &Scope{Admin: true, PaymentActions: &viewOnly}
	if !admin.CanPayment(PaymentContractDelete) {
		t.Error("an administrator is never limited")
	}
}

func TestPaymentsOwner(t *testing.T) {
	// «Только свои» — фильтр по автору записи
	own := &Scope{UserID: 7, PaymentsOwnOnly: true}
	if id := own.PaymentsOwner(); id == nil || *id != 7 {
		t.Errorf("own only: owner = %v, want 7", id)
	}
	if (&Scope{UserID: 7}).PaymentsOwner() != nil {
		t.Error("shared payments: no owner filter")
	}
	if (&Scope{UserID: 7, Admin: true, PaymentsOwnOnly: true}).PaymentsOwner() != nil {
		t.Error("an administrator sees every record")
	}
}

func TestPaymentsEffective(t *testing.T) {
	yes, no := true, false
	ownOnly := func(p Permissions) bool { return p.PaymentsOwnOnly != nil && *p.PaymentsOwnOnly }

	// Не задано нигде — только свои, как было до настройки
	if !ownOnly(effective(nil, nil, nil)) {
		t.Error("default set: payments must stay own only")
	}
	shared := Permissions{PaymentsOwnOnly: &no}
	if ownOnly(effective(&shared, nil, nil)) {
		t.Error("personal rights open every record")
	}

	// Группа без ограничения открывает все записи, даже при «только свои» в
	// личных правах
	if ownOnly(effective(&Permissions{PaymentsOwnOnly: &yes}, nil, []Permissions{{}})) {
		t.Error("a group without the restriction opens every record")
	}
	// При группах решают они
	if !ownOnly(effective(&shared, nil, []Permissions{{PaymentsOwnOnly: &yes}})) {
		t.Error("a group sets own only")
	}
	// Несколько групп складываются: хватает одной, открывшей все записи
	both := []Permissions{{PaymentsOwnOnly: &yes}, {PaymentsOwnOnly: &no}}
	if ownOnly(effective(nil, nil, both)) {
		t.Error("groups add up: one group opening every record is enough")
	}
	// Флаг группы не меняется при объединении
	if !*both[0].PaymentsOwnOnly {
		t.Error("merging must not modify a group's own value")
	}

	// Действия складываются по правилу вкладок
	create := []string{PaymentContractCreate, "bogus"}
	pay := []string{PaymentInvoicePay}
	got := effective(&shared, nil, []Permissions{{PaymentActions: &create}, {PaymentActions: &pay}}).PaymentActions
	if got == nil || len(*got) != 2 {
		t.Errorf("payment actions of two groups = %v", got)
	}
	got = effective(&shared, nil, []Permissions{{PaymentActions: &create}}).PaymentActions
	if got == nil || len(*got) != 1 || (*got)[0] != PaymentContractCreate {
		t.Errorf("payment actions = %v", got)
	}
}
