package notify

import (
	"fmt"
	"math"
	"time"
)

// IssueState — то, что уведомлениям важно знать о задаче.
type IssueState struct {
	ID          int
	Subject     string
	ProjectID   int
	ProjectName string
	// AssignedTo — сотрудник источника, на которого назначена задача.
	AssignedTo *int
	StatusID   int
	StatusName string
	// StatusGroup — группа статуса: open, testing или closed.
	StatusGroup  string
	PriorityID   int
	PriorityName string
	// Due — срок, ГГГГ-ММ-ДД, или пусто.
	Due      string
	Estimate *float64
}

// Closed сообщает, что задача в закрытом статусе.
func (s IssueState) Closed() bool { return s.StatusGroup == "closed" }

// Change — изменение одного ключевого поля задачи.
type Change struct {
	// Field: status, priority, due_date или estimated_hours.
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// changes возвращает, какие ключевые поля задачи изменились: статус,
// приоритет, срок, оценка. Смена ответственного сюда не входит — это
// отдельное событие.
func changes(prev, cur IssueState) []Change {
	var list []Change
	if prev.StatusID != cur.StatusID {
		list = append(list, Change{Field: "status", From: prev.StatusName, To: cur.StatusName})
	}
	if prev.PriorityID != cur.PriorityID {
		list = append(list, Change{Field: "priority", From: prev.PriorityName, To: cur.PriorityName})
	}
	if prev.Due != cur.Due {
		list = append(list, Change{Field: "due_date", From: prev.Due, To: cur.Due})
	}
	if hoursText(prev.Estimate) != hoursText(cur.Estimate) {
		list = append(list, Change{Field: "estimated_hours", From: hoursText(prev.Estimate), To: hoursText(cur.Estimate)})
	}
	return list
}

// hoursText записывает часы без лишних нулей; пусто — оценки нет.
func hoursText(hours *float64) string {
	if hours == nil {
		return ""
	}
	return fmt.Sprintf("%g", math.Round(*hours*100)/100)
}

// assignedTo сообщает, на кого задача назначена заново: сотрудник, которому
// она только что досталась. ok ложно, если ответственный не менялся или его
// сняли. Для задачи, которую раньше не видели (hasPrev ложно), назначение
// считается новым.
func assignedTo(prev IssueState, hasPrev bool, cur IssueState) (member int, ok bool) {
	if cur.AssignedTo == nil {
		return 0, false
	}
	if hasPrev && prev.AssignedTo != nil && *prev.AssignedTo == *cur.AssignedTo {
		return 0, false
	}
	return *cur.AssignedTo, true
}

// daysBetween возвращает, сколько дней от from до to (обе ГГГГ-ММ-ДД);
// отрицательное — to раньше. ok ложно для неразборчивой даты.
func daysBetween(from, to string) (days int, ok bool) {
	a, errA := time.Parse("2006-01-02", from)
	b, errB := time.Parse("2006-01-02", to)
	if errA != nil || errB != nil {
		return 0, false
	}
	return int(math.Round(b.Sub(a).Hours() / 24)), true
}

// Ключи повтора: одно событие приходит пользователю один раз.

// dueSoonKey — «до срока осталось N дней»: один раз на каждый срок задачи;
// перенесли срок — напомнит снова.
func dueSoonKey(issueID int, due string) string {
	return fmt.Sprintf("%s:%d:%s", RuleTaskDueSoon, issueID, due)
}

// overdueKey — «задача просрочена»: один раз в день.
func overdueKey(issueID int, today string) string {
	return fmt.Sprintf("%s:%d:%s", RuleTaskOverdue, issueID, today)
}

// eventKey — событие, которое само случается один раз (назначение,
// изменение): ключ с моментом обнаружения.
func eventKey(rule string, issueID int, at time.Time) string {
	return fmt.Sprintf("%s:%d:%d", rule, issueID, at.UnixNano())
}
