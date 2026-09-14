package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type AnalyticsHandler struct {
	DB *sql.DB
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

type DeadlineGroup struct {
	Name  string         `json:"name"`
	Tasks []TaskResponse `json:"tasks"`
}

type ProjectDist struct {
	ProjectName string            `json:"project_name"`
	Assignees   []AssigneeCount   `json:"assignees"`
}

type AssigneeCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (h *AnalyticsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	projects := h.getSelectedProjects(userID)
	projectFilter, args := h.buildProjectFilter(projects, 1)

	var stats []StatCard
	today := time.Now().Format("2006-01-02")

	// Active projects
	var activeProjects int
	q := `SELECT COUNT(DISTINCT project_id) FROM issues WHERE ` + projectFilter + `
		AND (LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')
		OR LOWER(status_name) LIKE '%test%')`
	h.DB.QueryRow(q, args...).Scan(&activeProjects)
	stats = append(stats, StatCard{Label: "active_projects", Value: float64(activeProjects)})

	// Completion: closed / (open + testing*0.5 + closed)
	var closed, open, testing int
	q = `SELECT
		COUNT(CASE WHEN LOWER(status_name) IN ('closed','rejected','resolved','tested') THEN 1 END),
		COUNT(CASE WHEN LOWER(status_name) NOT IN ('closed','rejected','resolved','tested') AND LOWER(status_name) NOT LIKE '%test%' THEN 1 END),
		COUNT(CASE WHEN LOWER(status_name) LIKE '%test%' THEN 1 END)
		FROM issues WHERE ` + projectFilter
	h.DB.QueryRow(q, args...).Scan(&closed, &open, &testing)
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
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')`
	overdueArgs := append(args, today)
	h.DB.QueryRow(q, overdueArgs...).Scan(&overdue)
	stats = append(stats, StatCard{Label: "overdue", Value: float64(overdue), Variant: "danger"})

	// Bugs
	var bugs int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND LOWER(status_name) LIKE '%bug%'`
	h.DB.QueryRow(q, args...).Scan(&bugs)
	stats = append(stats, StatCard{Label: "bugs", Value: float64(bugs)})

	// No estimate
	var noEstimate int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND (estimated_hours IS NULL OR estimated_hours = 0)
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')`
	h.DB.QueryRow(q, args...).Scan(&noEstimate)
	stats = append(stats, StatCard{Label: "no_estimate", Value: float64(noEstimate)})

	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

func (h *AnalyticsHandler) GetTeamLoad(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	projects := h.getSelectedProjects(userID)
	team := h.getSelectedTeam(userID)
	today := time.Now().Format("2006-01-02")

	projectFilter, args := h.buildProjectFilter(projects, 1)

	// Get max priority ID for "high priority" check
	var maxPriorityID int
	h.DB.QueryRow("SELECT COALESCE(MAX(external_id), 0) FROM priorities").Scan(&maxPriorityID)

	// Build team filter
	teamFilter := ""
	if len(team) > 0 {
		placeholders := make([]string, len(team))
		for i, mid := range team {
			placeholders[i] = "$" + itoa(len(args)+1)
			args = append(args, mid)
		}
		teamFilter = " AND assigned_to_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	q := `SELECT
		COALESCE(assigned_to_name, 'Неназначенные'),
		COUNT(CASE WHEN LOWER(status_name) NOT IN ('closed','rejected','resolved','tested') AND LOWER(status_name) NOT LIKE '%test%' THEN 1 END),
		COUNT(CASE WHEN LOWER(status_name) LIKE '%test%' THEN 1 END),
		COUNT(CASE WHEN LOWER(status_name) IN ('closed','rejected','resolved','tested') THEN 1 END),
		COUNT(CASE WHEN due_date IS NOT NULL AND due_date < $` + itoa(len(args)+1) + ` AND LOWER(status_name) NOT IN ('closed','rejected','resolved','tested') THEN 1 END),
		COUNT(CASE WHEN LOWER(status_name) LIKE '%bug%' THEN 1 END),
		COUNT(CASE WHEN priority_id = $` + itoa(len(args)+2) + ` THEN 1 END),
		COUNT(CASE WHEN (estimated_hours IS NULL OR estimated_hours = 0) AND LOWER(status_name) NOT IN ('closed','rejected','resolved','tested') THEN 1 END)
		FROM issues WHERE ` + projectFilter + teamFilter + `
		GROUP BY assigned_to_name
		ORDER BY assigned_to_name`

	args = append(args, today, maxPriorityID)
	rows, err := h.DB.Query(q, args...)
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
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	projects := h.getSelectedProjects(userID)
	projectFilter, args := h.buildProjectFilter(projects, 1)

	q := `SELECT project_name, COALESCE(assigned_to_name, 'Неназначенные'), COUNT(*)
		FROM issues WHERE ` + projectFilter + `
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')
		GROUP BY project_name, assigned_to_name
		ORDER BY project_name, assigned_to_name`

	rows, err := h.DB.Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	distMap := make(map[string]*ProjectDist)
	for rows.Next() {
		var projName, assigneeName string
		var count int
		if rows.Scan(&projName, &assigneeName, &count) != nil {
			continue
		}
		if _, ok := distMap[projName]; !ok {
			distMap[projName] = &ProjectDist{ProjectName: projName}
		}
		distMap[projName].Assignees = append(distMap[projName].Assignees, AssigneeCount{
			Name: assigneeName, Count: count,
		})
	}

	dist := make([]ProjectDist, 0, len(distMap))
	for _, d := range distMap {
		dist = append(dist, *d)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"distribution": dist})
}

func (h *AnalyticsHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	projects := h.getSelectedProjects(userID)
	projectFilter, args := h.buildProjectFilter(projects, 1)
	today := time.Now()
	todayStr := today.Format("2006-01-02")
	threeDays := today.AddDate(0, 0, 3).Format("2006-01-02")
	week := today.AddDate(0, 0, 7).Format("2006-01-02")

	// All open tasks with due dates
	q := `SELECT external_id, project_id, project_name, subject, '',
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE ` + projectFilter + `
		AND due_date IS NOT NULL
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')
		ORDER BY due_date ASC`

	// Add today as arg for deadline comparison
	argIdx := len(args) + 1
	q = `SELECT external_id, project_id, project_name, subject, '',
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE ` + projectFilter + `
		AND due_date IS NOT NULL
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')
		ORDER BY due_date ASC`

	_ = argIdx
	rows, err := h.DB.Query(q, args...)
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
		var t TaskResponse
		if rows.Scan(
			&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
			&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
			&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
			&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
			&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
		) != nil {
			continue
		}

		if t.DueDate == nil {
			continue
		}
		dueDate := *t.DueDate

		switch {
		case dueDate < todayStr:
			groups["overdue"].Tasks = append(groups["overdue"].Tasks, t)
		case dueDate == todayStr:
			groups["today"].Tasks = append(groups["today"].Tasks, t)
		case dueDate <= threeDays:
			groups["three_days"].Tasks = append(groups["three_days"].Tasks, t)
		case dueDate <= week:
			groups["week"].Tasks = append(groups["week"].Tasks, t)
		}
	}

	result := []DeadlineGroup{}
	for _, g := range groups {
		if g.Tasks == nil {
			g.Tasks = []TaskResponse{}
		}
		result = append(result, *g)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"deadlines": result})
}

// Helpers

func (h *AnalyticsHandler) getSelectedProjects(userID int) []int64 {
	var spJSON []byte
	h.DB.QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID).Scan(&spJSON)
	var projects []int64
	if len(spJSON) > 0 {
		json.Unmarshal(spJSON, &projects)
	}
	return projects
}

func (h *AnalyticsHandler) getSelectedTeam(userID int) []int64 {
	var stJSON []byte
	h.DB.QueryRow("SELECT selected_team FROM user_settings WHERE user_id = $1", userID).Scan(&stJSON)
	var team []int64
	if len(stJSON) > 0 {
		json.Unmarshal(stJSON, &team)
	}
	return team
}

func (h *AnalyticsHandler) buildProjectFilter(projects []int64, startArg int) (string, []interface{}) {
	if len(projects) == 0 {
		return "1=1", nil
	}
	placeholders := make([]string, len(projects))
	args := make([]interface{}, len(projects))
	for i, pid := range projects {
		placeholders[i] = "$" + strconv.Itoa(startArg+i)
		args[i] = pid
	}
	return "project_id IN (" + strings.Join(placeholders, ",") + ")", args
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
