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
	// Effective is what the sources add up to.
	Effective Permissions `json:"effective"`
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
	g.Effective = effective(g.Individual, templateSet, groupSets)
	return g, nil
}

// Scope returns what the user may see right now.
func (r *Resolver) Scope(userID int) (*Scope, error) {
	db := *r.DB

	var role string
	var blocked bool
	var memberID *int
	err := db.QueryRow("SELECT role, is_blocked, member_id FROM users WHERE id = $1", userID).Scan(&role, &blocked, &memberID)
	if err != nil || blocked {
		return nil, ErrUserNotFound
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

	if role == "admin" {
		return newScope(userID, true, Permissions{}, memberID, selectedProjects, selectedTeam), nil
	}
	grants, err := r.Grants(userID)
	if err != nil {
		return nil, err
	}
	return newScope(userID, false, grants.Effective, memberID, selectedProjects, selectedTeam), nil
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
