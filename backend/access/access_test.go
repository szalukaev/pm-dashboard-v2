package access

import (
	"reflect"
	"testing"
)

func keys(list ...string) *[]string { return &list }

func TestEffective(t *testing.T) {
	groupA := Permissions{ProjectIDs: []int{1, 2}, TeamIDs: []int{10}, VisibleTabs: keys("tasks")}
	groupB := Permissions{ProjectIDs: []int{2, 3}, TeamIDs: []int{11}, VisibleTabs: keys("kanban")}

	t.Run("no rights at all: own tasks only", func(t *testing.T) {
		got := effective(nil, nil, nil)
		if !got.OwnTasksOnly || got.AllProjects || len(got.ProjectIDs) != 0 || got.VisibleTabs != nil {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("groups are summed", func(t *testing.T) {
		got := effective(nil, nil, []Permissions{groupA, groupB})
		if !reflect.DeepEqual(got.ProjectIDs, []int{1, 2, 3}) || !reflect.DeepEqual(got.TeamIDs, []int{10, 11}) {
			t.Errorf("lists = %v / %v", got.ProjectIDs, got.TeamIDs)
		}
		if !reflect.DeepEqual(*got.VisibleTabs, []string{"tasks", "kanban"}) {
			t.Errorf("tabs = %v", *got.VisibleTabs)
		}
	})

	t.Run("template is used until rights are set individually", func(t *testing.T) {
		template := Permissions{AllProjects: true, AllTeam: true, ReadOnly: true}
		got := effective(nil, &template, nil)
		if !got.AllProjects || !got.ReadOnly || got.OwnTasksOnly {
			t.Errorf("from template: %+v", got)
		}
		individual := Permissions{ProjectIDs: []int{7}}
		got = effective(&individual, &template, nil)
		if got.AllProjects || got.ReadOnly || !reflect.DeepEqual(got.ProjectIDs, []int{7}) {
			t.Errorf("individual must replace the template: %+v", got)
		}
	})

	t.Run("individual rights add to groups, individual tabs win", func(t *testing.T) {
		individual := Permissions{ProjectIDs: []int{9}, VisibleTabs: keys("analytics")}
		got := effective(&individual, nil, []Permissions{groupA})
		if !reflect.DeepEqual(got.ProjectIDs, []int{1, 2, 9}) {
			t.Errorf("projects = %v", got.ProjectIDs)
		}
		if !reflect.DeepEqual(*got.VisibleTabs, []string{"analytics"}) {
			t.Errorf("tabs = %v", *got.VisibleTabs)
		}
	})

	t.Run("a group cannot make a read-only user a writer", func(t *testing.T) {
		individual := Permissions{ReadOnly: true}
		got := effective(&individual, nil, []Permissions{{AllProjects: true}})
		if !got.ReadOnly || !got.AllProjects {
			t.Errorf("got %+v", got)
		}
	})
}

func TestNarrow(t *testing.T) {
	tests := []struct {
		name       string
		allowedAll bool
		allowed    []int
		selected   []int
		wantAll    bool
		want       []int
	}{
		{"nothing selected: everything allowed", false, []int{1, 2, 3}, nil, false, []int{1, 2, 3}},
		{"selection reduces", false, []int{1, 2, 3}, []int{1}, false, []int{1}},
		{"selection cannot widen", false, []int{1, 2, 3}, []int{1, 99}, false, []int{1}},
		{"only forbidden selected: nothing", false, []int{1, 2, 3}, []int{99}, false, []int{}},
		{"all allowed, nothing selected", true, nil, nil, true, []int{}},
		{"all allowed, selection applies", true, nil, []int{5, 4}, false, []int{4, 5}},
	}
	for _, tt := range tests {
		gotAll, got := narrow(tt.allowedAll, tt.allowed, tt.selected)
		if gotAll != tt.wantAll || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", tt.name, gotAll, got, tt.wantAll, tt.want)
		}
	}
}

func TestScope(t *testing.T) {
	member := 42

	t.Run("administrator is never restricted", func(t *testing.T) {
		// Whatever is stored for an administrator is ignored.
		s := newScope(1, true, Permissions{ReadOnly: true, VisibleTabs: keys("tasks")}, userFacts{})
		if s.ReadOnly || !s.TabVisible("payments") || !s.AllowsProject(123) || !s.AllowsMember(5) {
			t.Errorf("admin scope = %+v", s)
		}
		var args []interface{}
		if cond := s.IssueCond("", &args); cond != "TRUE" || len(args) != 0 {
			t.Errorf("admin condition = %q", cond)
		}
	})

	t.Run("own tasks only", func(t *testing.T) {
		s := newScope(2, false, effective(nil, nil, nil), userFacts{memberID: &member})
		var args []interface{}
		cond := s.IssueCond("i.", &args)
		want := "((i.project_id = ANY($1) AND (i.assigned_to_id IS NULL OR i.assigned_to_id = ANY($2))) OR i.assigned_to_id = $3)"
		if cond != want || len(args) != 3 || args[2] != member {
			t.Errorf("condition = %q, args = %v", cond, args)
		}
		if s.AllowsProject(1) || !s.AllowsMember(member) || s.AllowsMember(7) {
			t.Error("own-only user must see no projects and only themselves")
		}
	})

	t.Run("own tasks without a linked member: nothing", func(t *testing.T) {
		s := newScope(2, false, effective(nil, nil, nil), userFacts{})
		var args []interface{}
		if cond := s.IssueCond("", &args); cond != "(project_id = ANY($1) AND (assigned_to_id IS NULL OR assigned_to_id = ANY($2)))" {
			t.Errorf("condition = %q", cond)
		}
	})

	t.Run("selection narrows, placeholders continue", func(t *testing.T) {
		perms := Permissions{ProjectIDs: []int{1, 2, 3}, AllTeam: true}
		s := newScope(3, false, perms, userFacts{selectedProjects: []int{2, 99}})
		if !reflect.DeepEqual(s.Projects, []int{2}) || !s.AllTeam {
			t.Errorf("scope = %+v", s)
		}
		args := []interface{}{"existing"}
		if cond := s.IssueCond("", &args); cond != "(project_id = ANY($2))" || len(args) != 2 {
			t.Errorf("condition = %q, args = %d", cond, len(args))
		}
		if !s.AllowsProject(3) || s.ShowsProject(3) || s.AllowsProject(99) {
			t.Error("allowed and shown projects are mixed up")
		}
		if !reflect.DeepEqual(s.FilterProjects([]int{3, 99, 1}), []int{1, 3}) {
			t.Errorf("FilterProjects = %v", s.FilterProjects([]int{3, 99, 1}))
		}
	})

	t.Run("hidden tabs", func(t *testing.T) {
		s := newScope(4, false, Permissions{VisibleTabs: keys("tasks", "kanban")}, userFacts{})
		if !s.TabVisible("kanban") || s.TabVisible("payments") {
			t.Error("tab visibility is wrong")
		}
	})

	t.Run("rights as in the data source", func(t *testing.T) {
		facts := userFacts{sourceProjects: []int{20, 10}}

		// The template gives the projects of the source and the whole team
		template := Permissions{FromSource: true, AllTeam: true}
		s := newScope(5, false, effective(nil, &template, nil), facts)
		if s.AllowedAllProjects || !reflect.DeepEqual(s.AllowedProjects, []int{10, 20}) || !s.AllowedAllTeam {
			t.Errorf("scope = %+v", s)
		}

		// Projects given by a group are added to them
		s = newScope(5, false, effective(nil, &template, []Permissions{{ProjectIDs: []int{30}}}), facts)
		if !reflect.DeepEqual(s.AllowedProjects, []int{10, 20, 30}) {
			t.Errorf("with a group: projects = %v", s.AllowedProjects)
		}

		// Without the flag the membership in the source gives nothing
		s = newScope(5, false, Permissions{ProjectIDs: []int{30}}, facts)
		if !reflect.DeepEqual(s.AllowedProjects, []int{30}) {
			t.Errorf("without the flag: projects = %v", s.AllowedProjects)
		}

		// A user not linked to the source yet has no projects there
		s = newScope(5, false, effective(nil, &template, nil), userFacts{})
		if len(s.AllowedProjects) != 0 || s.AllowsProject(10) {
			t.Errorf("not linked: projects = %v", s.AllowedProjects)
		}
	})
}

func TestPermissionsRoundTrip(t *testing.T) {
	p := Permissions{ProjectIDs: []int{3, 1, 3}, VisibleTabs: keys("kanban", "nonsense", "tasks"), ReadOnly: true}
	values := p.Values()

	raw := rawPermissions{}
	raw.projects, raw.team = values[0].([]byte), values[1].([]byte)
	raw.tabs = values[2].([]byte)
	if values[3] != nil {
		t.Errorf("widgets must be stored as NULL, got %v", values[3])
	}
	raw.p.ReadOnly = values[7].(bool)

	got := raw.permissions()
	want := Permissions{ProjectIDs: []int{1, 3}, TeamIDs: []int{}, VisibleTabs: keys("tasks", "kanban"), ReadOnly: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestPrefixed(t *testing.T) {
	if got := prefixed("t", "a, b,c"); got != "t.a, t.b, t.c" {
		t.Errorf("prefixed = %q", got)
	}
}
