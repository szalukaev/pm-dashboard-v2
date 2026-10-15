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

// overdueKey — «задача просрочена»: одно уведомление в день на человека,
// сколько бы задач ни было просрочено.
func overdueKey(today string) string {
	return fmt.Sprintf("%s:%s", RuleTaskOverdue, today)
}

// eventKey — событие, которое само случается один раз (назначение,
// изменение): ключ с моментом обнаружения.
func eventKey(rule string, issueID int, at time.Time) string {
	return fmt.Sprintf("%s:%d:%d", rule, issueID, at.UnixNano())
}

// recentDays — сколько дней однократное событие ещё считается свежим. О том,
// что случилось раньше, не сообщается: иначе включённое правило высыпало бы
// всю историю разом.
const recentDays = 7

// Сроки договоров и спринтов: что сообщить по числу дней до срока.
const (
	deadlineNone = ""
	deadlineSoon = "soon" // срок близко
	deadlineLate = "late" // срок прошёл недавно
)

// deadlineKind решает, о чём сообщить по сроку, до которого осталось left
// дней (отрицательное — срок прошёл): «скоро», если осталось не больше
// soonDays; «прошёл», если прошёл не раньше чем recentDays дней назад.
func deadlineKind(left, soonDays int) string {
	switch {
	case left < 0 && -left <= recentDays:
		return deadlineLate
	case left >= 0 && left <= soonDays:
		return deadlineSoon
	}
	return deadlineNone
}

// reachedThreshold сообщает, что счётчик дней (возраст счёта, дни просрочки)
// дошёл до порога и это случилось недавно.
func reachedThreshold(days, threshold int) bool {
	return days >= threshold && days <= threshold+recentDays
}

// onceKey — ключ однократного события по сущности и её сроку: если срок
// перенесут, о новом сроке сообщится заново.
func onceKey(rule string, id int, date string) string {
	return fmt.Sprintf("%s:%d:%s", rule, id, date)
}
