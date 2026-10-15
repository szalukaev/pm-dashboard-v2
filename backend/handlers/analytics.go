package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type AnalyticsHandler struct {
	DB **sql.DB
}

type StatCard struct {
	Label   string  `json:"label"`
	Value   float64 `json:"value"`
	IsPct   bool    `json:"is_pct"`
	Variant string  `json:"variant,omitempty"`
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
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
	// Самый верхний родитель проекта среди проектов ответа (сам проект, если
	// родителя нет): вложенные проекты можно показать одной диаграммой
	RootID    int             `json:"root_id"`
	RootName  string          `json:"root_name"`
	Assignees []AssigneeCount `json:"assignees"`
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

	now := time.Now()
	period, ok := parsePeriod(r.URL.Query(), now)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": h.stats(middleware.GetScope(r), now, period)})
}

// stats считает карточки статистики по проектам пользователя в пределах его
// прав. Этим же расчётом пользуется письмо со сводкой. period оставляет
// задачи, изменённые в периоде.
func (h *AnalyticsHandler) stats(scope *access.Scope, now time.Time, period Period) []StatCard {
	args := []interface{}{}
	projectFilter := scope.ProjectIssueCond("", &args)
	projectFilter += period.updatedCond(&args)

	var stats []StatCard
	today := now.Format("2006-01-02")

	// Активные проекты: те, где есть открытые задачи или задачи в тестировании
	var activeProjects int
	q := `SELECT COUNT(DISTINCT project_id) FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen, GroupTesting)
	(*h.DB).QueryRow(q, args...).Scan(&activeProjects)
	stats = append(stats, StatCard{Label: "active_projects", Value: float64(activeProjects)})

	// Выполнение: закрытые / (открытые + в тестировании × 0,5 + закрытые)
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

	// Открытые
	stats = append(stats, StatCard{Label: "open", Value: float64(open)})

	// В тестировании
	stats = append(stats, StatCard{Label: "in_test", Value: float64(testing)})

	// Просрочено: открытые и тестируемые задачи с due_date раньше сегодняшнего дня
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

	// Без оценки
	var noEstimate int
	q = `SELECT COUNT(*) FROM issues WHERE ` + projectFilter + `
		AND (estimated_hours IS NULL OR estimated_hours = 0)
		AND ` + statusIn("status_id", GroupOpen)
	(*h.DB).QueryRow(q, args...).Scan(&noEstimate)
	stats = append(stats, StatCard{Label: "no_estimate", Value: float64(noEstimate)})

	return stats
}

func (h *AnalyticsHandler) GetTeamLoad(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	now := time.Now()
	period, ok := parsePeriod(r.URL.Query(), now)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	team, err := h.teamLoad(middleware.GetScope(r), now, period)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"team": team})
}

// teamLoad считает нагрузку на команду, которую пользователь отслеживает, в
// пределах его прав. Этим же расчётом пользуется письмо со сводкой.
func (h *AnalyticsHandler) teamLoad(scope *access.Scope, now time.Time, period Period) ([]TeamLoadRow, error) {
	today := now.Format("2006-01-02")

	// Команда, за которой следит пользователь, в пределах прав
	args := []interface{}{}
	projectFilter := scope.IssueCond("", &args)
	// Только задачи, изменённые в периоде
	projectFilter += period.updatedCond(&args)
	teamFilter := ""
	if !scope.AllTeam {
		// Выбранная команда — это строка на каждого сотрудника, без «неназначенных»
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
		return nil, err
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

	return rows_data, nil
}

func (h *AnalyticsHandler) GetDistribution(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	period, ok := parsePeriod(r.URL.Query(), time.Now())
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	dist, err := h.distribution(middleware.GetScope(r), period)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"distribution": dist})
}

// distribution считает открытые задачи по проектам и сотрудникам в пределах
// прав пользователя. Этим же расчётом пользуется письмо со сводкой.
func (h *AnalyticsHandler) distribution(scope *access.Scope, period Period) ([]ProjectDist, error) {
	args := []interface{}{}
	projectFilter := scope.ProjectIssueCond("", &args) + period.updatedCond(&args)

	q := `SELECT COALESCE(project_id, 0), project_name, COALESCE(assigned_to_name, 'Неназначенные'), COUNT(*)
		FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen) + `
		GROUP BY project_id, project_name, assigned_to_name
		ORDER BY project_name, project_id, assigned_to_name`

	rows, err := (*h.DB).Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Строки приходят отсортированными по проекту: проект — это его
	// подряд идущие строки, а проекты идут в порядке названий
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

	// Где каждый проект стоит в дереве проектов
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
	shown := make(map[int]bool, len(dist))
	for _, d := range dist {
		shown[d.ProjectID] = true
		names[d.ProjectID] = d.ProjectName
	}
	for i := range dist {
		root := projectRoot(dist[i].ProjectID, parents, shown)
		dist[i].RootID, dist[i].RootName = root, names[root]
	}

	return dist, nil
}

func (h *AnalyticsHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	now := time.Now()
	period, ok := parsePeriod(r.URL.Query(), now)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	result, err := h.deadlines(middleware.GetScope(r), now, period)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"deadlines": result})
}

// deadlines раскладывает незакрытые задачи со сроком по срочности в пределах
// прав пользователя. Этим же расчётом пользуется письмо со сводкой.
func (h *AnalyticsHandler) deadlines(scope *access.Scope, today time.Time, period Period) ([]DeadlineGroup, error) {
	args := []interface{}{}
	projectFilter := scope.ProjectIssueCond("", &args) + period.updatedCond(&args)

	// Все незакрытые задачи со сроком
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
		return nil, err
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

	return result, nil
}

// Вспомогательное

// projectRoot возвращает самого верхнего родителя проекта среди показанных
// либо сам проект, если ни один его родитель не показан. parents
// сопоставляет проекту его родителя; 0 или отсутствие записи — «родителя нет».
//
// Корнем может быть только показанный проект: общий проект-«зонтик», у
// которого нет своей диаграммы, не должен собрать в одну диаграмму все
// проекты под ним. Цикл в данных обрывает подъём.
func projectRoot(id int, parents map[int]int, shown map[int]bool) int {
	root := id
	seen := map[int]bool{id: true}
	for {
		parent := parents[id]
		if parent == 0 || seen[parent] {
			return root
		}
		if shown[parent] {
			root = parent
		}
		seen[parent] = true
		id = parent
	}
}

// deadlineOrder — порядок колонок дедлайнов на экране.
var deadlineOrder = []string{"overdue", "today", "three_days", "week"}

// deadlineBucket возвращает колонку дедлайнов для срока (YYYY-MM-DD, более
// длинная отметка времени обрезается) или "", если до срока больше недели:
// overdue — раньше сегодняшнего дня, three_days — ближайшие 3 дня, week — дни с 4-го по 7-й.
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

// projectIssues возвращает условие и его параметры, отбирающие задачи
// проектов пользователя в пределах прав, — основу всех цифр «по проектам».
func projectIssues(r *http.Request) (string, []interface{}) {
	args := []interface{}{}
	cond := middleware.GetScope(r).ProjectIssueCond("", &args)
	return cond, args
}

// periodIssues — projectIssues плюс задачи, изменённые в периоде из
// запроса; ok = false, если период задан неверно
func periodIssues(r *http.Request) (string, []interface{}, bool) {
	period, ok := parsePeriod(r.URL.Query(), time.Now())
	if !ok {
		return "", nil, false
	}
	cond, args := projectIssues(r)
	return cond + period.updatedCond(&args), args, true
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
