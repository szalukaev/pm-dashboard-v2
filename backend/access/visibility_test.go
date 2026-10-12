package access

import "testing"

func memberPtr(v int) *int { return &v }

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
		{"свой общий, сам в команду не входит", 1, memberPtr(99), 1, true, team, true},
		{"чужой личный не виден никому", 2, memberPtr(10), 1, false, team, false},
		{"чужой общий, сотрудник в команде создателя", 2, memberPtr(11), 1, true, team, true},
		{"чужой общий, сотрудник вне команды", 2, memberPtr(12), 1, true, team, false},
		{"чужой общий, пользователь не связан с источником", 2, nil, 1, true, team, false},
		{"создатель не выбирал команду — видят все связанные", 2, memberPtr(12), 1, true, nil, true},
		{"создатель не выбирал команду, но пользователь не связан", 2, nil, 1, true, nil, false},
	}
	for _, c := range cases {
		if got := SprintVisibleTo(c.viewer, c.member, c.owner, c.shared, c.team); got != c.want {
			t.Errorf("%s: получено %v, ожидалось %v", c.name, got, c.want)
		}
	}
}

func TestSeesIssue(t *testing.T) {
	limited := &Scope{AllowedProjects: []int{1, 2}, AllowedTeam: []int{10, 11}}
	cases := []struct {
		name     string
		scope    *Scope
		project  int
		assignee *int
		want     bool
	}{
		{"свой проект и свой сотрудник", limited, 1, memberPtr(10), true},
		{"чужой проект", limited, 3, memberPtr(10), false},
		{"сотрудник вне команды", limited, 1, memberPtr(12), false},
		{"без ответственного — без права не видно", limited, 1, nil, false},
		{"без ответственного — с правом видно", &Scope{AllowedProjects: []int{1}, AllowedTeam: []int{10}, ShowUnassigned: true}, 1, nil, true},
		{"все проекты и все сотрудники", &Scope{AllowedAllProjects: true, AllowedAllTeam: true}, 7, memberPtr(99), true},
		{"все проекты и все сотрудники, но без ответственного", &Scope{AllowedAllProjects: true, AllowedAllTeam: true}, 7, nil, false},
		{"своя задача видна и вне разрешённых проектов", &Scope{OwnMember: memberPtr(5)}, 9, memberPtr(5), true},
		{"чужая задача при правах «только свои»", &Scope{OwnMember: memberPtr(5)}, 9, memberPtr(6), false},
	}
	for _, c := range cases {
		if got := c.scope.SeesIssue(c.project, c.assignee); got != c.want {
			t.Errorf("%s: получено %v, ожидалось %v", c.name, got, c.want)
		}
	}
}
