package access

import (
	"strconv"

	"github.com/lib/pq"
)

// effective returns the rights of a user before the groups narrow them.
//
// The user's own set is the individual one, or the role template when rights
// are not set individually, or the default — own tasks only. Access to
// projects and to the team comes from this set alone: groups give nothing,
// they narrow it (see limitOf and newScope).
//
// Tabs, widgets and sprint actions follow another rule: a group that sets
// them has priority, so that a group can hide a tab from its members
// whatever their own rights say. Several such groups are summed with each
// other; a group that leaves them at "everything" has no say.
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
	// Seeing issues without an assignee is a right any source may give
	for _, g := range groups {
		result.ShowUnassigned = result.ShowUnassigned || g.ShowUnassigned
	}
	result.VisibleTabs = groupKeys(groups, func(p Permissions) *[]string { return p.VisibleTabs }, own.VisibleTabs)
	result.Widgets = groupKeys(groups, func(p Permissions) *[]string { return p.Widgets }, own.Widgets)
	// What may be done with sprints follows the same rule.
	result.SprintActions = groupKeys(groups, func(p Permissions) *[]string { return p.SprintActions }, own.SprintActions)
	result.OwnTasksOnly = own.OwnTasksOnly
	result.ReadOnly = own.ReadOnly
	result.FromSource = own.FromSource
	return result.Normalized()
}

// groupKeys returns the tabs or widgets (picked by get) the groups set: the
// sum over the groups that set them at all, or own when no group does.
func groupKeys(groups []Permissions, get func(Permissions) *[]string, own *[]string) *[]string {
	var result *[]string
	for _, g := range groups {
		keys := get(g)
		if keys == nil {
			continue
		}
		if result == nil {
			result = &[]string{}
		}
		*result = append(*result, *keys...)
	}
	if result == nil {
		return own
	}
	return result
}

// Scope is what one user may see right now: the maximum set by the
// administrator, narrowed by the user's own settings.
type Scope struct {
	UserID int
	Admin  bool
	// ReadOnly: the user may not change data.
	ReadOnly bool
	// VisibleTabs / Widgets: nil = everything.
	VisibleTabs *[]string
	Widgets     *[]string
	// SprintActions: nil = every action with sprints is allowed.
	SprintActions *[]string

	// The maximum set by the administrator.
	AllowedAllProjects bool
	AllowedProjects    []int
	AllowedAllTeam     bool
	AllowedTeam        []int

	// What is shown: the maximum narrowed by the selected projects and team.
	AllProjects bool
	Projects    []int
	AllTeam     bool
	Team        []int

	// ShowUnassigned: issues without an assignee are visible.
	ShowUnassigned bool

	// OwnMember: issues assigned to this member are visible regardless of
	// the lists above ("own tasks").
	OwnMember *int
}

// userFacts is what is known about the user besides the permission sets.
type userFacts struct {
	// memberID: who the user is in the data source, nil when not linked.
	memberID *int
	// sourceProjects: the projects the user is a member of in the source.
	sourceProjects []int
	// sourceMembers: the members the source shows to the user; nil when
	// they have not been read yet.
	sourceMembers *[]int
	// limit: the most the groups of the user allow (projects with their
	// subprojects).
	limit Limit
	// selectedProjects (with subprojects) and selectedTeam: the user's own
	// choice in the settings.
	selectedProjects []int
	selectedTeam     []int
}

// newScope builds the scope from effective permissions and what is known
// about the user. perms.ProjectIDs must already include subprojects: a right
// to a project is a right to its whole branch.
func newScope(userID int, admin bool, perms Permissions, facts userFacts) *Scope {
	memberID, selectedProjects, selectedTeam := facts.memberID, facts.selectedProjects, facts.selectedTeam
	s := &Scope{UserID: userID, Admin: admin}
	if admin {
		// An administrator sees everything and is never restricted.
		s.AllowedAllProjects, s.AllowedAllTeam = true, true
		s.ShowUnassigned = true
	} else {
		s.ShowUnassigned = perms.ShowUnassigned
		s.ReadOnly = perms.ReadOnly
		s.VisibleTabs, s.Widgets = perms.VisibleTabs, perms.Widgets
		s.SprintActions = perms.SprintActions
		s.AllowedAllProjects, s.AllowedProjects = perms.AllProjects, perms.ProjectIDs
		if perms.FromSource {
			// Exactly the projects of the source: membership there is per
			// project and is not extended to subprojects here.
			s.AllowedProjects = uniqueInts(append(append([]int{}, perms.ProjectIDs...), facts.sourceProjects...))
		}
		s.AllowedAllTeam, s.AllowedTeam = perms.AllTeam, perms.TeamIDs
		if perms.FromSource {
			// "As in the source" is a ceiling for the team too: nobody the
			// source hides from the user is shown here.
			s.AllowedAllTeam, s.AllowedTeam = restrict(s.AllowedAllTeam, s.AllowedTeam, facts.sourceMembers)
		}
		// The groups narrow what the user has and never add to it — after
		// the source, so that a group cannot open a project or a member
		// the source does not give.
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

// narrow applies the user's selection to what is allowed. No selection means
// "everything allowed"; a selection can only reduce it.
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

// The three conditions below select issues in SQL. alias is the table prefix
// ("" or "i."); placeholders continue *args.

// IssueCond: the issues shown in lists — rights narrowed by both the
// projects and the team the user selected.
func (s *Scope) IssueCond(alias string, args *[]interface{}) string {
	return s.issueCond(alias, args, s.AllProjects, s.Projects, s.AllTeam, s.Team)
}

// ProjectIssueCond: the issues of the selected projects whoever they are
// assigned to (within rights) — for figures and boards "by projects", which
// the selected team must not thin out.
func (s *Scope) ProjectIssueCond(alias string, args *[]interface{}) string {
	return s.issueCond(alias, args, s.AllProjects, s.Projects, s.AllowedAllTeam, s.AllowedTeam)
}

// AllowedIssueCond: every issue the administrator allows, ignoring the
// user's own selection — for opening or changing one particular issue.
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
	// Issues without an assignee belong to nobody's team: they are shown
	// only to those given the right to see them.
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

// AllowsProject reports whether the administrator gave access to the project.
func (s *Scope) AllowsProject(id int) bool {
	return s.AllowedAllProjects || containsInt(s.AllowedProjects, id)
}

// AllowsMember reports whether the user may see the member of the team.
func (s *Scope) AllowsMember(id int) bool {
	return s.AllowedAllTeam || containsInt(s.AllowedTeam, id) || (s.OwnMember != nil && *s.OwnMember == id)
}

// ShowsProject reports whether the project is among those shown now.
func (s *Scope) ShowsProject(id int) bool {
	return s.AllProjects || containsInt(s.Projects, id)
}

// TabVisible reports whether the user may open the tab.
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

// CanSprint reports whether the user may do the action with sprints (one of
// the Sprint* constants).
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

// FilterProjects keeps the ids the administrator allows.
func (s *Scope) FilterProjects(ids []int) []int {
	if s.AllowedAllProjects {
		return uniqueInts(ids)
	}
	return intersect(uniqueInts(ids), s.AllowedProjects)
}

// FilterTeam keeps the member ids the administrator allows.
func (s *Scope) FilterTeam(ids []int) []int {
	result := []int{}
	for _, id := range uniqueInts(ids) {
		if s.AllowsMember(id) {
			result = append(result, id)
		}
	}
	return result
}
