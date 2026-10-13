package notify

import (
	"fmt"
	"reflect"
	"strings"
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

func TestDeadlineKind(t *testing.T) {
	cases := []struct {
		name       string
		left, soon int
		want       string
	}{
		{"срок через 14 дней при пороге 14", 14, 14, deadlineSoon},
		{"срок сегодня", 0, 14, deadlineSoon},
		{"до срока дальше порога", 15, 14, deadlineNone},
		{"срок прошёл вчера", -1, 14, deadlineLate},
		{"срок прошёл неделю назад", -7, 14, deadlineLate},
		{"срок прошёл давно — уже не новость", -8, 14, deadlineNone},
		{"порог 0: только в день срока", 0, 0, deadlineSoon},
		{"порог 0: за день ещё рано", 1, 0, deadlineNone},
	}
	for _, c := range cases {
		if got := deadlineKind(c.left, c.soon); got != c.want {
			t.Errorf("%s: получено %q, ожидалось %q", c.name, got, c.want)
		}
	}
}

func TestReachedThreshold(t *testing.T) {
	cases := []struct {
		days, threshold int
		want            bool
	}{
		{13, 14, false}, // порог ещё не достигнут
		{14, 14, true},  // достигнут сегодня
		{21, 14, true},  // достигнут неделю назад — ещё свежо
		{22, 14, false}, // давно: включённое правило не поднимает старое
	}
	for _, c := range cases {
		if got := reachedThreshold(c.days, c.threshold); got != c.want {
			t.Errorf("reachedThreshold(%d, %d) = %v", c.days, c.threshold, got)
		}
	}
}

func TestOnceKey(t *testing.T) {
	if onceKey(RuleContractExpired, 5, "2026-10-01") != onceKey(RuleContractExpired, 5, "2026-10-01") {
		t.Error("однократное событие — один ключ")
	}
	// Срок договора продлили: о новом сроке сообщится заново
	if onceKey(RuleContractExpired, 5, "2026-10-01") == onceKey(RuleContractExpired, 5, "2026-12-01") {
		t.Error("новый срок — новое уведомление")
	}
	if onceKey(RuleContractExpired, 5, "2026-10-01") == onceKey(RuleContractEnding, 5, "2026-10-01") {
		t.Error("у разных правил разные ключи")
	}
}

func TestRuleForProject(t *testing.T) {
	if !(Rule{}).ForProject(7) {
		t.Error("без списка проектов правило работает для всех")
	}
	chosen := Rule{Projects: []int{1, 2}}
	if !chosen.ForProject(2) || chosen.ForProject(7) {
		t.Error("со списком правило работает только для выбранных проектов")
	}
}

func TestRuleSpecs(t *testing.T) {
	// У каждого ключа есть описание, и наоборот
	if len(Keys()) != len(specs) {
		t.Fatalf("ключей %d, описаний %d", len(Keys()), len(specs))
	}
	for _, key := range Keys() {
		if _, known := specs[key]; !known {
			t.Errorf("у правила %s нет описания", key)
		}
	}
	// Правила по проектам пользователь включает сам; о своём сообщается сразу
	defaults := Defaults()
	for _, key := range []string{RuleProjectOverdue, RuleProjectHighPriority, RuleProjectTaskClosed} {
		if defaults[key].Enabled() {
			t.Errorf("правило %s не должно быть включено по умолчанию", key)
		}
	}
	for _, key := range []string{RuleTaskAssigned, RuleContractEnding, RuleSprintOverdue} {
		if !defaults[key].Enabled() {
			t.Errorf("правило %s должно быть включено по умолчанию", key)
		}
	}
	// Стандартные дни лежат в допустимых пределах
	for key, s := range specs {
		if s.hasDays && (s.def.Days < s.minDays || s.def.Days > s.maxDays) {
			t.Errorf("правило %s: стандартные дни %d вне пределов", key, s.def.Days)
		}
	}
	if !AdminOnly(RuleSyncFailed) || !AdminOnly(RuleLicenseState) || AdminOnly(RuleTaskAssigned) {
		t.Error("системные правила — только администратору")
	}
}

func TestMailTexts(t *testing.T) {
	changed := Notification{Rule: RuleTaskChanged, CreatedAt: time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC),
		Params: []byte(`{"issue_id": 101, "subject": "Отчёт <b>", "project": "Albank", "changes": [
			{"field": "status", "from": "Новая", "to": "В работе"},
			{"field": "due_date", "from": "2026-10-20", "to": ""}]}`)}
	if got := Title(changed, "ru"); got != "Задача изменена" {
		t.Errorf("заголовок: %q", got)
	}
	want := []string{"#101 Отчёт <b>", "Статус: Новая → В работе", "Срок: 20.10.2026 → —", "Albank"}
	if got := Lines(changed, "ru"); !reflect.DeepEqual(got, want) {
		t.Errorf("строки: %q", got)
	}
	if got := Lines(changed, "en"); got[1] != "Status: Новая → В работе" {
		t.Errorf("строки по-английски: %q", got)
	}

	overdue := Notification{Rule: RuleTaskOverdue, Params: []byte(`{"issue_id": 5, "subject": "Задача", "days": 12, "due": "2026-10-01"}`)}
	if got := Title(overdue, "ru"); got != "Задача просрочена, дней: 12" {
		t.Errorf("просрочка: %q", got)
	}
	if got := Title(overdue, "de"); got != "Задача просрочена, дней: 12" {
		t.Errorf("неизвестный язык считается русским: %q", got)
	}

	contract := Notification{Rule: RuleContractEnding, Params: []byte(`{"name": "Сопровождение 2026", "due": "2026-10-27", "days": 14}`)}
	if got := Lines(contract, "ru"); !reflect.DeepEqual(got, []string{"Сопровождение 2026", "Срок: 27.10.2026"}) {
		t.Errorf("договор: %q", got)
	}

	license := Notification{Rule: RuleLicenseState, Params: []byte(`{"state": "read_only"}`)}
	if got := Title(license, "ru"); got != "Лицензия: только чтение" {
		t.Errorf("лицензия: %q", got)
	}

	// У каждого правила есть заголовок на обоих языках
	for _, key := range Keys() {
		for _, lang := range []string{"ru", "en"} {
			if titles[lang][key] == "" {
				t.Errorf("нет заголовка правила %s на языке %s", key, lang)
			}
		}
	}

	// Без адреса дашборда задача в письме — текст, ссылок нет
	if _, plain := Digest([]Notification{changed}, "ru", nil); strings.Contains(plain, "<a ") {
		t.Error("без адреса дашборда ссылок в письме быть не должно")
	}
	link := func(id int) string { return fmt.Sprintf("http://pm.example.com/tasks?task=%d", id) }
	subject, body := Digest([]Notification{changed, overdue, contract}, "ru", link)
	// Номер и название задачи ведут на её карточку; у договора ссылки нет
	if !strings.Contains(body, `<a href="http://pm.example.com/tasks?task=101"`) || !strings.Contains(body, `task=5"`) {
		t.Error("номер и название задачи должны быть ссылкой на карточку")
	}
	if strings.Count(body, "<a ") != 2 {
		t.Errorf("ссылок должно быть две — по одной на задачу, найдено %d", strings.Count(body, "<a "))
	}
	if subject != "PM Dashboard: новых уведомлений — 3" {
		t.Errorf("тема: %q", subject)
	}
	// Текст из задач не должен стать разметкой письма
	if strings.Contains(body, "<b>") || !strings.Contains(body, "Отчёт &lt;b&gt;") {
		t.Error("название задачи должно быть экранировано")
	}
	if !strings.Contains(body, "Задача просрочена, дней: 12") || !strings.Contains(body, "13.10.2026 09:30") {
		t.Error("в письме должны быть заголовки уведомлений и время")
	}
}
