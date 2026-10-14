package access

import (
	"strconv"

	"github.com/lib/pq"
)

// effective возвращает права пользователя до того, как их сузят группы.
//
// Собственный набор — личный, либо шаблон роли, если лично права не заданы,
// либо набор по умолчанию: только свои задачи. Доступ к проектам и команде
// берётся только из него: группы ничего не дают, они сужают (см. limitOf и
// newScope).
//
// Для вкладок, виджетов, действий со спринтами и оплатами, видов Kanban
// правило другое: если у пользователя есть группы, решают они — так группа
// может скрыть вкладку от своих участников, что бы ни говорили их
// собственные права. Несколько групп складываются: разрешено то, что
// разрешает хотя бы одна, а группа без ограничения открывает всё.
func effective(individual, template *Permissions, groups []Permissions) Permissions {
	own := Permissions{OwnTasksOnly: true}
	switch {
	case individual != nil:
		own = *individual
	case template != nil:
		own = *template
	}

	result := Permissions{
		ProjectIDs: own.ProjectIDs, TeamIDs: own.TeamIDs,
		AllProjects: own.AllProjects, AllTeam: own.AllTeam,
		ShowUnassigned: own.ShowUnassigned,
	}
	// Право видеть задачи без исполнителя может дать любой источник
	for _, g := range groups {
		result.ShowUnassigned = result.ShowUnassigned || g.ShowUnassigned
	}
	result.VisibleTabs = groupKeys(groups, func(p Permissions) *[]string { return p.VisibleTabs }, own.VisibleTabs)
	result.Widgets = groupKeys(groups, func(p Permissions) *[]string { return p.Widgets }, own.Widgets)
	// Действия со спринтами подчиняются тому же правилу.
	result.SprintActions = groupKeys(groups, func(p Permissions) *[]string { return p.SprintActions }, own.SprintActions)
	// Виды доски Kanban — по тому же правилу
	result.KanbanModes = groupKeys(groups, func(p Permissions) *[]string { return p.KanbanModes }, own.KanbanModes)
	// Действия с оплатами — по тому же правилу
	result.PaymentActions = groupKeys(groups, func(p Permissions) *[]string { return p.PaymentActions }, own.PaymentActions)
	// «Только свои оплаты» — по тому же правилу: при группах действует, только
	// если его ввели все группы, — хватает одной, открывшей все записи. Без
	// групп решают личные права; не задано и там — только свои, как было до
	// этой настройки.
	ownOnly := own.PaymentsOwnOnly == nil || *own.PaymentsOwnOnly
	if len(groups) > 0 {
		ownOnly = true
		for _, g := range groups {
			ownOnly = ownOnly && g.PaymentsOwnOnly != nil && *g.PaymentsOwnOnly
		}
	}
	result.PaymentsOwnOnly = &ownOnly
	// «Свои задачи» только добавляют видимое — их может дать любой источник
	result.OwnTasksOnly = own.OwnTasksOnly
	for _, g := range groups {
		result.OwnTasksOnly = result.OwnTasksOnly || g.OwnTasksOnly
	}
	// «Только чтение»: личное действует всегда, а от групп — только если его
	// ввели все группы: права групп складываются, хватает одной, где
	// изменения разрешены
	result.ReadOnly = own.ReadOnly
	if len(groups) > 0 {
		allReadOnly := true
		for _, g := range groups {
			allReadOnly = allReadOnly && g.ReadOnly
		}
		result.ReadOnly = result.ReadOnly || allReadOnly
	}
	result.FromSource = own.FromSource
	return result.Normalized()
}

// groupKeys возвращает вкладки или виджеты (их выбирает get), заданные
// группами. Права групп складываются: группа без ограничения (nil) открывает
// всё, иначе разрешено то, что разрешает хотя бы одна. Без групп — own.
func groupKeys(groups []Permissions, get func(Permissions) *[]string, own *[]string) *[]string {
	if len(groups) == 0 {
		return own
	}
	result := []string{}
	for _, g := range groups {
		keys := get(g)
		if keys == nil {
			return nil
		}
		result = append(result, *keys...)
	}
	return &result
}

// Scope — то, что пользователь видит прямо сейчас: предел, заданный
// администратором, суженный собственными настройками пользователя.
type Scope struct {
	UserID int
	Admin  bool
	// ReadOnly: менять данные пользователю нельзя.
	ReadOnly bool
	// VisibleTabs / Widgets: nil — всё.
	VisibleTabs *[]string
	Widgets     *[]string
	// SprintActions: nil — разрешены все действия со спринтами.
	SprintActions *[]string
	// KanbanModes: nil = доступны оба вида доски Kanban.
	KanbanModes *[]string
	// PaymentActions: nil — разрешены все действия с оплатами.
	PaymentActions *[]string
	// PaymentsOwnOnly: в «Оплатах» видны только свои записи.
	PaymentsOwnOnly bool

	// Предел, заданный администратором.
	AllowedAllProjects bool
	AllowedProjects    []int
	AllowedAllTeam     bool
	AllowedTeam        []int

	// Что показано: предел, суженный выбранными проектами и командой.
	AllProjects bool
	Projects    []int
	AllTeam     bool
	Team        []int

	// ShowUnassigned: видны задачи без исполнителя.
	ShowUnassigned bool

	// OwnMember: задачи этого сотрудника видны независимо от списков выше
	// («свои задачи»).
	OwnMember *int
}

// userFacts — то, что известно о пользователе помимо наборов прав.
type userFacts struct {
	// memberID: кто этот пользователь в источнике данных; nil — не привязан.
	memberID *int
	// sourceProjects: проекты, в которых пользователь состоит в источнике.
	sourceProjects []int
	// sourceMembers: сотрудники, которых источник показывает пользователю;
	// nil — ещё не прочитаны.
	sourceMembers *[]int
	// limit: предел, который задают группы пользователя (проекты вместе с
	// подпроектами).
	limit Limit
	// selectedProjects (с подпроектами) и selectedTeam: собственный выбор
	// пользователя в настройках.
	selectedProjects []int
	selectedTeam     []int
}

// newScope собирает Scope из действующих прав и того, что известно о
// пользователе. В perms.ProjectIDs уже должны входить подпроекты: право на
// проект — это право на всю его ветку.
func newScope(userID int, admin bool, perms Permissions, facts userFacts) *Scope {
	memberID, selectedProjects, selectedTeam := facts.memberID, facts.selectedProjects, facts.selectedTeam
	s := &Scope{UserID: userID, Admin: admin}
	if admin {
		// Администратор видит всё и ничем не ограничен.
		s.AllowedAllProjects, s.AllowedAllTeam = true, true
		s.ShowUnassigned = true
	} else {
		s.ShowUnassigned = perms.ShowUnassigned
		s.ReadOnly = perms.ReadOnly
		s.VisibleTabs, s.Widgets = perms.VisibleTabs, perms.Widgets
		s.SprintActions = perms.SprintActions
		s.KanbanModes = perms.KanbanModes
		s.PaymentActions = perms.PaymentActions
		s.PaymentsOwnOnly = perms.PaymentsOwnOnly == nil || *perms.PaymentsOwnOnly
		s.AllowedAllProjects, s.AllowedProjects = perms.AllProjects, perms.ProjectIDs
		if perms.FromSource {
			// Ровно проекты источника: участие там задаётся по каждому
			// проекту и на подпроекты здесь не распространяется.
			s.AllowedProjects = uniqueInts(append(append([]int{}, perms.ProjectIDs...), facts.sourceProjects...))
		}
		s.AllowedAllTeam, s.AllowedTeam = perms.AllTeam, perms.TeamIDs
		if perms.FromSource {
			// «Как в источнике» — потолок и для команды: тот, кого источник
			// от пользователя скрывает, не показывается и здесь.
			s.AllowedAllTeam, s.AllowedTeam = restrict(s.AllowedAllTeam, s.AllowedTeam, facts.sourceMembers)
		}
		// Группы сужают то, что есть у пользователя, и ничего не добавляют —
		// после источника, чтобы группа не могла открыть проект или
		// сотрудника, которых источник не даёт.
		s.AllowedAllProjects, s.AllowedProjects = restrict(s.AllowedAllProjects, s.AllowedProjects, facts.limit.Projects)
		s.AllowedAllTeam, s.AllowedTeam = restrict(s.AllowedAllTeam, s.AllowedTeam, facts.limit.Team)
		if perms.OwnTasksOnly && memberID != nil {
			s.OwnMember = memberID
		}
	}

	s.AllProjects, s.Projects = narrow(s.AllowedAllProjects, s.AllowedProjects, selectedProjects)
	s.AllTeam, s.Team = narrow(s.AllowedAllTeam, s.AllowedTeam, selectedTeam)
	return s
}

// narrow применяет выбор пользователя к разрешённому. Пустой выбор значит
// «всё разрешённое»; выбор может только уменьшить его.
func narrow(allowedAll bool, allowed, selected []int) (bool, []int) {
	selected = uniqueInts(selected)
	switch {
	case len(selected) == 0:
		return allowedAll, uniqueInts(allowed)
	case allowedAll:
		return false, selected
	default:
		return false, intersect(selected, allowed)
	}
}

// Три условия ниже отбирают задачи в SQL. alias — префикс таблицы ("" или
// "i."); номера параметров продолжают *args.

// IssueCond: задачи, показываемые в списках, — права, суженные и проектами,
// и командой, которые выбрал пользователь.
func (s *Scope) IssueCond(alias string, args *[]interface{}) string {
	return s.issueCond(alias, args, s.AllProjects, s.Projects, s.AllTeam, s.Team)
}

// ProjectIssueCond: задачи выбранных проектов, на кого бы они ни были
// назначены (в пределах прав), — для цифр и досок «по проектам», которые
// выбранная команда прореживать не должна.
func (s *Scope) ProjectIssueCond(alias string, args *[]interface{}) string {
	return s.issueCond(alias, args, s.AllProjects, s.Projects, s.AllowedAllTeam, s.AllowedTeam)
}

// AllowedIssueCond: все задачи, разрешённые администратором, без учёта
// собственного выбора пользователя, — чтобы открыть или изменить одну задачу.
func (s *Scope) AllowedIssueCond(alias string, args *[]interface{}) string {
	return s.issueCond(alias, args, s.AllowedAllProjects, s.AllowedProjects, s.AllowedAllTeam, s.AllowedTeam)
}

func (s *Scope) issueCond(alias string, args *[]interface{}, allProjects bool, projects []int, allTeam bool, team []int) string {
	cond := ""
	add := func(part string) {
		if cond != "" {
			cond += " AND "
		}
		cond += part
	}
	placeholder := func(value interface{}) string {
		*args = append(*args, value)
		return "$" + strconv.Itoa(len(*args))
	}

	if !allProjects {
		add(alias + "project_id = ANY(" + placeholder(pq.Array(projects)) + ")")
	}
	// Задачи без исполнителя не относятся ни к чьей команде: их видят
	// только те, кому дано такое право.
	assignee := alias + "assigned_to_id"
	switch {
	case !allTeam && s.ShowUnassigned:
		add("(" + assignee + " IS NULL OR " + assignee + " = ANY(" + placeholder(pq.Array(team)) + "))")
	case !allTeam:
		add(assignee + " = ANY(" + placeholder(pq.Array(team)) + ")")
	case !s.ShowUnassigned:
		add(assignee + " IS NOT NULL")
	}
	if cond == "" {
		return "TRUE"
	}
	if s.OwnMember != nil {
		return "((" + cond + ") OR " + alias + "assigned_to_id = " + placeholder(*s.OwnMember) + ")"
	}
	return "(" + cond + ")"
}

// AllowsProject сообщает, дал ли администратор доступ к проекту.
func (s *Scope) AllowsProject(id int) bool {
	return s.AllowedAllProjects || containsInt(s.AllowedProjects, id)
}

// AllowsMember сообщает, вправе ли пользователь видеть сотрудника.
func (s *Scope) AllowsMember(id int) bool {
	return s.AllowedAllTeam || containsInt(s.AllowedTeam, id) || (s.OwnMember != nil && *s.OwnMember == id)
}

// ShowsProject сообщает, входит ли проект в показываемые сейчас.
func (s *Scope) ShowsProject(id int) bool {
	return s.AllProjects || containsInt(s.Projects, id)
}

// TabVisible сообщает, может ли пользователь открыть вкладку.
func (s *Scope) TabVisible(tab string) bool {
	if s.Admin || s.VisibleTabs == nil {
		return true
	}
	for _, t := range *s.VisibleTabs {
		if t == tab {
			return true
		}
	}
	return false
}

// CanSprint сообщает, разрешено ли пользователю действие со спринтами (одна
// из констант Sprint*).
func (s *Scope) CanSprint(action string) bool {
	if s.Admin || s.SprintActions == nil {
		return true
	}
	for _, a := range *s.SprintActions {
		if a == action {
			return true
		}
	}
	return false
}

// CanKanban сообщает, доступен ли пользователю вид доски Kanban (одна из
// констант Kanban*).
func (s *Scope) CanKanban(mode string) bool {
	if s.Admin || s.KanbanModes == nil {
		return true
	}
	for _, m := range *s.KanbanModes {
		if m == mode {
			return true
		}
	}
	return false
}

// CanPayment сообщает, разрешено ли пользователю действие с оплатами (одна
// из констант Payment*).
func (s *Scope) CanPayment(action string) bool {
	if s.Admin || s.PaymentActions == nil {
		return true
	}
	for _, a := range *s.PaymentActions {
		if a == action {
			return true
		}
	}
	return false
}

// PaymentsOwner возвращает автора, которым ограничены записи «Оплат», или
// nil, если пользователю видны все записи.
func (s *Scope) PaymentsOwner() *int {
	if s.Admin || !s.PaymentsOwnOnly {
		return nil
	}
	id := s.UserID
	return &id
}

// FilterProjects оставляет проекты, разрешённые администратором.
func (s *Scope) FilterProjects(ids []int) []int {
	if s.AllowedAllProjects {
		return uniqueInts(ids)
	}
	return intersect(uniqueInts(ids), s.AllowedProjects)
}

// FilterTeam оставляет сотрудников, разрешённых администратором.
func (s *Scope) FilterTeam(ids []int) []int {
	result := []int{}
	for _, id := range uniqueInts(ids) {
		if s.AllowsMember(id) {
			result = append(result, id)
		}
	}
	return result
}
