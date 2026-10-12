package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type AnalyticsHandler struct {
	DB **sql.DB
}

type StatCard struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	IsPct bool    `json:"is_pct"`
	Variant string `json:"variant,omitempty"`
}

type TeamLoadRow struct {
	Name       string `json:"name"`
	Open       int    `json:"open"`
	Testing    int    `json:"testing"`
	Closed     int    `json:"closed"`
	Overdue    int    `json:"overdue"`
	Bugs       int    `json:"bugs"`
	HighPrio   int    `json:"high_priority"`
	NoEstimate int    `json:"no_estimate"`
}

type DeadlineTask struct {
	TaskResponse
	IsHighPriority bool `json:"is_high_priority"`
}

type DeadlineGroup struct {
	Name  string         `json:"name"`
	Tasks []DeadlineTask `json:"tasks"`
}

type ProjectDist struct {
	ProjectID   int               `json:"project_id"`
	ProjectName string            `json:"project_name"`
	// The topmost parent of the project (the project itself when it has
	// none): nested projects can be shown as one chart
	RootID   int    `json:"root_id"`
	RootName string `json:"root_name"`
	Assignees   []AssigneeCount   `json:"assignees"`
}

type AssigneeCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (h *AnalyticsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args := projectIssues(r)

	var stats []StatCard
	today := time.Now().Format("2006-01-02")

	// Active projects: those with open or testing issues
	var activeProjects int
	q := `SELECT COUNT(DISTINCT project_id) FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen, GroupTesting)
	(*h.DB).QueryRow(q, args...).Scan(&activeProjects)
	stats = append(stats, StatCard{Label: "active_projects", Value: float64(activeProjects)})

	// Completion: closed / (open + testing*0.5 + closed)
	var closed, open, testing int
	q = `SELECT
		COUNT(CASE WHEN ` + statusIn("status_id", GroupClosed) + ` THEN 1 END),
		COUNT(CASE WHEN ` + statusIn("status_id", GroupOpen) + ` THEN 1 END),
		COUNT(CASE WHEN ` + statusIn("status_id", GroupTesting) + ` THEN 1 END)
		FROM issues WHERE ` + projectFilter
	(*h.DB).QueryRow(q, args...).Scan(&closed, &open, &testing)
	denom := float64(open) + float64(testing)*0.5 + float64(closed)
	completion := 0.0
	if denom > 0 {
		completion = (float64(closed) / denom) * 100
	}
	stats = append(stats, StatCard{Label: "completion", Value: completion, IsPct: true})

	// Open
	stats = append(stats, StatCard{Label: "open", Value: float64(open)})

	// In test
	stats = append(stats, StatCard{Label: "in_test", Value: float64(testing)})

	// Overdue: open/testing tasks with due_date < today
	var overdue int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND due_date IS NOT NULL AND due_date < $` + itoa(len(args)+1) + `
		AND ` + statusIn("status_id", GroupOpen, GroupTesting)
	overdueArgs := append(args, today)
	(*h.DB).QueryRow(q, overdueArgs...).Scan(&overdue)
	stats = append(stats, StatCard{Label: "overdue", Value: float64(overdue), Variant: "danger"})

	// Bugs
	var bugs int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND ` + bugStatus("status_id")
	(*h.DB).QueryRow(q, args...).Scan(&bugs)
	stats = append(stats, StatCard{Label: "bugs", Value: float64(bugs)})

	// No estimate
	var noEstimate int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND (estimated_hours IS NULL OR estimated_hours = 0)
		AND ` + statusIn("status_id", GroupOpen)
	(*h.DB).QueryRow(q, args...).Scan(&noEstimate)
	stats = append(stats, StatCard{Label: "no_estimate", Value: float64(noEstimate)})

	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

func (h *AnalyticsHandler) GetTeamLoad(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	today := time.Now().Format("2006-01-02")

	// The team the user tracks, within rights
	scope := middleware.GetScope(r)
	args := []interface{}{}
	projectFilter := scope.IssueCond("", &args)
	teamFilter := ""
	if !scope.AllTeam {
		// A chosen team means a row per member, without "unassigned"
		teamFilter = " AND assigned_to_id IS NOT NULL"
	}

	q := `SELECT
		COALESCE(assigned_to_name, 'Неназначенные'),
		COUNT(CASE WHEN ` + statusIn("status_id", GroupOpen) + ` THEN 1 END),
		COUNT(CASE WHEN ` + statusIn("status_id", GroupTesting) + ` THEN 1 END),
		COUNT(CASE WHEN ` + statusIn("status_id", GroupClosed) + ` THEN 1 END),
		COUNT(CASE WHEN due_date IS NOT NULL AND due_date < $` + itoa(len(args)+1) + ` AND ` + statusIn("status_id", GroupOpen, GroupTesting) + ` THEN 1 END),
		COUNT(CASE WHEN ` + bugStatus("status_id") + ` THEN 1 END),
		COUNT(CASE WHEN ` + highPriority("priority_id") + ` THEN 1 END),
		COUNT(CASE WHEN (estimated_hours IS NULL OR estimated_hours = 0) AND ` + statusIn("status_id", GroupOpen) + ` THEN 1 END)
		FROM issues WHERE ` + projectFilter + teamFilter + `
		GROUP BY assigned_to_name
		ORDER BY assigned_to_name`

	args = append(args, today)
	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var rows_data []TeamLoadRow
	for rows.Next() {
		var tr TeamLoadRow
		if rows.Scan(&tr.Name, &tr.Open, &tr.Testing, &tr.Closed, &tr.Overdue, &tr.Bugs, &tr.HighPrio, &tr.NoEstimate) == nil {
			rows_data = append(rows_data, tr)
		}
	}
	if rows_data == nil {
		rows_data = []TeamLoadRow{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"team": rows_data})
}

func (h *AnalyticsHandler) GetDistribution(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args := projectIssues(r)

	q := `SELECT COALESCE(project_id, 0), project_name, COALESCE(assigned_to_name, 'Неназначенные'), COUNT(*)
		FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen) + `
		GROUP BY project_id, project_name, assigned_to_name
		ORDER BY project_name, project_id, assigned_to_name`

	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	// The rows come sorted by project: a project is its run of rows, and the
	// projects keep the order of their names
	dist := []ProjectDist{}
	for rows.Next() {
		var projID, count int
		var projName, assigneeName string
		if rows.Scan(&projID, &projName, &assigneeName, &count) != nil {
			continue
		}
		if n := len(dist); n == 0 || dist[n-1].ProjectID != projID || dist[n-1].ProjectName != projName {
			dist = append(dist, ProjectDist{ProjectID: projID, ProjectName: projName})
		}
		last := &dist[len(dist)-1]
		last.Assignees = append(last.Assignees, AssigneeCount{Name: assigneeName, Count: count})
	}

	// Where each project stands in the tree of projects
	parents := map[int]int{}
	names := map[int]string{}
	if tree, err := (*h.DB).Query(`SELECT external_id, name, COALESCE(parent_id, 0) FROM projects WHERE data_source = 'redmine'`); err == nil {
		for tree.Next() {
			var id, parent int
			var name string
			if tree.Scan(&id, &name, &parent) == nil {
				parents[id] = parent
				names[id] = name
			}
		}
		tree.Close()
	}
	for i := range dist {
		root := projectRoot(dist[i].ProjectID, parents)
		dist[i].RootID, dist[i].RootName = root, names[root]
		if root == dist[i].ProjectID || dist[i].RootName == "" {
			dist[i].RootID, dist[i].RootName = dist[i].ProjectID, dist[i].ProjectName
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"distribution": dist})
}

func (h *AnalyticsHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args := projectIssues(r)
	today := time.Now()

	// All tasks that are not closed and have a due date
	q := `SELECT external_id, project_id, project_name, subject, '',
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0),
		COALESCE(` + highPriority("priority_id") + `, false)
		FROM issues WHERE ` + projectFilter + `
		AND due_date IS NOT NULL
		AND ` + statusNotIn("status_id", GroupClosed) + `
		ORDER BY due_date ASC`

	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	groups := map[string]*DeadlineGroup{
		"overdue":    {Name: "overdue"},
		"today":      {Name: "today"},
		"three_days": {Name: "three_days"},
		"week":       {Name: "week"},
	}

	for rows.Next() {
		var t DeadlineTask
		if rows.Scan(
			&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
			&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
			&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
			&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
			&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
			&t.IsHighPriority,
		) != nil {
			continue
		}

		if t.DueDate == nil {
			continue
		}
		if g, ok := groups[deadlineBucket(*t.DueDate, today)]; ok {
			g.Tasks = append(g.Tasks, t)
		}
	}

	result := []DeadlineGroup{}
	for _, name := range deadlineOrder {
		g := groups[name]
		if g.Tasks == nil {
			g.Tasks = []DeadlineTask{}
		}
		result = append(result, *g)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"deadlines": result})
}

// Helpers

// projectRoot returns the topmost parent of a project; parents maps a project
// to its parent, 0 or a missing entry meaning "no parent". A project whose
// parent is unknown is its own root. A loop in the data ends the climb.
func projectRoot(id int, parents map[int]int) int {
	seen := map[int]bool{id: true}
	for {
		parent, known := parents[id]
		if !known || parent == 0 || seen[parent] {
			return id
		}
		if _, exists := parents[parent]; !exists {
			return id
		}
		seen[parent] = true
		id = parent
	}
}

// deadlineOrder is the order of the deadline columns on the screen.
var deadlineOrder = []string{"overdue", "today", "three_days", "week"}

// deadlineBucket returns the deadline column of a due date (YYYY-MM-DD, a
// longer timestamp is cut), or "" when it is more than a week away:
// overdue — before today, three_days — the next 3 days, week — days 4 to 7.
func deadlineBucket(dueDate string, today time.Time) string {
	if len(dueDate) > 10 {
		dueDate = dueDate[:10]
	}
	day := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }
	switch {
	case dueDate < day(0):
		return "overdue"
	case dueDate == day(0):
		return "today"
	case dueDate <= day(3):
		return "three_days"
	case dueDate <= day(7):
		return "week"
	}
	return ""
}

// projectIssues returns the condition and its args selecting the issues of
// the user's projects within rights — the base of every "by projects" figure.
func projectIssues(r *http.Request) (string, []interface{}) {
	args := []interface{}{}
	cond := middleware.GetScope(r).ProjectIssueCond("", &args)
	return cond, args
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
