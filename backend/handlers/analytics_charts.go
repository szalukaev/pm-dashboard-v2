package handlers

import (
	"math"
	"net/http"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// velocityWeeks is how many weeks the velocity chart shows, the current one
// included.
const velocityWeeks = 12

// HoursRow is the hours of one employee over their tasks that are not closed.
type HoursRow struct {
	Name       string  `json:"name"`
	Estimated  float64 `json:"estimated_hours"`
	Spent      float64 `json:"spent_hours"`
	BugHours   float64 `json:"bug_hours"`
	BugPercent float64 `json:"bug_percent"`
}

// VelocityWeek is the tasks closed during one week, Monday to Sunday.
type VelocityWeek struct {
	WeekStart string  `json:"week_start"`
	Closed    int     `json:"closed"`
	Estimated float64 `json:"estimated_hours"`
}

// GetHours returns estimated, spent and bug-fixing hours by employee — the
// same team and tasks as the team load table, closed tasks left out.
func (h *AnalyticsHandler) GetHours(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

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
		COALESCE(SUM(estimated_hours), 0),
		COALESCE(SUM(spent_hours), 0),
		COALESCE(SUM(bug_fix_hours), 0)
		FROM issues WHERE ` + projectFilter + teamFilter + `
		AND ` + statusNotIn("status_id", GroupClosed) + `
		GROUP BY assigned_to_name
		ORDER BY assigned_to_name`

	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	result := []HoursRow{}
	for rows.Next() {
		var row HoursRow
		if rows.Scan(&row.Name, &row.Estimated, &row.Spent, &row.BugHours) == nil {
			row.BugPercent = bugPercent(row.BugHours, row.Spent)
			result = append(result, row)
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"hours": result})
}

// GetVelocity returns how many tasks were closed in each of the last weeks
// and their total estimate. The closing date comes from the data source, so
// the history is there from the first sync.
func (h *AnalyticsHandler) GetVelocity(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args := projectIssues(r)
	now := time.Now()
	period, ok := parsePeriod(r.URL.Query(), now)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	// С периодом показываются его недели, и задачи считаются только в его границах
	weeks := velocityWeeksOf(period, now)
	projectFilter += period.closedCond(&args)

	// A task reopened after closing is not a closed one
	q := `SELECT closed_on, COALESCE(estimated_hours, 0)
		FROM issues WHERE ` + projectFilter + `
		AND closed_on IS NOT NULL AND closed_on >= $` + itoa(len(args)+1) + `
		AND ` + statusIn("status_id", GroupClosed)
	args = append(args, weeks[0])

	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	result := make([]VelocityWeek, len(weeks))
	index := make(map[string]int, len(weeks))
	for i, start := range weeks {
		result[i].WeekStart = start.Format("2006-01-02")
		index[result[i].WeekStart] = i
	}
	for rows.Next() {
		var closedOn time.Time
		var estimated float64
		if rows.Scan(&closedOn, &estimated) != nil {
			continue
		}
		if i, ok := index[weekStart(closedOn.In(time.Local)).Format("2006-01-02")]; ok {
			result[i].Closed++
			result[i].Estimated += estimated
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"weeks": result})
}

// bugPercent is the share of bug-fixing hours in the spent ones, 0–100 with
// one decimal; 0 when nothing was spent.
func bugPercent(bugHours, spent float64) float64 {
	if spent <= 0 || bugHours <= 0 {
		return 0
	}
	return math.Round(bugHours/spent*1000) / 10
}

// weekStart returns the midnight of the Monday of the week t belongs to, in
// the time zone of t.
func weekStart(t time.Time) time.Time {
	// time.Weekday counts from Sunday; a week here starts on Monday
	sinceMonday := (int(t.Weekday()) + 6) % 7
	y, m, d := t.Date()
	return time.Date(y, m, d-sinceMonday, 0, 0, 0, 0, t.Location())
}

// lastWeeks returns the starts of the last n weeks ending with the week of
// now, oldest first.
func lastWeeks(now time.Time, n int) []time.Time {
	current := weekStart(now)
	weeks := make([]time.Time, n)
	for i := range weeks {
		weeks[i] = current.AddDate(0, 0, -7*(n-1-i))
	}
	return weeks
}
