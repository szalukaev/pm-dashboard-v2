// Package access decides what data a user may see and change.
//
// The administrator sets the maximum: individually, through a role template
// or through groups. The user may narrow it in their settings (selected
// projects and team) but can never widen it. Every handler that returns data
// asks this package for a Scope and filters by it in SQL.
package access

import (
	"encoding/json"
	"sort"
)

// Tabs and analytics widgets that can be hidden from a user.
var (
	Tabs    = []string{"tasks", "analytics", "kanban", "sprint", "payments"}
	Widgets = []string{"stats", "team_load", "distribution", "deadlines"}
)

// Permissions is one permission set: of a user, a group or a role template.
type Permissions struct {
	ProjectIDs []int `json:"project_ids"`
	TeamIDs    []int `json:"team_ids"`
	// AllProjects / AllTeam: everything, the lists are ignored.
	AllProjects bool `json:"all_projects"`
	AllTeam     bool `json:"all_team"`
	// OwnTasksOnly adds the issues assigned to the user themselves.
	OwnTasksOnly bool `json:"own_tasks_only"`
	// ReadOnly: data may be viewed but not changed.
	ReadOnly bool `json:"read_only"`
	// VisibleTabs / Widgets: nil = everything, otherwise the allowed keys.
	VisibleTabs *[]string `json:"visible_tabs"`
	Widgets     *[]string `json:"widgets"`
}

// Columns is the list of permission columns shared by user_permissions,
// group_permissions and role_templates, in the order Scan and Values use.
const Columns = "project_ids, team_ids, visible_tabs, widgets, all_projects, all_team, own_tasks_only, read_only"

// rawPermissions receives the permission columns of a row.
type rawPermissions struct {
	projects, team, tabs, widgets []byte
	p                             Permissions
}

// dest returns the scan destinations for Columns.
func (r *rawPermissions) dest() []interface{} {
	return []interface{}{&r.projects, &r.team, &r.tabs, &r.widgets,
		&r.p.AllProjects, &r.p.AllTeam, &r.p.OwnTasksOnly, &r.p.ReadOnly}
}

func (r *rawPermissions) permissions() Permissions {
	p := r.p
	json.Unmarshal(r.projects, &p.ProjectIDs)
	json.Unmarshal(r.team, &p.TeamIDs)
	if len(r.tabs) > 0 && string(r.tabs) != "null" {
		var tabs []string
		if json.Unmarshal(r.tabs, &tabs) == nil {
			p.VisibleTabs = &tabs
		}
	}
	if len(r.widgets) > 0 && string(r.widgets) != "null" {
		var widgets []string
		if json.Unmarshal(r.widgets, &widgets) == nil {
			p.Widgets = &widgets
		}
	}
	return p.Normalized()
}

// Values returns the values for Columns, ready for INSERT / UPDATE.
func (p Permissions) Values() []interface{} {
	p = p.Normalized()
	projects, _ := json.Marshal(p.ProjectIDs)
	team, _ := json.Marshal(p.TeamIDs)
	var tabs, widgets interface{}
	if p.VisibleTabs != nil {
		data, _ := json.Marshal(*p.VisibleTabs)
		tabs = data
	}
	if p.Widgets != nil {
		data, _ := json.Marshal(*p.Widgets)
		widgets = data
	}
	return []interface{}{projects, team, tabs, widgets, p.AllProjects, p.AllTeam, p.OwnTasksOnly, p.ReadOnly}
}

// Normalized returns the set with sorted lists without duplicates, non-nil
// id lists and only known tab / widget keys.
func (p Permissions) Normalized() Permissions {
	p.ProjectIDs = uniqueInts(p.ProjectIDs)
	p.TeamIDs = uniqueInts(p.TeamIDs)
	p.VisibleTabs = knownKeys(p.VisibleTabs, Tabs)
	p.Widgets = knownKeys(p.Widgets, Widgets)
	return p
}

// union merges permission sets: a user gets everything any of them gives.
// Only data access and visibility are merged; OwnTasksOnly and ReadOnly are
// personal and come from the user's own set.
func union(sets ...Permissions) Permissions {
	result := Permissions{VisibleTabs: &[]string{}, Widgets: &[]string{}}
	for _, s := range sets {
		result.ProjectIDs = append(result.ProjectIDs, s.ProjectIDs...)
		result.TeamIDs = append(result.TeamIDs, s.TeamIDs...)
		result.AllProjects = result.AllProjects || s.AllProjects
		result.AllTeam = result.AllTeam || s.AllTeam
		result.VisibleTabs = unionKeys(result.VisibleTabs, s.VisibleTabs)
		result.Widgets = unionKeys(result.Widgets, s.Widgets)
	}
	if len(sets) == 0 {
		result.VisibleTabs, result.Widgets = nil, nil
	}
	return result.Normalized()
}

// unionKeys merges two key lists where nil means "everything".
func unionKeys(a, b *[]string) *[]string {
	if a == nil || b == nil {
		return nil
	}
	merged := append(append([]string{}, *a...), *b...)
	return &merged
}

func uniqueInts(list []int) []int {
	seen := make(map[int]bool, len(list))
	result := make([]int, 0, len(list))
	for _, v := range list {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	sort.Ints(result)
	return result
}

// knownKeys keeps the keys that exist, in the canonical order; nil stays nil.
func knownKeys(keys *[]string, known []string) *[]string {
	if keys == nil {
		return nil
	}
	has := make(map[string]bool, len(*keys))
	for _, k := range *keys {
		has[k] = true
	}
	result := []string{}
	for _, k := range known {
		if has[k] {
			result = append(result, k)
		}
	}
	return &result
}

func intersect(a, b []int) []int {
	in := make(map[int]bool, len(b))
	for _, v := range b {
		in[v] = true
	}
	result := []int{}
	for _, v := range a {
		if in[v] {
			result = append(result, v)
		}
	}
	return result
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
