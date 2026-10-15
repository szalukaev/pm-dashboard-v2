package handlers

import (
	"net/http"
	"strconv"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// analyticsTasksPage — сколько задач на странице списка, стоящего за цифрой.
const analyticsTasksPage = 25

// analyticsTaskCond возвращает условие, отбирающее задачи, посчитанные
// цифрой вкладки «Аналитика», — то самое, по которому цифра считается,
// поэтому задач в списке всегда столько, сколько говорит цифра. todayParam —
// номер параметра с сегодняшней датой. ok равен false для неизвестной цифры.
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

// GetTasks возвращает задачи, стоящие за цифрой, по проектам, постранично:
// ?metric= — цифра, ?offset= — первая задача страницы. Без ?assignee= это
// задачи карточки статистики; с ним — ячейки таблицы «Нагрузка на команду»
// в строке этого сотрудника.
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

	// Те же задачи, по которым считалась цифра
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
		filter += " AND " + assigneeLabel + " = $" + itoa(len(args))
	} else {
		filter, args = projectIssues(r)
	}

	// Тот же период, что у цифры: иначе список закрытых разошёлся бы с числом
	now := time.Now()
	period, periodOK := parsePeriod(query, now)
	if !periodOK {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	filter += period.updatedCond(&args)

	args = append(args, now.Format("2006-01-02"))
	cond, ok := analyticsTaskCond(query.Get("metric"), "$"+itoa(len(args)))
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_METRIC")
		return
	}
	// Дата — параметр каждого запроса, нужна она цифре или нет
	filter += " AND (" + cond + ") AND $" + itoa(len(args)) + "::date IS NOT NULL"

	var total int
	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	if err := (*h.DB).QueryRow("SELECT COUNT(*) FROM issues WHERE "+filter, args...).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	rows, err := (*h.DB).Query(`SELECT external_id, COALESCE(project_id, 0), COALESCE(project_name, ''), COALESCE(subject, ''),
		COALESCE(status_name, ''), COALESCE(status_id, 0), COALESCE(priority_name, ''), COALESCE(priority_id, 0),
		COALESCE(assigned_to_name, ''), assigned_to_id,
		estimated_hours, COALESCE(spent_hours, 0), COALESCE(bug_fix_hours, 0), due_date
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
			&t.EstimatedHours, &t.SpentHours, &t.BugFixHours, &t.DueDate) == nil {
			tasks = append(tasks, t)
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks, "total": total, "limit": analyticsTasksPage, "offset": offset,
	})
}
