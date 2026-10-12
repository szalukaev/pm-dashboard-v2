package handlers

import (
	"reflect"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func TestSprintVisibleTo(t *testing.T) {
	team := []int{10, 11}
	cases := []struct {
		name   string
		viewer int
		member *int
		owner  int
		shared bool
		team   []int
		want   bool
	}{
		{"свой личный", 1, nil, 1, false, nil, true},
		{"свой общий, сам в команду не входит", 1, intPtr(99), 1, true, team, true},
		{"чужой личный не виден никому", 2, intPtr(10), 1, false, team, false},
		{"чужой общий, сотрудник в команде создателя", 2, intPtr(11), 1, true, team, true},
		{"чужой общий, сотрудник вне команды", 2, intPtr(12), 1, true, team, false},
		{"чужой общий, пользователь не связан с источником", 2, nil, 1, true, team, false},
		{"создатель не выбирал команду — видят все связанные", 2, intPtr(12), 1, true, nil, true},
		{"создатель не выбирал команду, но пользователь не связан", 2, nil, 1, true, nil, false},
	}
	for _, c := range cases {
		if got := sprintVisibleTo(c.viewer, c.member, c.owner, c.shared, c.team); got != c.want {
			t.Errorf("%s: получено %v, ожидалось %v", c.name, got, c.want)
		}
	}
}

func TestSortSprintTasks(t *testing.T) {
	// Запрос отдаёт задачи по приоритету; группы статусов идут вперемешку
	tasks := []SprintTask{
		{ExternalID: 1, statusGroup: GroupClosed},
		{ExternalID: 2, statusGroup: GroupOpen},
		{ExternalID: 3, statusGroup: GroupTesting},
		{ExternalID: 4, statusGroup: GroupOpen},
		{ExternalID: 5, statusGroup: GroupClosed},
		{ExternalID: 6, statusGroup: GroupTesting},
	}
	sortSprintTasks(tasks)
	var got []int
	for _, task := range tasks {
		got = append(got, task.ExternalID)
	}
	// Открытые, в тестировании, закрытые; внутри части порядок прежний
	if want := []int{2, 4, 3, 6, 1, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("порядок задач %v, ожидался %v", got, want)
	}
}

func TestSprintProgress(t *testing.T) {
	sprintOf := func(open, testing, closed int) Sprint {
		var s Sprint
		for i := 0; i < open; i++ {
			s.Tasks = append(s.Tasks, SprintTask{statusGroup: GroupOpen})
		}
		for i := 0; i < testing; i++ {
			s.Tasks = append(s.Tasks, SprintTask{statusGroup: GroupTesting})
		}
		for i := 0; i < closed; i++ {
			s.Tasks = append(s.Tasks, SprintTask{statusGroup: GroupClosed})
		}
		s.calculateProgress()
		return s
	}

	cases := []struct {
		name                  string
		open, testing, closed int
		progress              float64
	}{
		{"из 10 задач одна закрыта", 9, 0, 1, 10},
		{"из 10 задач одна в тесте", 9, 1, 0, 5},
		{"из 10 задач одна закрыта и одна в тесте", 8, 1, 1, 15},
		{"все закрыты", 0, 0, 4, 100},
		{"ничего не сделано", 3, 0, 0, 0},
		{"пустой спринт", 0, 0, 0, 0},
	}
	for _, c := range cases {
		s := sprintOf(c.open, c.testing, c.closed)
		if s.Progress != c.progress {
			t.Errorf("%s: готовность %v, ожидалась %v", c.name, s.Progress, c.progress)
		}
		// Задача в тестировании — не открытая: счётчики не пересекаются
		if s.OpenCount != c.open || s.TestingCount != c.testing || s.ClosedCount != c.closed {
			t.Errorf("%s: счётчики %d/%d/%d, ожидались %d/%d/%d", c.name,
				s.OpenCount, s.TestingCount, s.ClosedCount, c.open, c.testing, c.closed)
		}
	}
}

func TestSprintDates(t *testing.T) {
	const today = "2026-10-12"
	loc := time.Local
	at := func(y int, m time.Month, d int) *time.Time {
		v := time.Date(y, m, d, 12, 0, 0, 0, loc)
		return &v
	}

	open := Sprint{Status: SprintOpen, DueDate: strPtr("2026-10-11T00:00:00Z")}
	open.setDates(nil, today)
	if !open.IsOverdue {
		t.Error("открытый спринт со сроком вчера должен быть просрочен")
	}

	dueToday := Sprint{Status: SprintOpen, DueDate: strPtr("2026-10-12")}
	dueToday.setDates(nil, today)
	if dueToday.IsOverdue {
		t.Error("срок сегодня — ещё не просрочен")
	}

	noDue := Sprint{Status: SprintOpen}
	noDue.setDates(nil, today)
	if noDue.IsOverdue {
		t.Error("спринт без даты окончания не бывает просроченным")
	}

	late := Sprint{Status: SprintClosed, DueDate: strPtr("2026-10-01")}
	late.setDates(at(2026, 10, 5), today)
	if late.IsOverdue || !late.ClosedLate || late.ClosedAt == nil {
		t.Errorf("закрыт позже срока: %+v", late)
	}

	inTime := Sprint{Status: SprintClosed, DueDate: strPtr("2026-10-01")}
	inTime.setDates(at(2026, 10, 1), today)
	if inTime.ClosedLate {
		t.Error("закрыт в день срока — вовремя")
	}

	// Спринт закрыт до появления даты закрытия: о просрочке ничего не известно
	old := Sprint{Status: SprintClosed, DueDate: strPtr("2026-01-01")}
	old.setDates(nil, today)
	if old.IsOverdue || old.ClosedLate || old.ClosedAt != nil {
		t.Errorf("старый закрытый спринт: %+v", old)
	}
}

func TestSortSprints(t *testing.T) {
	sprints := []Sprint{
		{ID: 1, Status: SprintOpen, DueDate: strPtr("2026-11-20")},
		{ID: 2, Status: SprintClosed, ClosedAt: strPtr("2026-09-01T10:00:00+05:00")},
		{ID: 3, Status: SprintOpen},
		{ID: 4, Status: SprintOpen, DueDate: strPtr("2026-10-01"), IsOverdue: true},
		{ID: 5, Status: SprintOpen, DueDate: strPtr("2026-10-20")},
		{ID: 6, Status: SprintClosed, ClosedAt: strPtr("2026-10-05T10:00:00+05:00")},
		{ID: 7, Status: SprintOpen, DueDate: strPtr("2026-09-15"), IsOverdue: true},
		{ID: 8, Status: SprintClosed},
	}
	sortSprints(sprints)
	var got []int
	for _, s := range sprints {
		got = append(got, s.ID)
	}
	// Просроченные по сроку, открытые по сроку (без срока — в конце),
	// закрытые — недавно закрытые выше, без даты закрытия последними
	if want := []int{7, 4, 5, 1, 3, 6, 2, 8}; !reflect.DeepEqual(got, want) {
		t.Errorf("порядок спринтов %v, ожидался %v", got, want)
	}
}

func TestValidSprintStatus(t *testing.T) {
	for _, status := range []interface{}{"open", "closed"} {
		if !validSprintStatus(status) {
			t.Errorf("статус %v должен приниматься", status)
		}
	}
	// «active» из ТЗ не используется
	for _, status := range []interface{}{"active", "", "Closed", nil, 1, true} {
		if validSprintStatus(status) {
			t.Errorf("статус %v не должен приниматься", status)
		}
	}
}

func TestIDList(t *testing.T) {
	if got := idList([]int{3, 1, 20}); got != "3,1,20" {
		t.Errorf("idList = %q", got)
	}
	// Пустой список не должен ломать условие IN и не должен ничего выбирать
	if got := idList(nil); got != "-1" {
		t.Errorf("idList пустого списка = %q", got)
	}
}
