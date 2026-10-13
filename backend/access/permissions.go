// Package access решает, какие данные пользователь видит и может менять.
//
// Права администратор выдаёт лично или через шаблон роли; группы пользователя
// никогда их не добавляют, а только сужают. Сам пользователь может сузить
// итог ещё сильнее в настройках (выбранные проекты и команда), но расширить
// не может. Каждый обработчик, отдающий данные, получает здесь Scope и
// фильтрует по нему в SQL.
package access

import (
	"encoding/json"
	"sort"
)

// Вкладки и виджеты аналитики, которые можно скрыть от пользователя.
var (
	Tabs    = []string{"tasks", "analytics", "kanban", "sprint", "payments"}
	Widgets = []string{"stats", "health", "velocity", "team_load", "hours", "bug_hours", "distribution", "deadlines", "gantt"}
)

// Действия со спринтами; каждое разрешается отдельно.
const (
	SprintCreate = "create" // создать спринт
	SprintEdit   = "edit"   // изменить название, даты, описание
	SprintTasks  = "tasks"  // добавлять и убирать задачи
	SprintClose  = "close"  // закрыть (и открыть снова)
	SprintDelete = "delete" // удалить
)

// SprintActions — действия со спринтами в том порядке, в каком они показаны.
var SprintActions = []string{SprintCreate, SprintEdit, SprintTasks, SprintClose, SprintDelete}

// Виды доски Kanban, каждый разрешается отдельно.
const (
	KanbanUsers    = "users"    // по сотрудникам: колонка на исполнителя
	KanbanStatuses = "statuses" // по проектам: колонки статусов одного проекта
)

// KanbanModes — виды доски в том порядке, в каком они показаны.
var KanbanModes = []string{KanbanUsers, KanbanStatuses}

// Permissions — один набор прав: пользователя, группы или шаблона роли.
type Permissions struct {
	ProjectIDs []int `json:"project_ids"`
	TeamIDs    []int `json:"team_ids"`
	// AllProjects / AllTeam: всё целиком, списки не учитываются.
	AllProjects bool `json:"all_projects"`
	AllTeam     bool `json:"all_team"`
	// ShowUnassigned: видны и задачи без исполнителя.
	ShowUnassigned bool `json:"show_unassigned"`
	// OwnTasksOnly добавляет задачи, назначенные самому пользователю.
	OwnTasksOnly bool `json:"own_tasks_only"`
	// ReadOnly: данные можно смотреть, но не менять.
	ReadOnly bool `json:"read_only"`
	// FromSource добавляет проекты, в которых пользователь состоит в
	// источнике данных («права как в Redmine»); список обновляет синхронизация.
	FromSource bool `json:"from_source"`
	// VisibleTabs / Widgets: nil — всё, иначе разрешённые ключи.
	VisibleTabs *[]string `json:"visible_tabs"`
	Widgets     *[]string `json:"widgets"`
	// SprintActions: nil — все действия, иначе разрешённые.
	SprintActions *[]string `json:"sprint_actions"`
	// KanbanModes: nil = оба вида доски, иначе разрешённые.
	KanbanModes *[]string `json:"kanban_modes"`
}

// Columns — столбцы прав, общие для user_permissions, group_permissions и
// role_templates, в том порядке, в каком их читает Scan и отдаёт Values.
const Columns = "project_ids, team_ids, visible_tabs, widgets, all_projects, all_team, own_tasks_only, read_only, from_source, show_unassigned, sprint_actions, kanban_modes"

// rawPermissions принимает столбцы прав одной строки.
type rawPermissions struct {
	projects, team, tabs, widgets, sprint, kanban []byte
	p                                             Permissions
}

// keyList разбирает JSON-список ключей из столбца, допускающего NULL; NULL — «всё».
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

// keyValue готовит список ключей к записи в JSON-столбец, допускающий NULL.
func keyValue(keys *[]string) interface{} {
	if keys == nil {
		return nil
	}
	data, _ := json.Marshal(*keys)
	return data
}

// dest возвращает приёмники Scan для столбцов Columns.
func (r *rawPermissions) dest() []interface{} {
	return []interface{}{&r.projects, &r.team, &r.tabs, &r.widgets,
		&r.p.AllProjects, &r.p.AllTeam, &r.p.OwnTasksOnly, &r.p.ReadOnly, &r.p.FromSource, &r.p.ShowUnassigned,
		&r.sprint, &r.kanban}
}

func (r *rawPermissions) permissions() Permissions {
	p := r.p
	json.Unmarshal(r.projects, &p.ProjectIDs)
	json.Unmarshal(r.team, &p.TeamIDs)
	p.VisibleTabs = keyList(r.tabs)
	p.Widgets = keyList(r.widgets)
	p.SprintActions = keyList(r.sprint)
	p.KanbanModes = keyList(r.kanban)
	return p.Normalized()
}

// Values возвращает значения столбцов Columns, готовые для INSERT / UPDATE.
func (p Permissions) Values() []interface{} {
	p = p.Normalized()
	projects, _ := json.Marshal(p.ProjectIDs)
	team, _ := json.Marshal(p.TeamIDs)
	return []interface{}{projects, team, keyValue(p.VisibleTabs), keyValue(p.Widgets),
		p.AllProjects, p.AllTeam, p.OwnTasksOnly, p.ReadOnly, p.FromSource, p.ShowUnassigned,
		keyValue(p.SprintActions), keyValue(p.KanbanModes)}
}

// Normalized возвращает набор с отсортированными списками без повторов, с
// непустыми (не nil) списками идентификаторов и только с известными ключами.
func (p Permissions) Normalized() Permissions {
	p.ProjectIDs = uniqueInts(p.ProjectIDs)
	p.TeamIDs = uniqueInts(p.TeamIDs)
	p.VisibleTabs = knownKeys(p.VisibleTabs, Tabs)
	p.Widgets = knownKeys(p.Widgets, Widgets)
	p.SprintActions = knownKeys(p.SprintActions, SprintActions)
	p.KanbanModes = knownKeys(p.KanbanModes, KanbanModes)
	return p
}

// Limit — предел, который задают группы пользователя, отдельно для проектов
// и для команды. Список nil значит, что с этой стороны группы не ограничивают.
//
// Несколько групп складываются: разрешено то, что разрешает хотя бы одна.
// Группа, в которой не названы проекты (или сотрудники), на них не влияет —
// она может существовать только ради скрытия вкладок, — а группа, которая
// разрешает все, снимает ограничение.
type Limit struct {
	Projects *[]int
	Team     *[]int
}

func limitOf(groups []Permissions) Limit {
	side := func(all func(Permissions) bool, list func(Permissions) []int) *[]int {
		var ids []int
		restricted := false
		for _, g := range groups {
			if all(g) {
				return nil
			}
			if len(list(g)) > 0 {
				restricted = true
				ids = append(ids, list(g)...)
			}
		}
		if !restricted {
			return nil
		}
		ids = uniqueInts(ids)
		return &ids
	}
	return Limit{
		Projects: side(func(p Permissions) bool { return p.AllProjects }, func(p Permissions) []int { return p.ProjectIDs }),
		Team:     side(func(p Permissions) bool { return p.AllTeam }, func(p Permissions) []int { return p.TeamIDs }),
	}
}

// restrict сужает то, что есть у пользователя (всё или список), до предела.
func restrict(all bool, list []int, limit *[]int) (bool, []int) {
	switch {
	case limit == nil:
		return all, uniqueInts(list)
	case all:
		return false, uniqueInts(*limit)
	default:
		return false, intersect(uniqueInts(list), *limit)
	}
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

// knownKeys оставляет существующие ключи в принятом порядке; nil остаётся nil.
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
