package handlers

import (
	"math"
	"net/http"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// velocityWeeks — сколько недель показывает график velocity, включая
// текущую.
const velocityWeeks = 12

// HoursRow — часы одного сотрудника по его незакрытым задачам.
type HoursRow struct {
	Name       string  `json:"name"`
	Estimated  float64 `json:"estimated_hours"`
	Spent      float64 `json:"spent_hours"`
	BugHours   float64 `json:"bug_hours"`
	BugPercent float64 `json:"bug_percent"`
}

// VelocityWeek — задачи, закрытые за одну неделю, с понедельника по воскресенье.
type VelocityWeek struct {
	WeekStart string  `json:"week_start"`
	Closed    int     `json:"closed"`
	Estimated float64 `json:"estimated_hours"`
}

// GetHours возвращает оценку, факт и часы на исправление багов по
// сотрудникам — та же команда и те же задачи, что в таблице «Нагрузка на
// команду», без закрытых задач.
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
		// Выбранная команда — это строка на каждого сотрудника, без «неназначенных»
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

// GetVelocity возвращает, сколько задач закрыто в каждую из последних
// недель, и их суммарную оценку. Дата закрытия приходит из источника
// данных, поэтому история есть с первой же синхронизации.
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

	// Задача, открытая заново после закрытия, закрытой не считается
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

// bugPercent — доля часов на исправление багов в фактических, 0–100 с
// одним знаком после запятой; 0, если времени не списано.
func bugPercent(bugHours, spent float64) float64 {
	if spent <= 0 || bugHours <= 0 {
		return 0
	}
	return math.Round(bugHours/spent*1000) / 10
}

// weekStart возвращает полночь понедельника той недели, к которой относится
// t, в часовом поясе t.
func weekStart(t time.Time) time.Time {
	// time.Weekday считает с воскресенья; неделя здесь начинается с понедельника
	sinceMonday := (int(t.Weekday()) + 6) % 7
	y, m, d := t.Date()
	return time.Date(y, m, d-sinceMonday, 0, 0, 0, 0, t.Location())
}

// lastWeeks возвращает начала последних n недель, заканчивая неделей now,
// сначала самые давние.
func lastWeeks(now time.Time, n int) []time.Time {
	current := weekStart(now)
	weeks := make([]time.Time, n)
	for i := range weeks {
		weeks[i] = current.AddDate(0, 0, -7*(n-1-i))
	}
	return weeks
}
