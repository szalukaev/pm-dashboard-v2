package handlers

import (
	"net/http"
	"strconv"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// analyticsTasksPage is how many tasks a page of the list behind a figure has.
const analyticsTasksPage = 25

// analyticsTaskCond returns the condition that selects the tasks counted by a
// figure of the Analytics tab — the very condition the figure is counted
// with, so the list always has as many tasks as the figure says. todayParam
// is the placeholder of today's date. ok is false for an unknown figure.
func analyticsTaskCond(metric, todayParam string) (cond string, ok bool) {
	switch metric {
	case "open":
		return statusIn("status_id", GroupOpen), true
	case "testing":
		return statusIn("status_id", GroupTesting), true
	case "closed":
		return statusIn("status_id", GroupClosed), true
	case "overdue":
		return "due_date IS NOT NULL AND due_date < " + todayParam + " AND " + statusIn("status_id", GroupOpen, GroupTesting), true
	case "bugs":
		return bugStatus("status_id"), true
	case "high_priority":
		return highPriority("priority_id"), true
	case "no_estimate":
		return "(estimated_hours IS NULL OR estimated_hours = 0) AND " + statusIn("status_id", GroupOpen), true
	}
	return "", false
}

// GetTasks returns the tasks behind a figure, sorted by project, a page at a
// time: ?metric= is the figure, ?offset= the first task of the page.
// Without ?assignee= these are the tasks of a card of the statistics; with
// it — of a cell of the team load table in the row of that employee.
func (h *AnalyticsHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	query := r.URL.Query()
	offset, _ := strconv.Atoi(query.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// The same tasks the figure was counted over
	var filter string
	var args []interface{}
	if _, byEmployee := query["assignee"]; byEmployee {
		scope := middleware.GetScope(r)
		args = []interface{}{}
		filter = scope.IssueCond("", &args)
		if !scope.AllTeam {
			filter += " AND assigned_to_id IS NOT NULL"
		}
		args = append(args, query.Get("assignee"))
		filter += " AND COALESCE(assigned_to_name, 'Неназначенные') = $" + itoa(len(args))
	} else {
		filter, args = projectIssues(r)
	}

	args = append(args, time.Now().Format("2006-01-02"))
	cond, ok := analyticsTaskCond(query.Get("metric"), "$"+itoa(len(args)))
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_METRIC")
		return
	}
	// The date is a parameter of every query, used by the figure or not
	filter += " AND (" + cond + ") AND $" + itoa(len(args)) + "::date IS NOT NULL"

	var total int
	if err := (*h.DB).QueryRow("SELECT COUNT(*) FROM issues WHERE "+filter, args...).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	rows, err := (*h.DB).Query(`SELECT external_id, COALESCE(project_id, 0), COALESCE(project_name, ''), COALESCE(subject, ''),
		COALESCE(status_name, ''), COALESCE(status_id, 0), COALESCE(priority_name, ''), COALESCE(priority_id, 0),
		COALESCE(assigned_to_name, ''), assigned_to_id,
		estimated_hours, COALESCE(spent_hours, 0), COALESCE(bug_fix_hours, 0)
		FROM issues WHERE `+filter+`
		ORDER BY project_name ASC NULLS LAST, project_id ASC, external_id DESC
		LIMIT `+itoa(analyticsTasksPage)+` OFFSET `+itoa(offset), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	tasks := []TaskResponse{}
	for rows.Next() {
		var t TaskResponse
		if rows.Scan(&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject,
			&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
			&t.AssignedToName, &t.AssignedToID,
			&t.EstimatedHours, &t.SpentHours, &t.BugFixHours) == nil {
			tasks = append(tasks, t)
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks, "total": total, "limit": analyticsTasksPage, "offset": offset,
	})
}
