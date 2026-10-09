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

	t.Run("groups give no access, their tabs are summed", func(t *testing.T) {
		got := effective(nil, nil, []Permissions{groupA, groupB})
		if len(got.ProjectIDs) != 0 || len(got.TeamIDs) != 0 || got.AllProjects || got.AllTeam {
			t.Errorf("a group must not give access: %v / %v", got.ProjectIDs, got.TeamIDs)
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

	t.Run("individual rights stay the user's own, group tabs have priority", func(t *testing.T) {
		individual := Permissions{ProjectIDs: []int{9}, VisibleTabs: keys("analytics")}
		got := effective(&individual, nil, []Permissions{groupA})
		if !reflect.DeepEqual(got.ProjectIDs, []int{9}) {
			t.Errorf("projects = %v", got.ProjectIDs)
		}
		if !reflect.DeepEqual(*got.VisibleTabs, []string{"tasks"}) {
			t.Errorf("tabs = %v, want the group's", *got.VisibleTabs)
		}
	})

	t.Run("a group hides tabs from a user who has all of them", func(t *testing.T) {
		template := Permissions{AllProjects: true} // tabs: everything
		got := effective(nil, &template, []Permissions{groupA})
		if got.VisibleTabs == nil || !reflect.DeepEqual(*got.VisibleTabs, []string{"tasks"}) {
			t.Errorf("tabs = %v, want [tasks]", got.VisibleTabs)
		}
	})

	t.Run("a group that sets no tabs or widgets has no say in them", func(t *testing.T) {
		individual := Permissions{VisibleTabs: keys("analytics"), Widgets: keys("stats")}
		silent := Permissions{ProjectIDs: []int{1}}
		got := effective(&individual, nil, []Permissions{silent})
		if !reflect.DeepEqual(*got.VisibleTabs, []string{"analytics"}) || !reflect.DeepEqual(*got.Widgets, []string{"stats"}) {
			t.Errorf("tabs = %v, widgets = %v, want the user's own", *got.VisibleTabs, *got.Widgets)
		}
		// Only the group that sets widgets decides them
		withWidgets := Permissions{Widgets: keys("deadlines")}
		got = effective(&individual, nil, []Permissions{silent, withWidgets})
		if !reflect.DeepEqual(*got.Widgets, []string{"deadlines"}) || !reflect.DeepEqual(*got.VisibleTabs, []string{"analytics"}) {
			t.Errorf("tabs = %v, widgets = %v", *got.VisibleTabs, *got.Widgets)
		}
	})

	t.Run("sprint actions: everything by default, a group has priority", func(t *testing.T) {
		everything := newScope(1, false, effective(&Permissions{}, nil, nil), userFacts{})
		if !everything.CanSprint(SprintDelete) {
			t.Error("without a restriction every sprint action is allowed")
		}

		individual := Permissions{SprintActions: keys("create", "edit", "nonsense")}
		own := newScope(1, false, effective(&individual, nil, nil), userFacts{})
		if !own.CanSprint(SprintCreate) || !own.CanSprint(SprintEdit) || own.CanSprint(SprintClose) || own.CanSprint(SprintDelete) {
			t.Errorf("own actions = %v", *own.SprintActions)
		}

		group := Permissions{SprintActions: keys("tasks")}
		viaGroup := newScope(1, false, effective(&individual, nil, []Permissions{group}), userFacts{})
		if !viaGroup.CanSprint(SprintTasks) || viaGroup.CanSprint(SprintCreate) {
			t.Errorf("with a group: actions = %v, want the group's", *viaGroup.SprintActions)
		}

		admin := newScope(1, true, Permissions{SprintActions: keys()}, userFacts{})
		if !admin.CanSprint(SprintDelete) {
			t.Error("an administrator may do everything with sprints")
		}
	})

	t.Run("unassigned issues: any source may give the right", func(t *testing.T) {
		if effective(nil, nil, nil).ShowUnassigned {
			t.Error("off by default")
		}
		got := effective(&Permissions{}, nil, []Permissions{{ShowUnassigned: true}})
		if !got.ShowUnassigned {
			t.Error("a group must be able to give the right")
		}
	})

	t.Run("a group cannot make a read-only user a writer", func(t *testing.T) {
		individual := Permissions{ReadOnly: true}
		got := effective(&individual, nil, []Permissions{{AllProjects: true}})
		if !got.ReadOnly || got.AllProjects {
			t.Errorf("got %+v", got)
		}
	})
}

func TestGroupsNarrow(t *testing.T) {
	scope := func(own Permissions, facts userFacts, groups ...Permissions) *Scope {
		facts.limit = limitOf(groups)
		return newScope(1, false, effective(&own, nil, groups), facts)
	}
	own := Permissions{ProjectIDs: []int{1, 2, 3, 4, 5}, TeamIDs: []int{10, 11, 12, 13}}

	t.Run("a group narrows the user's own rights", func(t *testing.T) {
		s := scope(own, userFacts{}, Permissions{ProjectIDs: []int{2, 3, 4}, TeamIDs: []int{10, 11}})
		if !reflect.DeepEqual(s.AllowedProjects, []int{2, 3, 4}) || !reflect.DeepEqual(s.AllowedTeam, []int{10, 11}) {
			t.Errorf("projects = %v, team = %v", s.AllowedProjects, s.AllowedTeam)
		}
	})

	t.Run("a group never adds anything", func(t *testing.T) {
		s := scope(own, userFacts{}, Permissions{ProjectIDs: []int{3, 99}, TeamIDs: []int{10, 77}})
		if !reflect.DeepEqual(s.AllowedProjects, []int{3}) || !reflect.DeepEqual(s.AllowedTeam, []int{10}) {
			t.Errorf("projects = %v, team = %v", s.AllowedProjects, s.AllowedTeam)
		}
		// Not even to a user with no rights of their own
		none := newScope(1, false, effective(nil, nil, []Permissions{{AllProjects: true, AllTeam: true}}),
			userFacts{limit: limitOf([]Permissions{{AllProjects: true, AllTeam: true}})})
		if none.AllowedAllProjects || len(none.AllowedProjects) != 0 || none.AllowedAllTeam {
			t.Errorf("no own rights: %+v", none)
		}
	})

	t.Run("a group narrows a user who has everything", func(t *testing.T) {
		s := scope(Permissions{AllProjects: true, AllTeam: true}, userFacts{}, Permissions{ProjectIDs: []int{7}, TeamIDs: []int{20}})
		if s.AllowedAllProjects || !reflect.DeepEqual(s.AllowedProjects, []int{7}) || s.AllowedAllTeam || !reflect.DeepEqual(s.AllowedTeam, []int{20}) {
			t.Errorf("scope = %+v", s)
		}
	})

	t.Run("several groups are summed", func(t *testing.T) {
		s := scope(own, userFacts{}, Permissions{ProjectIDs: []int{1}, TeamIDs: []int{10}}, Permissions{ProjectIDs: []int{5}, TeamIDs: []int{13}})
		if !reflect.DeepEqual(s.AllowedProjects, []int{1, 5}) || !reflect.DeepEqual(s.AllowedTeam, []int{10, 13}) {
			t.Errorf("projects = %v, team = %v", s.AllowedProjects, s.AllowedTeam)
		}
		// A group that allows everything lifts the restriction of the others
		s = scope(own, userFacts{}, Permissions{ProjectIDs: []int{1}}, Permissions{AllProjects: true})
		if !reflect.DeepEqual(s.AllowedProjects, []int{1, 2, 3, 4, 5}) {
			t.Errorf("with an unrestricted group: projects = %v", s.AllowedProjects)
		}
	})

	t.Run("a group that names nothing has no say", func(t *testing.T) {
		// A group made only to hide tabs must not take the data away
		s := scope(own, userFacts{}, Permissions{VisibleTabs: keys("tasks")})
		if !reflect.DeepEqual(s.AllowedProjects, []int{1, 2, 3, 4, 5}) || !reflect.DeepEqual(s.AllowedTeam, []int{10, 11, 12, 13}) {
			t.Errorf("projects = %v, team = %v", s.AllowedProjects, s.AllowedTeam)
		}
		// It restricts one side only when it names only that side
		s = scope(own, userFacts{}, Permissions{ProjectIDs: []int{2}})
		if !reflect.DeepEqual(s.AllowedProjects, []int{2}) || !reflect.DeepEqual(s.AllowedTeam, []int{10, 11, 12, 13}) {
			t.Errorf("projects = %v, team = %v", s.AllowedProjects, s.AllowedTeam)
		}
	})

	t.Run("own tasks stay visible whatever the groups say", func(t *testing.T) {
		member := 42
		ownTasks := Permissions{ProjectIDs: []int{1}, TeamIDs: []int{10}, OwnTasksOnly: true}
		s := scope(ownTasks, userFacts{memberID: &member}, Permissions{ProjectIDs: []int{2}, TeamIDs: []int{11}})
		if !s.AllowsMember(member) || s.AllowsMember(10) || s.AllowsProject(1) {
			t.Errorf("scope = %+v", s)
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
		want := "((i.project_id = ANY($1) AND i.assigned_to_id = ANY($2)) OR i.assigned_to_id = $3)"
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
		if cond := s.IssueCond("", &args); cond != "(project_id = ANY($1) AND assigned_to_id = ANY($2))" {
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
		if cond := s.IssueCond("", &args); cond != "(project_id = ANY($2) AND assigned_to_id IS NOT NULL)" || len(args) != 2 {
			t.Errorf("condition = %q, args = %d", cond, len(args))
		}
		if !s.AllowsProject(3) || s.ShowsProject(3) || s.AllowsProject(99) {
			t.Error("allowed and shown projects are mixed up")
		}
		if !reflect.DeepEqual(s.FilterProjects([]int{3, 99, 1}), []int{1, 3}) {
			t.Errorf("FilterProjects = %v", s.FilterProjects([]int{3, 99, 1}))
		}
	})

	t.Run("issues without an assignee", func(t *testing.T) {
		cond := func(p Permissions) string {
			var args []interface{}
			return newScope(6, false, p, userFacts{}).IssueCond("", &args)
		}
		tests := []struct {
			name  string
			perms Permissions
			want  string
		}{
			{"everything, unassigned hidden", Permissions{AllProjects: true, AllTeam: true}, "(assigned_to_id IS NOT NULL)"},
			{"everything, unassigned shown", Permissions{AllProjects: true, AllTeam: true, ShowUnassigned: true}, "TRUE"},
			{"a team, unassigned hidden", Permissions{AllProjects: true, TeamIDs: []int{1}}, "(assigned_to_id = ANY($1))"},
			{"a team, unassigned shown", Permissions{AllProjects: true, TeamIDs: []int{1}, ShowUnassigned: true},
				"((assigned_to_id IS NULL OR assigned_to_id = ANY($1)))"},
		}
		for _, tt := range tests {
			if got := cond(tt.perms); got != tt.want {
				t.Errorf("%s: condition = %q, want %q", tt.name, got, tt.want)
			}
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

		// A group narrows them and cannot open a project the source does not give
		group := []Permissions{{ProjectIDs: []int{20, 30}}}
		withGroup := facts
		withGroup.limit = limitOf(group)
		s = newScope(5, false, effective(nil, &template, group), withGroup)
		if !reflect.DeepEqual(s.AllowedProjects, []int{20}) {
			t.Errorf("with a group: projects = %v", s.AllowedProjects)
		}

		// The source is the ceiling for the team too: of the two members a
		// group allows, only the one the source shows is visible
		visible := []int{7}
		group = []Permissions{{TeamIDs: []int{7, 8}}}
		s = newScope(5, false, effective(nil, &template, group),
			userFacts{sourceProjects: []int{10}, sourceMembers: &visible, limit: limitOf(group)})
		if s.AllowedAllTeam || !reflect.DeepEqual(s.AllowedTeam, []int{7}) {
			t.Errorf("team = %v (all: %v), want [7]", s.AllowedTeam, s.AllowedAllTeam)
		}
		// Until the members are read from the source there is no ceiling yet
		s = newScope(5, false, effective(nil, &template, nil), userFacts{sourceProjects: []int{10}})
		if !s.AllowedAllTeam {
			t.Error("members not read yet: the template's own team must apply")
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
