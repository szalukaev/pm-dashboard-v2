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

// What a user may do with sprints, each allowed separately.
const (
	SprintCreate = "create" // create a sprint
	SprintEdit   = "edit"   // change its name, dates, description
	SprintTasks  = "tasks"  // add and remove its tasks
	SprintClose  = "close"  // close (and reopen) it
	SprintDelete = "delete" // delete it
)

// SprintActions lists the sprint actions in the order they are shown.
var SprintActions = []string{SprintCreate, SprintEdit, SprintTasks, SprintClose, SprintDelete}

// Permissions is one permission set: of a user, a group or a role template.
type Permissions struct {
	ProjectIDs []int `json:"project_ids"`
	TeamIDs    []int `json:"team_ids"`
	// AllProjects / AllTeam: everything, the lists are ignored.
	AllProjects bool `json:"all_projects"`
	AllTeam     bool `json:"all_team"`
	// ShowUnassigned: issues without an assignee are visible too.
	ShowUnassigned bool `json:"show_unassigned"`
	// OwnTasksOnly adds the issues assigned to the user themselves.
	OwnTasksOnly bool `json:"own_tasks_only"`
	// ReadOnly: data may be viewed but not changed.
	ReadOnly bool `json:"read_only"`
	// FromSource adds the projects the user is a member of in the data
	// source ("rights as in Redmine"), kept up to date by the sync.
	FromSource bool `json:"from_source"`
	// VisibleTabs / Widgets: nil = everything, otherwise the allowed keys.
	VisibleTabs *[]string `json:"visible_tabs"`
	Widgets     *[]string `json:"widgets"`
	// SprintActions: nil = every action, otherwise the allowed ones.
	SprintActions *[]string `json:"sprint_actions"`
}

// Columns is the list of permission columns shared by user_permissions,
// group_permissions and role_templates, in the order Scan and Values use.
const Columns = "project_ids, team_ids, visible_tabs, widgets, all_projects, all_team, own_tasks_only, read_only, from_source, show_unassigned, sprint_actions"

// rawPermissions receives the permission columns of a row.
type rawPermissions struct {
	projects, team, tabs, widgets, sprint []byte
	p                                     Permissions
}

// keyList decodes a nullable JSON list of keys; NULL means "everything".
func keyList(raw []byte) *[]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var keys []string
	if json.Unmarshal(raw, &keys) != nil {
		return nil
	}
	return &keys
}

// keyValue encodes a key list for a nullable JSON column.
func keyValue(keys *[]string) interface{} {
	if keys == nil {
		return nil
	}
	data, _ := json.Marshal(*keys)
	return data
}

// dest returns the scan destinations for Columns.
func (r *rawPermissions) dest() []interface{} {
	return []interface{}{&r.projects, &r.team, &r.tabs, &r.widgets,
		&r.p.AllProjects, &r.p.AllTeam, &r.p.OwnTasksOnly, &r.p.ReadOnly, &r.p.FromSource, &r.p.ShowUnassigned,
		&r.sprint}
}

func (r *rawPermissions) permissions() Permissions {
	p := r.p
	json.Unmarshal(r.projects, &p.ProjectIDs)
	json.Unmarshal(r.team, &p.TeamIDs)
	p.VisibleTabs = keyList(r.tabs)
	p.Widgets = keyList(r.widgets)
	p.SprintActions = keyList(r.sprint)
	return p.Normalized()
}

// Values returns the values for Columns, ready for INSERT / UPDATE.
func (p Permissions) Values() []interface{} {
	p = p.Normalized()
	projects, _ := json.Marshal(p.ProjectIDs)
	team, _ := json.Marshal(p.TeamIDs)
	return []interface{}{projects, team, keyValue(p.VisibleTabs), keyValue(p.Widgets),
		p.AllProjects, p.AllTeam, p.OwnTasksOnly, p.ReadOnly, p.FromSource, p.ShowUnassigned,
		keyValue(p.SprintActions)}
}

// Normalized returns the set with sorted lists without duplicates, non-nil
// id lists and only known tab / widget keys.
func (p Permissions) Normalized() Permissions {
	p.ProjectIDs = uniqueInts(p.ProjectIDs)
	p.TeamIDs = uniqueInts(p.TeamIDs)
	p.VisibleTabs = knownKeys(p.VisibleTabs, Tabs)
	p.Widgets = knownKeys(p.Widgets, Widgets)
	p.SprintActions = knownKeys(p.SprintActions, SprintActions)
	return p
}

// union merges permission sets: a user gets everything any of them gives.
// Only access to data is merged. OwnTasksOnly, ReadOnly and FromSource are
// personal and come from the user's own set; tabs, widgets and sprint
// actions follow their own rule (see effective).
func union(sets ...Permissions) Permissions {
	result := Permissions{}
	for _, s := range sets {
		result.ProjectIDs = append(result.ProjectIDs, s.ProjectIDs...)
		result.TeamIDs = append(result.TeamIDs, s.TeamIDs...)
		result.AllProjects = result.AllProjects || s.AllProjects
		result.AllTeam = result.AllTeam || s.AllTeam
		result.ShowUnassigned = result.ShowUnassigned || s.ShowUnassigned
	}
	return result.Normalized()
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
