package notify

import (
	"reflect"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func hours(v float64) *float64 { return &v }

func TestChanges(t *testing.T) {
	base := IssueState{ID: 1, StatusID: 1, StatusName: "Новая", PriorityID: 3, PriorityName: "Нормальный", Due: "2026-10-20", Estimate: hours(4)}

	if got := changes(base, base); len(got) != 0 {
		t.Errorf("без изменений: %v", got)
	}

	cur := base
	cur.StatusID, cur.StatusName = 2, "В работе"
	if got, want := changes(base, cur), []Change{{Field: "status", From: "Новая", To: "В работе"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("статус: %v", got)
	}

	cur = base
	cur.PriorityID, cur.PriorityName = 5, "Срочный"
	if got, want := changes(base, cur), []Change{{Field: "priority", From: "Нормальный", To: "Срочный"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("приоритет: %v", got)
	}

	cur = base
	cur.Due = ""
	if got, want := changes(base, cur), []Change{{Field: "due_date", From: "2026-10-20", To: ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("срок снят: %v", got)
	}

	cur = base
	cur.Estimate = hours(6.5)
	if got, want := changes(base, cur), []Change{{Field: "estimated_hours", From: "4", To: "6.5"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("оценка: %v", got)
	}

	cur = base
	cur.Estimate = nil
	if got := changes(base, cur); len(got) != 1 || got[0].To != "" {
		t.Errorf("оценка снята: %v", got)
	}

	// Переименование статуса без смены самого статуса — не изменение задачи
	cur = base
	cur.StatusName = "New"
	if got := changes(base, cur); len(got) != 0 {
		t.Errorf("переименование статуса: %v", got)
	}

	// Смена ответственного — отдельное событие, в изменения полей не входит
	cur = base
	cur.AssignedTo = intPtr(7)
	if got := changes(base, cur); len(got) != 0 {
		t.Errorf("смена ответственного: %v", got)
	}

	// Несколько полей сразу — в постоянном порядке
	cur = base
	cur.StatusID, cur.Due = 2, "2026-11-01"
	if got := changes(base, cur); len(got) != 2 || got[0].Field != "status" || got[1].Field != "due_date" {
		t.Errorf("два поля: %v", got)
	}
}

func TestAssignedTo(t *testing.T) {
	cases := []struct {
		name    string
		prev    *int
		hasPrev bool
		cur     *int
		member  int
		ok      bool
	}{
		{"назначили впервые", nil, true, intPtr(5), 5, true},
		{"переназначили другому", intPtr(4), true, intPtr(5), 5, true},
		{"ответственный прежний", intPtr(5), true, intPtr(5), 0, false},
		{"ответственного сняли", intPtr(5), true, nil, 0, false},
		{"новая задача с ответственным", nil, false, intPtr(5), 5, true},
		{"новая задача без ответственного", nil, false, nil, 0, false},
	}
	for _, c := range cases {
		member, ok := assignedTo(IssueState{AssignedTo: c.prev}, c.hasPrev, IssueState{AssignedTo: c.cur})
		if member != c.member || ok != c.ok {
			t.Errorf("%s: получено %d, %v", c.name, member, ok)
		}
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		from, to string
		days     int
		ok       bool
	}{
		{"2026-10-13", "2026-10-13", 0, true},
		{"2026-10-13", "2026-10-16", 3, true},
		{"2026-10-13", "2026-10-10", -3, true},
		{"2026-10-30", "2026-11-02", 3, true},
		{"2026-12-31", "2027-01-01", 1, true},
		{"2026-10-13", "", 0, false},
		{"вчера", "2026-10-13", 0, false},
	}
	for _, c := range cases {
		days, ok := daysBetween(c.from, c.to)
		if days != c.days || ok != c.ok {
			t.Errorf("daysBetween(%q, %q) = %d, %v", c.from, c.to, days, ok)
		}
	}
}

func TestDedupeKeys(t *testing.T) {
	// Просрочка напоминает раз в день: в один день ключ один, назавтра другой
	if overdueKey(10, "2026-10-13") != overdueKey(10, "2026-10-13") {
		t.Error("ключ просрочки за один день должен совпадать")
	}
	if overdueKey(10, "2026-10-13") == overdueKey(10, "2026-10-14") {
		t.Error("на следующий день просрочка должна напомнить снова")
	}
	if overdueKey(10, "2026-10-13") == overdueKey(11, "2026-10-13") {
		t.Error("у разных задач разные ключи")
	}

	// «Скоро срок» — один раз на срок; перенесли срок — напомнит снова
	if dueSoonKey(10, "2026-10-20") != dueSoonKey(10, "2026-10-20") {
		t.Error("ключ для одного срока должен совпадать")
	}
	if dueSoonKey(10, "2026-10-20") == dueSoonKey(10, "2026-10-25") {
		t.Error("новый срок — новое напоминание")
	}

	// Назначение и изменение случаются сами по себе один раз: каждый случай отдельный
	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	if eventKey(RuleTaskChanged, 10, at) == eventKey(RuleTaskChanged, 10, at.Add(time.Minute)) {
		t.Error("два изменения одной задачи — два уведомления")
	}
	if eventKey(RuleTaskChanged, 10, at) == eventKey(RuleTaskAssigned, 10, at) {
		t.Error("у разных правил разные ключи")
	}
}

func TestEffectiveRules(t *testing.T) {
	// Ничего не настроено — правила по умолчанию
	rules := Effective(nil)
	if !rules[RuleTaskAssigned].Bell || rules[RuleTaskDueSoon].Days != 3 || rules[RuleTaskAssigned].Email {
		t.Errorf("по умолчанию: %+v", rules)
	}
	if len(rules) != len(Keys()) {
		t.Errorf("правил %d, ключей %d", len(rules), len(Keys()))
	}

	// Сохранённое правило заменяет своё, остальные остаются по умолчанию
	rules = Effective([]byte(`{"task_due_soon": {"bell": false, "email": true, "days": 7}}`))
	if got := rules[RuleTaskDueSoon]; got.Bell || !got.Email || got.Days != 7 {
		t.Errorf("сохранённое правило: %+v", got)
	}
	if !rules[RuleTaskOverdue].Bell {
		t.Error("ненастроенное правило должно остаться по умолчанию")
	}

	// Неизвестное правило и негодные дни не проходят
	rules = Effective([]byte(`{"nonsense": {"bell": true}, "task_due_soon": {"bell": true, "days": 900}}`))
	if _, has := rules["nonsense"]; has {
		t.Error("неизвестное правило не должно появляться")
	}
	if rules[RuleTaskDueSoon].Days != 3 {
		t.Errorf("негодные дни заменяются стандартными: %d", rules[RuleTaskDueSoon].Days)
	}

	// Испорченные настройки — правила по умолчанию
	if got := Effective([]byte(`[1, 2]`)); !reflect.DeepEqual(got, Defaults()) {
		t.Errorf("испорченные настройки: %+v", got)
	}
}

func TestValidRules(t *testing.T) {
	valid := []Rules{
		{},
		{RuleTaskAssigned: {Bell: true, Email: true}},
		{RuleTaskDueSoon: {Bell: true, Days: 0}},
		{RuleTaskDueSoon: {Days: 60}},
	}
	for _, rules := range valid {
		if !Valid(rules) {
			t.Errorf("должно приниматься: %+v", rules)
		}
	}
	invalid := []Rules{
		{"nonsense": {Bell: true}},
		{RuleTaskDueSoon: {Days: -1}},
		{RuleTaskDueSoon: {Days: 61}},
		{RuleTaskAssigned: {Days: 5}},
		{RuleTaskOverdue: {Projects: []int{0}}},
	}
	for _, rules := range invalid {
		if Valid(rules) {
			t.Errorf("не должно приниматься: %+v", rules)
		}
	}
}

func TestRuleEnabled(t *testing.T) {
	if (Rule{}).Enabled() || !(Rule{Bell: true}).Enabled() || !(Rule{Email: true}).Enabled() {
		t.Error("правило включено, если доставляется хоть куда-нибудь")
	}
}
