package access

import (
	"strconv"

	"github.com/lib/pq"
)

// effective combines everything the administrator gave a user.
//
// The user's own set is the individual one, or the role template when rights
// are not set individually, or the default — own tasks only. Groups add to
// it. Individually set tabs and widgets override whatever groups give.
func effective(individual, template *Permissions, groups []Permissions) Permissions {
	own := Permissions{OwnTasksOnly: true}
	sets := groups
	switch {
	case individual != nil:
		own = *individual
		sets = append([]Permissions{own}, groups...)
	case template != nil:
		own = *template
		sets = append([]Permissions{own}, groups...)
	}

	result := union(sets...)
	if individual != nil && individual.VisibleTabs != nil {
		result.VisibleTabs = individual.VisibleTabs
	}
	if individual != nil && individual.Widgets != nil {
		result.Widgets = individual.Widgets
	}
	result.OwnTasksOnly = own.OwnTasksOnly
	result.ReadOnly = own.ReadOnly
	result.FromSource = own.FromSource
	return result.Normalized()
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
	} else {
		s.ReadOnly = perms.ReadOnly
		s.VisibleTabs, s.Widgets = perms.VisibleTabs, perms.Widgets
		s.AllowedAllProjects, s.AllowedProjects = perms.AllProjects, perms.ProjectIDs
		if perms.FromSource {
			// Exactly the projects of the source: membership there is per
			// project and is not extended to subprojects here.
			s.AllowedProjects = uniqueInts(append(append([]int{}, perms.ProjectIDs...), facts.sourceProjects...))
		}
		s.AllowedAllTeam, s.AllowedTeam = perms.AllTeam, perms.TeamIDs
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
	if !allTeam {
		// Issues without an assignee belong to nobody's team and stay visible.
		add("(" + alias + "assigned_to_id IS NULL OR " + alias + "assigned_to_id = ANY(" + placeholder(pq.Array(team)) + "))")
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
