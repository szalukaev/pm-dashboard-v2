package access

import (
	"database/sql"
	"encoding/json"
	"errors"

	"pm-dashboard/datasource"
)

// ErrUserNotFound возвращается для неизвестного или заблокированного пользователя.
var ErrUserNotFound = errors.New("user not found")

// Resolver читает права из базы.
type Resolver struct {
	DB **sql.DB
}

// NamedPermissions — набор прав, пришедший из шаблона или группы.
type NamedPermissions struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Permissions Permissions `json:"permissions"`
}

// Grants — всё, что администратор выдал пользователю, по источникам.
type Grants struct {
	// Individual равен nil, если лично права не заданы.
	Individual *Permissions       `json:"individual"`
	Template   *NamedPermissions  `json:"template"`
	Groups     []NamedPermissions `json:"groups"`
	// Effective — итог по источникам: собственные права пользователя,
	// суженные группами. Сюда не входит то, что добавляют «права как в
	// источнике данных», — это зависит от источника, см. Scope.
	Effective Permissions `json:"effective"`

	// own: права до сужения группами; limit: то, что разрешают группы.
	// Оба — с подпроектами.
	own   Permissions
	limit Limit
}

// Grants загружает источники прав пользователя и их итог.
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
	// Проекты, в которых пользователь состоит в источнике: их дают права и
	// группы «как в источнике»
	var sourceJSON []byte
	if err := db.QueryRow("SELECT source_projects FROM users WHERE id = $1", userID).Scan(&sourceJSON); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var sourceProjects []int
	json.Unmarshal(sourceJSON, &sourceProjects)

	g.own = effective(g.Individual, templateSet, groupSets)
	// Право на проект покрывает все его подпроекты, включая созданные
	// позже: списки раскрываются при каждом чтении, а не при сохранении, —
	// и до того, как сравниваются друг с другом. Проекты источника не
	// раскрываются: участие там задаётся по каждому проекту.
	if g.own.ProjectIDs, err = expand(db, g.own.ProjectIDs); err != nil {
		return nil, err
	}
	expandedGroups := make([]Permissions, len(groupSets))
	for i, set := range groupSets {
		if set.ProjectIDs, err = expand(db, set.ProjectIDs); err != nil {
			return nil, err
		}
		expandedGroups[i] = set
	}
	g.limit = limitOf(expandedGroups, sourceProjects)

	if g.own.FromSource {
		// Итог для экрана прав — ровно проекты источника, как в newScope
		g.own.AllProjects, g.own.ProjectIDs = false, uniqueInts(sourceProjects)
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

// Scope возвращает то, что пользователь видит прямо сейчас.
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
	// Выбранный проект означает всё его поддерево.
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

// UsesSourceProjects сообщает, входят ли в права пользователя проекты
// источника данных, то есть нужно ли поддерживать их список в актуальном виде.
func (r *Resolver) UsesSourceProjects(userID int) bool {
	grants, err := r.Grants(userID)
	return err == nil && grants.own.FromSource
}

// SyncProjects возвращает проекты, задачи которых нужно синхронизировать:
// объединение того, что показано каждому активному пользователю. Тот, кому
// доступны все проекты, но ни один не выбран, ничего не добавляет — весь
// источник данных по умолчанию не синхронизируется.
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

// prefixed ставит псевдоним таблицы перед каждым столбцом списка через запятую.
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
