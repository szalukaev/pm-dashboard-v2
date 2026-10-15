package handlers

import (
	"net/http"
	"strconv"

	"pm-dashboard/utils"
)

// ganttLimit — сколько задач показывает одна страница диаграммы.
const ganttLimit = 25

// GanttTask — полоса диаграммы от Start до Due (YYYY-MM-DD).
type GanttTask struct {
	ExternalID     int    `json:"external_id"`
	Subject        string `json:"subject"`
	ProjectID      int    `json:"project_id"`
	ProjectName    string `json:"project_name"`
	AssignedToName string `json:"assigned_to_name"`
	// Часы, заложенные на задачу; null — оценки нет
	EstimatedHours *float64 `json:"estimated_hours"`
	Start          string   `json:"start"`
	Due            string   `json:"due"`
}

type GanttProject struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GetGantt возвращает незакрытые задачи со сроком — и просроченные, и
// запланированные, по сроку, постранично — и проекты, к которым они
// относятся, для фильтра. ?project_id= оставляет задачи одного проекта,
// ?offset= — первая задача страницы.
func (h *AnalyticsHandler) GetGantt(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args, ok := periodIssues(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	base := projectFilter + `
		AND due_date IS NOT NULL
		AND ` + statusNotIn("status_id", GroupClosed)

	projects := []GanttProject{}
	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	rows, err := (*h.DB).Query(`SELECT DISTINCT project_id, COALESCE(project_name, '')
		FROM issues WHERE `+base+` AND project_id IS NOT NULL
		ORDER BY 2, 1`, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	for rows.Next() {
		var p GanttProject
		if rows.Scan(&p.ID, &p.Name) == nil {
			projects = append(projects, p)
		}
	}
	_ = rows.Close()

	// Фильтр только сужает то, что пользователь и так вправе видеть
	taskFilter := base
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, "INVALID_PROJECT")
			return
		}
		args = append(args, id)
		taskFilter += ` AND project_id = $` + itoa(len(args))
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	var total int
	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	if err := (*h.DB).QueryRow(`SELECT COUNT(*) FROM issues WHERE `+taskFilter, args...).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	// Задача без даты начала начинается в день создания; начало позже
	// срока показывается полосой в один день на дате срока.
	q := `SELECT external_id, COALESCE(subject, ''), COALESCE(project_id, 0), COALESCE(project_name, ''),
		COALESCE(assigned_to_name, ''),
		to_char(LEAST(COALESCE(start_date, created_on::date, due_date), due_date), 'YYYY-MM-DD'),
		to_char(due_date, 'YYYY-MM-DD'), NULLIF(estimated_hours, 0)
		FROM issues WHERE ` + taskFilter + `
		ORDER BY due_date ASC, external_id ASC
		LIMIT ` + itoa(ganttLimit) + ` OFFSET ` + itoa(offset)

	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	taskRows, err := (*h.DB).Query(q, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer taskRows.Close()

	tasks := []GanttTask{}
	for taskRows.Next() {
		var t GanttTask
		if taskRows.Scan(&t.ExternalID, &t.Subject, &t.ProjectID, &t.ProjectName, &t.AssignedToName, &t.Start, &t.Due, &t.EstimatedHours) == nil {
			tasks = append(tasks, t)
		}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks, "projects": projects, "total": total, "limit": ganttLimit, "offset": offset,
	})
}
