package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// maxThreshold — наибольший порог, который можно задать.
const maxThreshold = 100000

// Thresholds — пороги индикаторов пользователя: при превышении индикатор на
// «Аналитике» подсвечивается. Порог превышен, когда задач больше порога.
type Thresholds struct {
	Overdue    int `json:"overdue"`
	Bugs       int `json:"bugs"`
	NoEstimate int `json:"no_estimate"`
	// PerProject: порог сравнивается с числом задач каждого проекта, а не
	// всех проектов сразу.
	PerProject bool `json:"per_project"`
}

// defaultThresholds — пороги из ТЗ: просрочено > 5, Bugs > 10, без оценки > 3.
func defaultThresholds() Thresholds {
	return Thresholds{Overdue: 5, Bugs: 10, NoEstimate: 3}
}

func (t Thresholds) valid() bool {
	for _, limit := range []int{t.Overdue, t.Bugs, t.NoEstimate} {
		if limit < 0 || limit > maxThreshold {
			return false
		}
	}
	return true
}

// parseThresholds читает сохранённые пороги поверх стандартных.
func parseThresholds(raw []byte) Thresholds {
	thresholds := defaultThresholds()
	if len(raw) > 0 {
		saved := thresholds
		if json.Unmarshal(raw, &saved) == nil && saved.valid() {
			thresholds = saved
		}
	}
	return thresholds
}

// indicatorCounts — значения индикаторов по всем проектам или по одному.
type indicatorCounts struct {
	ProjectID   int
	ProjectName string
	Overdue     int
	Bugs        int
	NoEstimate  int
}

// Exceeded — превышение порога: какой индикатор, сколько задач и какой порог.
// ProjectName пуст, когда порог сравнивался со всеми проектами сразу.
type Exceeded struct {
	Indicator   string `json:"indicator"`
	Value       int    `json:"value"`
	Limit       int    `json:"limit"`
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
}

// exceeded возвращает превышения порогов для набора значений: одного — по
// всем проектам, или по строке на проект.
func exceeded(counts []indicatorCounts, limits Thresholds) []Exceeded {
	result := []Exceeded{}
	for _, c := range counts {
		check := func(indicator string, value, limit int) {
			if value > limit {
				result = append(result, Exceeded{Indicator: indicator, Value: value, Limit: limit, ProjectID: c.ProjectID, ProjectName: c.ProjectName})
			}
		}
		check("overdue", c.Overdue, limits.Overdue)
		check("bugs", c.Bugs, limits.Bugs)
		check("no_estimate", c.NoEstimate, limits.NoEstimate)
	}
	return result
}

// indicators считает индикаторы так же, как карточки статистики: просрочено —
// открытые и в тестировании со сроком раньше сегодня; Bugs — задачи в
// статусах, отмеченных как Bugs; без оценки — открытые без оценки.
// perProject даёт строку на проект, иначе одну строку на все проекты.
func (h *AnalyticsHandler) indicators(scope *access.Scope, now time.Time, perProject bool) ([]indicatorCounts, error) {
	args := []interface{}{}
	filter := scope.ProjectIssueCond("", &args)
	args = append(args, now.Format("2006-01-02"))

	columns := `COUNT(CASE WHEN due_date IS NOT NULL AND due_date < $` + itoa(len(args)) + ` AND ` + statusIn("status_id", GroupOpen, GroupTesting) + ` THEN 1 END),
		COUNT(CASE WHEN ` + bugStatus("status_id") + ` THEN 1 END),
		COUNT(CASE WHEN (estimated_hours IS NULL OR estimated_hours = 0) AND ` + statusIn("status_id", GroupOpen) + ` THEN 1 END)`
	query := `SELECT 0, '', ` + columns + ` FROM issues WHERE ` + filter
	if perProject {
		query = `SELECT COALESCE(project_id, 0), COALESCE(project_name, ''), ` + columns + `
			FROM issues WHERE ` + filter + `
			GROUP BY project_id, project_name ORDER BY project_name, project_id`
	}

	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	rows, err := (*h.DB).Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []indicatorCounts
	for rows.Next() {
		var c indicatorCounts
		if err := rows.Scan(&c.ProjectID, &c.ProjectName, &c.Overdue, &c.Bugs, &c.NoEstimate); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// GetThresholds возвращает пороги пользователя и их превышения сейчас.
func (h *AnalyticsHandler) GetThresholds(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	var raw []byte
	if err := (*h.DB).QueryRow("SELECT indicator_thresholds FROM user_settings WHERE user_id = $1", middleware.GetUserID(r)).Scan(&raw); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Database query failed", "at", "GetThresholds", "error", err)
	}
	limits := parseThresholds(raw)

	counts, err := h.indicators(middleware.GetScope(r), time.Now(), limits.PerProject)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"thresholds": limits, "exceeded": exceeded(counts, limits)})
}
