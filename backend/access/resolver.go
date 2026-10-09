package access

import (
	"database/sql"
	"encoding/json"
	"errors"

	"pm-dashboard/datasource"
)

// ErrUserNotFound is returned for an unknown or blocked user.
var ErrUserNotFound = errors.New("user not found")

// Resolver reads permissions from the database.
type Resolver struct {
	DB **sql.DB
}

// NamedPermissions is a permission set that came from a template or a group.
type NamedPermissions struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Permissions Permissions `json:"permissions"`
}

// Grants is everything the administrator gave a user, by source.
type Grants struct {
	// Individual is nil when rights are not set individually.
	Individual *Permissions       `json:"individual"`
	Template   *NamedPermissions  `json:"template"`
	Groups     []NamedPermissions `json:"groups"`
	// Effective is what the sources come to: the user's own rights narrowed
	// by the groups. It does not include what "rights as in the data
	// source" adds — that depends on the source, see Scope.
	Effective Permissions `json:"effective"`

	// own: the rights before the groups narrow them; limit: what the groups
	// allow. Both with subprojects.
	own   Permissions
	limit Limit
}

// Grants loads the permission sources of a user and their sum.
func (r *Resolver) Grants(userID int) (*Grants, error) {
	db := *r.DB
	g := &Grants{Groups: []NamedPermissions{}}

	var raw rawPermissions
	err := db.QueryRow("SELECT "+Columns+" FROM user_permissions WHERE user_id = $1", userID).Scan(raw.dest()...)
	switch {
	case err == nil:
		p := raw.permissions()
		g.Individual = &p
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}

	var template NamedPermissions
	raw = rawPermissions{}
	err = db.QueryRow(`SELECT t.id, t.name, `+prefixed("t", Columns)+`
		FROM users u JOIN role_templates t ON t.id = u.role_template_id WHERE u.id = $1`, userID).
		Scan(append([]interface{}{&template.ID, &template.Name}, raw.dest()...)...)
	switch {
	case err == nil:
		template.Permissions = raw.permissions()
		g.Template = &template
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}

	rows, err := db.Query(`SELECT g.id, g.name, `+prefixed("p", Columns)+`
		FROM group_members m JOIN groups g ON g.id = m.group_id
		JOIN group_permissions p ON p.group_id = g.id
		WHERE m.user_id = $1 ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groupSets []Permissions
	for rows.Next() {
		var group NamedPermissions
		raw = rawPermissions{}
		if err := rows.Scan(append([]interface{}{&group.ID, &group.Name}, raw.dest()...)...); err != nil {
			return nil, err
		}
		group.Permissions = raw.permissions()
		g.Groups = append(g.Groups, group)
		groupSets = append(groupSets, group.Permissions)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var templateSet *Permissions
	if g.Template != nil {
		templateSet = &g.Template.Permissions
	}
	g.own = effective(g.Individual, templateSet, groupSets)
	g.limit = limitOf(groupSets)
	// A right to a project covers all its subprojects, including the ones
	// created later: the lists are expanded every time, not when they are
	// saved — and before they are compared with each other.
	if g.own.ProjectIDs, err = expand(db, g.own.ProjectIDs); err != nil {
		return nil, err
	}
	if g.limit.Projects != nil {
		expanded, err := expand(db, *g.limit.Projects)
		if err != nil {
			return nil, err
		}
		g.limit.Projects = &expanded
	}

	g.Effective = g.own
	g.Effective.AllProjects, g.Effective.ProjectIDs = restrict(g.own.AllProjects, g.own.ProjectIDs, g.limit.Projects)
	g.Effective.AllTeam, g.Effective.TeamIDs = restrict(g.own.AllTeam, g.own.TeamIDs, g.limit.Team)
	return g, nil
}

func expand(db *sql.DB, projects []int) ([]int, error) {
	if len(projects) == 0 {
		return projects, nil
	}
	return datasource.ExpandProjects(db, projects)
}

// Scope returns what the user may see right now.
func (r *Resolver) Scope(userID int) (*Scope, error) {
	db := *r.DB

	var role string
	var blocked bool
	var facts userFacts
	var sourceProjectsJSON, sourceMembersJSON []byte
	err := db.QueryRow("SELECT role, is_blocked, member_id, source_projects, source_members FROM users WHERE id = $1", userID).
		Scan(&role, &blocked, &facts.memberID, &sourceProjectsJSON, &sourceMembersJSON)
	if err != nil || blocked {
		return nil, ErrUserNotFound
	}
	json.Unmarshal(sourceProjectsJSON, &facts.sourceProjects)
	if len(sourceMembersJSON) > 0 && string(sourceMembersJSON) != "null" {
		members := []int{}
		json.Unmarshal(sourceMembersJSON, &members)
		facts.sourceMembers = &members
	}

	var projectsJSON, teamJSON []byte
	db.QueryRow("SELECT selected_projects, selected_team FROM user_settings WHERE user_id = $1", userID).Scan(&projectsJSON, &teamJSON)
	var selectedProjects, selectedTeam []int
	json.Unmarshal(projectsJSON, &selectedProjects)
	json.Unmarshal(teamJSON, &selectedTeam)
	// A selected project stands for its whole subtree.
	if len(selectedProjects) > 0 {
		if expanded, err := datasource.ExpandProjects(db, selectedProjects); err == nil {
			selectedProjects = expanded
		}
	}

	facts.selectedProjects, facts.selectedTeam = selectedProjects, selectedTeam

	if role == "admin" {
		return newScope(userID, true, Permissions{}, facts), nil
	}
	grants, err := r.Grants(userID)
	if err != nil {
		return nil, err
	}
	facts.limit = grants.limit
	return newScope(userID, false, grants.own, facts), nil
}

// UsesSourceProjects reports whether the rights of the user include the
// projects of the data source, i.e. whether they must be kept up to date.
func (r *Resolver) UsesSourceProjects(userID int) bool {
	grants, err := r.Grants(userID)
	return err == nil && grants.own.FromSource
}

// SyncProjects returns the projects whose issues must be synced: the union
// of what every active user is shown. A user who may see every project and
// selected none adds nothing — the whole data source is not synced by default.
func (r *Resolver) SyncProjects() ([]int, error) {
	rows, err := (*r.DB).Query("SELECT id FROM users WHERE NOT is_blocked")
	if err != nil {
		return nil, err
	}
	var userIDs []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			userIDs = append(userIDs, id)
		}
	}
	rows.Close()

	var projects []int
	for _, id := range userIDs {
		scope, err := r.Scope(id)
		if err != nil {
			continue
		}
		if !scope.AllProjects {
			projects = append(projects, scope.Projects...)
		}
	}
	return uniqueInts(projects), nil
}

// prefixed puts a table alias before every column of a comma separated list.
func prefixed(alias, columns string) string {
	result := ""
	start := 0
	for i := 0; i <= len(columns); i++ {
		if i == len(columns) || columns[i] == ',' {
			name := columns[start:i]
			for len(name) > 0 && name[0] == ' ' {
				name = name[1:]
			}
			if result != "" {
				result += ", "
			}
			result += alias + "." + name
			start = i + 1
		}
	}
	return result
}
