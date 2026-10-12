package notify

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
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
}

func (e *Engine) recipients() ([]recipient, error) {
	rows, err := (*e.DB).Query(`SELECT u.id, u.member_id, u.role = 'admin', us.alert_rules
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
		if rows.Scan(&r.userID, &r.member, &r.admin, &raw) != nil {
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
	(*e.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM issue_watch)").Scan(&any)
	return any
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

	people, err := e.recipients()
	if err != nil {
		slog.Error("Notifications: failed to read the recipients", "error", err)
		return
	}
	members := byMember(people)

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
			created += e.issueChanged(prev, hasPrev, cur, members, 0, now)
		}
	}
	created += e.deadlines(current, members, now)

	if err := e.remember(""); err != nil {
		slog.Error("Notifications: failed to remember the state of the issues", "error", err)
	}
	(*e.DB).Exec("DELETE FROM issue_watch WHERE issue_id NOT IN (SELECT external_id FROM issues)")
	(*e.DB).Exec("DELETE FROM notifications WHERE created_at < $1", now.Add(-retention))

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
	people, err := e.recipients()
	if err != nil {
		return
	}
	prev, hasPrev := previous[issueID]
	e.issueChanged(prev, hasPrev, cur, byMember(people), actorID, now)
	e.remember("WHERE i.external_id = $1", issueID)
}

// issueChanged создаёт уведомления об изменении одной задачи и возвращает их
// число. except — пользователь, которому не сообщается (он сам внёс изменение).
func (e *Engine) issueChanged(prev IssueState, hasPrev bool, cur IssueState, members map[int][]recipient, except int, now time.Time) int {
	created := 0
	base := map[string]interface{}{"issue_id": cur.ID, "subject": cur.Subject, "project": cur.ProjectName}

	member, assigned := assignedTo(prev, hasPrev, cur)
	if assigned {
		for _, who := range members[member] {
			if who.userID != except && e.create(who, RuleTaskAssigned, eventKey(RuleTaskAssigned, cur.ID, now), "issue", cur.ID, base) {
				created++
			}
		}
	}

	// Тому, кому задачу только что назначили, хватит уведомления о назначении
	if hasPrev && !assigned && cur.AssignedTo != nil {
		if list := changes(prev, cur); len(list) > 0 {
			params := map[string]interface{}{"issue_id": cur.ID, "subject": cur.Subject, "project": cur.ProjectName, "changes": list}
			for _, who := range members[*cur.AssignedTo] {
				if who.userID != except && e.create(who, RuleTaskChanged, eventKey(RuleTaskChanged, cur.ID, now), "issue", cur.ID, params) {
					created++
				}
			}
		}
	}
	return created
}

// deadlines создаёт уведомления о сроках задач ответственным: срок близко и
// задача просрочена. Возвращает число созданных.
func (e *Engine) deadlines(current map[int]IssueState, members map[int][]recipient, now time.Time) int {
	today := now.Format("2006-01-02")
	created := 0
	for _, cur := range current {
		if cur.Closed() || cur.AssignedTo == nil || cur.Due == "" {
			continue
		}
		left, ok := daysBetween(today, cur.Due)
		if !ok {
			continue
		}
		for _, who := range members[*cur.AssignedTo] {
			params := map[string]interface{}{"issue_id": cur.ID, "subject": cur.Subject, "project": cur.ProjectName, "due": cur.Due}
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
	return created
}

// create кладёт уведомление в журнал, если правило у пользователя включено и
// такого уведомления ещё не было, и сразу доставляет его открытым страницам.
// Возвращает, создано ли уведомление.
func (e *Engine) create(who recipient, rule, key, entityType string, entityID int, params map[string]interface{}) bool {
	setup := who.rules[rule]
	if !setup.Enabled() {
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
	return true
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
