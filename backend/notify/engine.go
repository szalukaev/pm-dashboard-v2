package notify

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pm-dashboard/access"
)

// EventNotification — событие WebSocket, с которым пользователю уходит новое
// уведомление.
const EventNotification = "notification"

// retention — сколько хранится журнал уведомлений.
const retention = 90 * 24 * time.Hour

// Notification — запись журнала, как её видит интерфейс.
type Notification struct {
	ID         int64           `json:"id"`
	Rule       string          `json:"rule"`
	Params     json.RawMessage `json:"params"`
	EntityType string          `json:"entity_type"`
	EntityID   int             `json:"entity_id"`
	CreatedAt  time.Time       `json:"created_at"`
	Read       bool            `json:"read"`
}

// Engine создаёт уведомления. Send доставляет новое уведомление открытым
// страницам пользователя; может быть nil.
type Engine struct {
	DB   **sql.DB
	Send func(userID int, event string, data interface{})
	// Scopes возвращает права пользователя: по ним правила по проектам
	// решают, вправе ли он видеть задачу. nil — такие правила молчат.
	Scopes func(userID int) (*access.Scope, error)
	// LicenseState возвращает состояние лицензии, о котором надо сообщить
	// администраторам (grace, read_only), или пустую строку. Может быть nil.
	LicenseState func() string
	// Push отправляет уведомление колокольчика push-ом на устройства
	// пользователя: заголовок, первая строка текста и ссылка. Может быть nil.
	Push func(userID int, title, body, link, tag string)

	// Проходы не пересекаются: два одновременных сравнения состояний
	// прислали бы одно изменение дважды.
	mu sync.Mutex
}

// recipient — пользователь, которому могут прийти уведомления.
type recipient struct {
	userID int
	member *int // сотрудник источника, с которым связан пользователь
	admin  bool
	rules  Rules
	lang   string // язык интерфейса пользователя: на нём текст push
}

func (e *Engine) recipients() ([]recipient, error) {
	rows, err := (*e.DB).Query(`SELECT u.id, u.member_id, u.role = 'admin', us.alert_rules, COALESCE(us.language, '')
		FROM users u LEFT JOIN user_settings us ON us.user_id = u.id
		WHERE NOT COALESCE(u.is_blocked, false)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []recipient
	for rows.Next() {
		var r recipient
		var raw []byte
		if rows.Scan(&r.userID, &r.member, &r.admin, &raw, &r.lang) != nil {
			continue
		}
		r.rules = Effective(raw)
		list = append(list, r)
	}
	return list, rows.Err()
}

// byMember раскладывает получателей по сотрудникам источника.
func byMember(list []recipient) map[int][]recipient {
	result := map[int][]recipient{}
	for _, r := range list {
		if r.member != nil {
			result[*r.member] = append(result[*r.member], r)
		}
	}
	return result
}

// Текущее состояние задач и последнее виденное читаются одинаковыми
// колонками, чтобы их можно было сравнивать.
const (
	issueColumns = `i.external_id, COALESCE(i.subject, ''), COALESCE(i.project_id, 0), COALESCE(i.project_name, ''),
		i.assigned_to_id, COALESCE(i.status_id, 0), COALESCE(i.status_name, ''), COALESCE(st.group_name, 'open'),
		COALESCE(i.priority_id, 0), COALESCE(i.priority_name, ''), COALESCE(to_char(i.due_date, 'YYYY-MM-DD'), ''),
		i.estimated_hours
		FROM issues i LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'`
	watchColumns = `issue_id, '', 0, '', assigned_to_id, status_id, status_name, status_group,
		priority_id, priority_name, due_date, estimated_hours FROM issue_watch`
)

func (e *Engine) states(query string, args ...interface{}) (map[int]IssueState, error) {
	rows, err := (*e.DB).Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[int]IssueState{}
	for rows.Next() {
		var s IssueState
		if err := rows.Scan(&s.ID, &s.Subject, &s.ProjectID, &s.ProjectName, &s.AssignedTo,
			&s.StatusID, &s.StatusName, &s.StatusGroup, &s.PriorityID, &s.PriorityName, &s.Due, &s.Estimate); err != nil {
			return nil, err
		}
		result[s.ID] = s
	}
	return result, rows.Err()
}

// remember записывает текущее состояние задач как последнее виденное.
// where с аргументами оставляет часть задач; пустое — все.
func (e *Engine) remember(where string, args ...interface{}) error {
	_, err := (*e.DB).Exec(`INSERT INTO issue_watch (issue_id, assigned_to_id, status_id, status_name, status_group,
			priority_id, priority_name, due_date, estimated_hours, seen_at)
		SELECT i.external_id, i.assigned_to_id, COALESCE(i.status_id, 0), COALESCE(i.status_name, ''), COALESCE(st.group_name, 'open'),
			COALESCE(i.priority_id, 0), COALESCE(i.priority_name, ''), COALESCE(to_char(i.due_date, 'YYYY-MM-DD'), ''),
			i.estimated_hours, NOW()
		FROM issues i LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'
		`+where+`
		ON CONFLICT (issue_id) DO UPDATE SET
			assigned_to_id = EXCLUDED.assigned_to_id, status_id = EXCLUDED.status_id,
			status_name = EXCLUDED.status_name, status_group = EXCLUDED.status_group,
			priority_id = EXCLUDED.priority_id, priority_name = EXCLUDED.priority_name,
			due_date = EXCLUDED.due_date, estimated_hours = EXCLUDED.estimated_hours, seen_at = NOW()`, args...)
	return err
}

// seen сообщает, запоминалось ли состояние задач хоть раз.
func (e *Engine) seen() bool {
	var any bool
	if err := (*e.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM issue_watch)").Scan(&any); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Database query failed", "at", "seen", "error", err)
	}
	return any
}

// pass — один проход по задачам: кто может получить уведомления и что для
// этого нужно знать.
type pass struct {
	people  []recipient
	members map[int][]recipient
	// scopes — права получателей на задачи; читаются по мере надобности.
	scopes map[int]*access.Scope
	// highPriority — номер высшего приоритета, 0 — неизвестен.
	highPriority int
	// except — пользователь, которому не сообщается: он сам внёс изменение.
	except int
	now    time.Time
}

func (e *Engine) newPass(now time.Time, except int) (*pass, error) {
	people, err := e.recipients()
	if err != nil {
		return nil, err
	}
	p := &pass{people: people, members: byMember(people), scopes: map[int]*access.Scope{}, except: except, now: now}
	if err := (*e.DB).QueryRow(`SELECT external_id FROM priorities WHERE data_source = 'redmine'
		ORDER BY sort_order DESC, external_id DESC LIMIT 1`).Scan(&p.highPriority); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Database query failed", "at", "newPass", "error", err)
	}
	return p, nil
}

// sees сообщает, вправе ли получатель видеть задачу. Без сведений о правах
// задача считается невидимой.
func (e *Engine) sees(p *pass, who recipient, issue IssueState) bool {
	if e.Scopes == nil {
		return false
	}
	scope, known := p.scopes[who.userID]
	if !known {
		scope, _ = e.Scopes(who.userID)
		p.scopes[who.userID] = scope
	}
	// Правила по проектам работают в отслеживаемых проектах — тех, что
	// пользователь выбрал в настройках (с подпроектами), а не во всех доступных
	return scope != nil && scope.ShowsProject(issue.ProjectID) && scope.SeesIssue(issue.ProjectID, issue.AssignedTo)
}

// AfterSync вызывается после синхронизации: сравнивает задачи с последним
// виденным состоянием, проверяет сроки и запоминает новое состояние. Самый
// первый проход только запоминает: о том, что было до него, не сообщается.
func (e *Engine) AfterSync(now time.Time) {
	if e.DB == nil || *e.DB == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	p, err := e.newPass(now, 0)
	if err != nil {
		slog.Error("Notifications: failed to read the recipients", "error", err)
		return
	}
	current, err := e.states("SELECT " + issueColumns)
	if err != nil {
		slog.Error("Notifications: failed to read the issues", "error", err)
		return
	}

	created := 0
	if e.seen() {
		previous, err := e.states("SELECT " + watchColumns)
		if err != nil {
			slog.Error("Notifications: failed to read the last seen state", "error", err)
			return
		}
		for id, cur := range current {
			prev, hasPrev := previous[id]
			created += e.issueChanged(p, prev, hasPrev, cur)
		}
	}
	created += e.deadlines(p, current)
	created += e.contracts(p)
	created += e.invoices(p)
	created += e.sprints(p)
	created += e.license(p)

	if err := e.remember(""); err != nil {
		slog.Error("Notifications: failed to remember the state of the issues", "error", err)
	}
	if _, err := (*e.DB).Exec("DELETE FROM issue_watch WHERE issue_id NOT IN (SELECT external_id FROM issues)"); err != nil {
		slog.Warn("Database write failed", "at", "AfterSync", "error", err)
	}
	if _, err := (*e.DB).Exec("DELETE FROM notifications WHERE created_at < $1", now.Add(-retention)); err != nil {
		slog.Warn("Database write failed", "at", "AfterSync", "error", err)
	}

	if created > 0 {
		slog.Info("Notifications created", "count", created)
	}
}

// IssueEdited вызывается, когда задачу изменили в самом дашборде: остальные
// узнают об изменении сразу, а сделавший его (actorID) — нет.
func (e *Engine) IssueEdited(issueID, actorID int, now time.Time) {
	if e.DB == nil || *e.DB == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	// До первого прохода сравнивать не с чем
	if !e.seen() {
		return
	}
	current, err := e.states("SELECT "+issueColumns+" WHERE i.external_id = $1", issueID)
	cur, found := current[issueID]
	if err != nil || !found {
		return
	}
	previous, err := e.states("SELECT "+watchColumns+" WHERE issue_id = $1", issueID)
	if err != nil {
		return
	}
	p, err := e.newPass(now, actorID)
	if err != nil {
		return
	}
	prev, hasPrev := previous[issueID]
	e.issueChanged(p, prev, hasPrev, cur)
	if err := e.remember("WHERE i.external_id = $1", issueID); err != nil {
		slog.Warn("Operation failed", "at", "IssueEdited", "error", err)
	}
}

// SyncFailed сообщает администраторам, что синхронизация с источником
// завершилась с ошибкой, — не чаще раза в день.
func (e *Engine) SyncFailed(now time.Time) {
	if e.DB == nil || *e.DB == nil {
		return
	}
	people, err := e.recipients()
	if err != nil {
		return
	}
	key := RuleSyncFailed + ":" + now.Format("2006-01-02")
	for _, who := range people {
		if who.admin {
			e.create(who, RuleSyncFailed, key, "system", 0, map[string]interface{}{})
		}
	}
}

// issueParams — данные о задаче для текста уведомления.
func issueParams(cur IssueState) map[string]interface{} {
	return map[string]interface{}{"issue_id": cur.ID, "subject": cur.Subject, "project": cur.ProjectName}
}

// issueChanged создаёт уведомления об изменении одной задачи и возвращает их
// число.
func (e *Engine) issueChanged(p *pass, prev IssueState, hasPrev bool, cur IssueState) int {
	created := 0
	to := func(who recipient, rule string, params map[string]interface{}) {
		if who.userID != p.except && e.create(who, rule, eventKey(rule, cur.ID, p.now), "issue", cur.ID, params) {
			created++
		}
	}

	member, assigned := assignedTo(prev, hasPrev, cur)
	if assigned {
		for _, who := range p.members[member] {
			to(who, RuleTaskAssigned, issueParams(cur))
		}
	}

	// Тому, кому задачу только что назначили, хватит уведомления о назначении
	if hasPrev && !assigned && cur.AssignedTo != nil {
		if list := changes(prev, cur); len(list) > 0 {
			params := issueParams(cur)
			params["changes"] = list
			for _, who := range p.members[*cur.AssignedTo] {
				to(who, RuleTaskChanged, params)
			}
		}
	}

	// Правила по проектам: кто бы ни был ответственным
	closed := hasPrev && !prev.Closed() && cur.Closed()
	becameHigh := p.highPriority != 0 && cur.PriorityID == p.highPriority && !cur.Closed() &&
		(!hasPrev || prev.PriorityID != p.highPriority)
	if closed || becameHigh {
		for _, who := range p.people {
			if closed && e.projectRule(p, who, RuleProjectTaskClosed, cur) {
				to(who, RuleProjectTaskClosed, issueParams(cur))
			}
			if becameHigh && e.projectRule(p, who, RuleProjectHighPriority, cur) {
				params := issueParams(cur)
				params["priority"] = cur.PriorityName
				to(who, RuleProjectHighPriority, params)
			}
		}
	}
	return created
}

// projectRule сообщает, касается ли получателя правило по проектам для этой
// задачи: правило включено, проект выбран, задачу он вправе видеть.
func (e *Engine) projectRule(p *pass, who recipient, rule string, issue IssueState) bool {
	setup := who.rules[rule]
	return setup.Enabled() && setup.ForProject(issue.ProjectID) && e.sees(p, who, issue)
}

// deadlines создаёт уведомления о сроках задач: ответственным — срок близко
// и задача просрочена; по правилу проектов — задача просрочена на заданное
// число дней. Возвращает число созданных.
func (e *Engine) deadlines(p *pass, current map[int]IssueState) int {
	today := p.now.Format("2006-01-02")
	created := 0
	for _, cur := range current {
		if cur.Closed() || cur.Due == "" {
			continue
		}
		left, ok := daysBetween(today, cur.Due)
		if !ok {
			continue
		}
		params := issueParams(cur)
		params["due"] = cur.Due

		if cur.AssignedTo != nil {
			for _, who := range p.members[*cur.AssignedTo] {
				switch {
				case left < 0:
					params["days"] = -left
					if e.create(who, RuleTaskOverdue, overdueKey(cur.ID, today), "issue", cur.ID, params) {
						created++
					}
				case left <= who.rules[RuleTaskDueSoon].Days:
					params["days"] = left
					if e.create(who, RuleTaskDueSoon, dueSoonKey(cur.ID, cur.Due), "issue", cur.ID, params) {
						created++
					}
				}
			}
		}

		if left < 0 {
			for _, who := range p.people {
				if !reachedThreshold(-left, who.rules[RuleProjectOverdue].Days) || !e.projectRule(p, who, RuleProjectOverdue, cur) {
					continue
				}
				params["days"] = -left
				if e.create(who, RuleProjectOverdue, onceKey(RuleProjectOverdue, cur.ID, cur.Due), "issue", cur.ID, params) {
					created++
				}
			}
		}
	}
	return created
}

// byUser возвращает получателя по номеру пользователя.
func (p *pass) byUser(userID int) (recipient, bool) {
	for _, who := range p.people {
		if who.userID == userID {
			return who, true
		}
	}
	return recipient{}, false
}

// contracts сообщает владельцам договоров, что срок договора близко или
// истёк.
func (e *Engine) contracts(p *pass) int {
	rows, err := (*e.DB).Query(`SELECT id, COALESCE(user_id, 0), name, to_char(end_date, 'YYYY-MM-DD')
		FROM contracts WHERE end_date IS NOT NULL`)
	if err != nil {
		slog.Error("Notifications: failed to read the contracts", "error", err)
		return 0
	}
	defer rows.Close()

	today := p.now.Format("2006-01-02")
	created := 0
	for rows.Next() {
		var id, userID int
		var name, end string
		if rows.Scan(&id, &userID, &name, &end) != nil {
			continue
		}
		who, known := p.byUser(userID)
		left, ok := daysBetween(today, end)
		if !known || !ok {
			continue
		}
		params := map[string]interface{}{"name": name, "due": end}
		switch deadlineKind(left, who.rules[RuleContractEnding].Days) {
		case deadlineSoon:
			params["days"] = left
			if e.create(who, RuleContractEnding, onceKey(RuleContractEnding, id, end), "contract", id, params) {
				created++
			}
		case deadlineLate:
			params["days"] = -left
			if e.create(who, RuleContractExpired, onceKey(RuleContractExpired, id, end), "contract", id, params) {
				created++
			}
		}
	}
	return created
}

// invoices сообщает владельцу договора, что выставленный по нему счёт не
// оплачен заданное число дней.
func (e *Engine) invoices(p *pass) int {
	rows, err := (*e.DB).Query(`SELECT inv.id, COALESCE(c.user_id, 0), c.name, to_char(inv.issued_at, 'YYYY-MM-DD'),
			COALESCE(inv.amount, 0)::float8
		FROM invoices inv JOIN contracts c ON c.id = inv.contract_id
		WHERE COALESCE(inv.status, 'unpaid') <> 'paid'`)
	if err != nil {
		slog.Error("Notifications: failed to read the invoices", "error", err)
		return 0
	}
	defer rows.Close()

	today := p.now.Format("2006-01-02")
	created := 0
	for rows.Next() {
		var id, userID int
		var name, issued string
		var amount float64
		if rows.Scan(&id, &userID, &name, &issued, &amount) != nil {
			continue
		}
		who, known := p.byUser(userID)
		age, ok := daysBetween(issued, today)
		if !known || !ok || !reachedThreshold(age, who.rules[RuleInvoiceUnpaid].Days) {
			continue
		}
		params := map[string]interface{}{"name": name, "issued": issued, "days": age, "amount": amount}
		if e.create(who, RuleInvoiceUnpaid, onceKey(RuleInvoiceUnpaid, id, issued), "invoice", id, params) {
			created++
		}
	}
	return created
}

// sprints сообщает тем, кому спринт виден, что его срок близко или прошёл.
func (e *Engine) sprints(p *pass) int {
	rows, err := (*e.DB).Query(`SELECT s.id, COALESCE(s.user_id, 0), s.name, s.is_shared,
			to_char(s.due_date, 'YYYY-MM-DD'), os.selected_team
		FROM sprints s LEFT JOIN user_settings os ON os.user_id = s.user_id
		WHERE s.status <> 'closed' AND s.due_date IS NOT NULL`)
	if err != nil {
		slog.Error("Notifications: failed to read the sprints", "error", err)
		return 0
	}
	defer rows.Close()

	today := p.now.Format("2006-01-02")
	created := 0
	for rows.Next() {
		var id, ownerID int
		var name, due string
		var shared bool
		var team []byte
		if rows.Scan(&id, &ownerID, &name, &shared, &due, &team) != nil {
			continue
		}
		left, ok := daysBetween(today, due)
		if !ok {
			continue
		}
		var ownerTeam []int
		_ = json.Unmarshal(team, &ownerTeam)

		for _, who := range p.people {
			if !access.SprintVisibleTo(who.userID, who.member, ownerID, shared, ownerTeam) {
				continue
			}
			params := map[string]interface{}{"name": name, "due": due}
			switch deadlineKind(left, who.rules[RuleSprintEnding].Days) {
			case deadlineSoon:
				params["days"] = left
				if e.create(who, RuleSprintEnding, onceKey(RuleSprintEnding, id, due), "sprint", id, params) {
					created++
				}
			case deadlineLate:
				params["days"] = -left
				if e.create(who, RuleSprintOverdue, onceKey(RuleSprintOverdue, id, due), "sprint", id, params) {
					created++
				}
			}
		}
	}
	return created
}

// license сообщает администраторам, что лицензия в льготном периоде или в
// режиме «только чтение», — раз в месяц, пока состояние не изменится.
func (e *Engine) license(p *pass) int {
	if e.LicenseState == nil {
		return 0
	}
	state := e.LicenseState()
	if state == "" {
		return 0
	}
	key := RuleLicenseState + ":" + state + ":" + p.now.Format("2006-01")
	created := 0
	for _, who := range p.people {
		if who.admin && e.create(who, RuleLicenseState, key, "system", 0, map[string]interface{}{"state": state}) {
			created++
		}
	}
	return created
}

// create кладёт уведомление в журнал, если правило у пользователя включено и
// такого уведомления ещё не было, и сразу доставляет его открытым страницам.
// Возвращает, создано ли уведомление.
func (e *Engine) create(who recipient, rule, key, entityType string, entityID int, params map[string]interface{}) bool {
	setup := who.rules[rule]
	if !setup.Enabled() || (AdminOnly(rule) && !who.admin) {
		return false
	}
	data, err := json.Marshal(params)
	if err != nil {
		return false
	}

	n := Notification{Rule: rule, Params: data, EntityType: entityType, EntityID: entityID}
	err = (*e.DB).QueryRow(`INSERT INTO notifications (user_id, rule, params, entity_type, entity_id, dedupe_key, to_bell, to_email)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, dedupe_key) DO NOTHING
		RETURNING id, created_at`,
		who.userID, rule, string(data), entityType, entityID, key, setup.Bell, setup.Email).Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		// sql.ErrNoRows — такое уведомление уже есть
		if err != sql.ErrNoRows {
			slog.Error("Notifications: failed to save", "rule", rule, "user", who.userID, "error", err)
		}
		return false
	}
	if setup.Bell && e.Send != nil {
		e.Send(who.userID, EventNotification, n)
	}
	if setup.Bell && e.Push != nil {
		body := ""
		if lines := Lines(n, who.lang); len(lines) > 0 {
			body = lines[0]
		}
		go e.Push(who.userID, Title(n, who.lang), body, Link(n), fmt.Sprintf("n%d", n.ID))
	}
	return true
}

// ErrNoIssue — нет задачи, по которой можно сделать проверочное уведомление.
var ErrNoIssue = errors.New("no issue for test notification")

// Test создаёт настоящее уведомление «Вам назначена задача» для проверки
// push: по открытой задаче пользователя, а если своих нет — по последней
// изменённой. Оно попадает в колокольчик и уходит в открытые страницы, как
// обычное; правила пользователя не проверяются. Push отправляет вызывающий:
// он показывает ответы сервисов доставки. Возвращает уведомление и текст push.
func (e *Engine) Test(userID int) (n Notification, title, body, link string, err error) {
	db := *e.DB
	var member *int
	var lang string
	if err = db.QueryRow(`SELECT u.member_id, COALESCE(us.language, '') FROM users u
		LEFT JOIN user_settings us ON us.user_id = u.id WHERE u.id = $1`, userID).Scan(&member, &lang); err != nil {
		return
	}
	var issue struct {
		id      int
		subject string
		project string
	}
	pick := `SELECT i.external_id, COALESCE(i.subject, ''), COALESCE(i.project_name, '')
		FROM issues i LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'
		WHERE COALESCE(st.group_name, 'open') <> 'closed' %s ORDER BY i.updated_on DESC NULLS LAST LIMIT 1`
	err = sql.ErrNoRows
	if member != nil {
		err = db.QueryRow(fmt.Sprintf(pick, "AND i.assigned_to_id = $1"), *member).Scan(&issue.id, &issue.subject, &issue.project)
	}
	if err == sql.ErrNoRows {
		err = db.QueryRow(fmt.Sprintf(pick, "")).Scan(&issue.id, &issue.subject, &issue.project)
	}
	if err == sql.ErrNoRows {
		err = ErrNoIssue
	}
	if err != nil {
		return
	}

	params := map[string]interface{}{"issue_id": issue.id, "subject": issue.subject, "project": issue.project, "test": true}
	data, _ := json.Marshal(params)
	n = Notification{Rule: RuleTaskAssigned, Params: data, EntityType: "issue", EntityID: issue.id}
	// Свой ключ на каждую проверку: повторная не теряется как дубль
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	if err = db.QueryRow(`INSERT INTO notifications (user_id, rule, params, entity_type, entity_id, dedupe_key, to_bell, to_email)
		VALUES ($1, $2, $3::jsonb, 'issue', $4, $5, TRUE, FALSE) RETURNING id, created_at`,
		userID, RuleTaskAssigned, string(data), issue.id, key).Scan(&n.ID, &n.CreatedAt); err != nil {
		return
	}
	if e.Send != nil {
		e.Send(userID, EventNotification, n)
	}
	if lines := Lines(n, lang); len(lines) > 0 {
		body = lines[0]
	}
	return n, Title(n, lang), body, Link(n), nil
}

// Link — куда ведёт уведомление в дашборде: задача открывается карточкой,
// остальное — своей вкладкой. Те же правила, что у колокольчика на фронте.
func Link(n Notification) string {
	switch n.EntityType {
	case "issue":
		if n.EntityID > 0 {
			return fmt.Sprintf("/tasks?task=%d", n.EntityID)
		}
	case "contract", "invoice":
		return "/payments"
	case "sprint":
		return "/sprint"
	case "system":
		return "/settings"
	}
	return "/notifications"
}

// List возвращает страницу колокольчика пользователя, новые сверху, и сколько
// в нём всего уведомлений и сколько непрочитанных.
func List(db *sql.DB, userID, limit, offset int) (list []Notification, total, unread int, err error) {
	list = []Notification{}
	if err = db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE read_at IS NULL)
		FROM notifications WHERE user_id = $1 AND to_bell`, userID).Scan(&total, &unread); err != nil {
		return
	}
	rows, err := db.Query(`SELECT id, rule, params, entity_type, entity_id, created_at, read_at IS NOT NULL
		FROM notifications WHERE user_id = $1 AND to_bell
		ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var n Notification
		var params []byte
		if rows.Scan(&n.ID, &n.Rule, &params, &n.EntityType, &n.EntityID, &n.CreatedAt, &n.Read) == nil {
			n.Params = params
			list = append(list, n)
		}
	}
	err = rows.Err()
	return
}

// MarkRead помечает уведомление пользователя прочитанным; id = 0 — все.
func MarkRead(db *sql.DB, userID int, id int64) error {
	_, err := db.Exec(`UPDATE notifications SET read_at = NOW()
		WHERE user_id = $1 AND read_at IS NULL AND ($2::bigint = 0 OR id = $2::bigint)`, userID, id)
	return err
}

// ClearBell убирает все уведомления пользователя из колокольчика. Записи не
// удаляются: по их ключу повтора то же уведомление не создаётся второй раз,
// а старые записи и так удаляет очистка по сроку хранения.
func ClearBell(db *sql.DB, userID int) error {
	_, err := db.Exec(`UPDATE notifications SET to_bell = FALSE, read_at = COALESCE(read_at, NOW())
		WHERE user_id = $1 AND to_bell`, userID)
	return err
}
