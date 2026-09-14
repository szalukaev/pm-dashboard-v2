package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type SprintHandler struct {
	DB *sql.DB
}

type Sprint struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	ProjectName      *string `json:"project_name"`
	Status           string  `json:"status"`
	StartDate        *string `json:"start_date"`
	DueDate          *string `json:"due_date"`
	Description      *string `json:"description"`
	CategoryName     *string `json:"category_name"`
	AutoFillCategory bool    `json:"auto_fill_category"`
	Tasks            []SprintTask `json:"tasks"`
	TaskCount        int     `json:"task_count"`
	OpenCount        int     `json:"open_count"`
	TestingCount     int     `json:"testing_count"`
	ClosedCount      int     `json:"closed_count"`
	Progress         float64 `json:"progress"`
}

type SprintTask struct {
	ExternalID     int     `json:"external_id"`
	Subject        string  `json:"subject"`
	ProjectName    string  `json:"project_name"`
	StatusName     string  `json:"status_name"`
	PriorityName   string  `json:"priority_name"`
	PriorityID     int     `json:"priority_id"`
	AssignedToName string  `json:"assigned_to_name"`
	DueDate        *string `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	IsOverdue      bool    `json:"is_overdue"`
}

func (h *SprintHandler) ListSprints(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	rows, err := h.DB.Query(`SELECT id, name, project_name, status, start_date, due_date,
		description, category_name, auto_fill_category
		FROM sprints WHERE user_id = $1 ORDER BY
		CASE status WHEN 'active' THEN 0 WHEN 'open' THEN 1 ELSE 2 END,
		due_date NULLS LAST`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var sprints []Sprint
	for rows.Next() {
		var s Sprint
		if rows.Scan(&s.ID, &s.Name, &s.ProjectName, &s.Status, &s.StartDate, &s.DueDate,
			&s.Description, &s.CategoryName, &s.AutoFillCategory) != nil {
			continue
		}
		s.Tasks = h.getSprintTasks(s.ID)
		s.TaskCount = len(s.Tasks)
		s.calculateProgress()
		sprints = append(sprints, s)
	}

	if sprints == nil {
		sprints = []Sprint{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"sprints": sprints})
}

func (h *SprintHandler) GetSprint(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var s Sprint
	err := h.DB.QueryRow(`SELECT id, name, project_name, status, start_date, due_date,
		description, category_name, auto_fill_category
		FROM sprints WHERE id = $1 AND user_id = $2`, sprintID, userID).Scan(
		&s.ID, &s.Name, &s.ProjectName, &s.Status, &s.StartDate, &s.DueDate,
		&s.Description, &s.CategoryName, &s.AutoFillCategory)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	s.Tasks = h.getSprintTasks(s.ID)
	s.TaskCount = len(s.Tasks)
	s.calculateProgress()

	utils.JSON(w, http.StatusOK, s)
}

func (h *SprintHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var body struct {
		Name             string  `json:"name"`
		ProjectName      *string `json:"project_name"`
		StartDate        *string `json:"start_date"`
		DueDate          *string `json:"due_date"`
		Description      *string `json:"description"`
		CategoryName     *string `json:"category_name"`
		AutoFillCategory bool    `json:"auto_fill_category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if body.Name == "" {
		utils.Error(w, http.StatusBadRequest, "NAME_REQUIRED")
		return
	}

	var sprintID int
	err := h.DB.QueryRow(`INSERT INTO sprints (user_id, name, project_name, start_date, due_date,
		description, category_name, auto_fill_category)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		userID, body.Name, body.ProjectName, body.StartDate, body.DueDate,
		body.Description, body.CategoryName, body.AutoFillCategory).Scan(&sprintID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}

	// Auto-fill if project + category specified
	if body.AutoFillCategory && body.ProjectName != nil && body.CategoryName != nil {
		h.autoFill(sprintID, *body.ProjectName, *body.CategoryName)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": sprintID, "success": true})
}

func (h *SprintHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	allowed := map[string]bool{
		"name": true, "project_name": true, "status": true,
		"start_date": true, "due_date": true, "description": true,
		"category_name": true, "auto_fill_category": true,
	}

	sets := []string{}
	args := []interface{}{}
	argIdx := 1
	for k, v := range body {
		if allowed[k] {
			sets = append(sets, k+" = $"+strconv.Itoa(argIdx))
			args = append(args, v)
			argIdx++
		}
	}

	if len(sets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS")
		return
	}

	args = append(args, sprintID, userID)
	query := "UPDATE sprints SET " + utils.JoinStrings(sets, ",") + " WHERE id = $" + strconv.Itoa(argIdx) + " AND user_id = $" + strconv.Itoa(argIdx+1)
	_, err := h.DB.Exec(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])

	_, err := h.DB.Exec("DELETE FROM sprints WHERE id = $1 AND user_id = $2", sprintID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var body struct {
		IssueExternalID int `json:"issue_external_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	_, err := h.DB.Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, sprintID, body.IssueExternalID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) UnassignTask(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])
	issueID, _ := strconv.Atoi(mux.Vars(r)["issueId"])

	_, err := h.DB.Exec("DELETE FROM sprint_issues WHERE sprint_id = $1 AND issue_external_id = $2", sprintID, issueID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UNASSIGN_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) GetBacklog(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	// Get user's selected projects
	var selectedProjects []int64
	var spJSON []byte
	h.DB.QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID).Scan(&spJSON)
	if len(spJSON) > 0 {
		json.Unmarshal(spJSON, &selectedProjects)
	}

	// Find issues NOT in any sprint
	where := []string{"LOWER(i.status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')"}
	args := []interface{}{}
	argIdx := 1

	if len(selectedProjects) > 0 {
		placeholders := make([]string, len(selectedProjects))
		for i, pid := range selectedProjects {
			placeholders[i] = "$" + strconv.Itoa(argIdx)
			args = append(args, pid)
			argIdx++
		}
		where = append(where, "i.project_id IN ("+utils.JoinStrings(placeholders, ",")+")")
	}

	query := `SELECT i.external_id, i.subject, i.project_name, i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours
		FROM issues i
		WHERE ` + utils.JoinStrings(where, " AND ") + `
		AND i.external_id NOT IN (SELECT issue_external_id FROM sprint_issues)
		ORDER BY i.project_name, i.priority_id DESC`

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	// Group by project
	groups := make(map[string][]SprintTask)
	for rows.Next() {
		var t SprintTask
		if rows.Scan(&t.ExternalID, &t.Subject, &t.ProjectName, &t.StatusName,
			&t.PriorityName, &t.PriorityID, &t.AssignedToName, &t.DueDate, &t.EstimatedHours) != nil {
			continue
		}
		projName := t.ProjectName
		if projName == "" {
			projName = "Без проекта"
		}
		groups[projName] = append(groups[projName], t)
	}

	type BacklogGroup struct {
		ProjectName string       `json:"project_name"`
		Tasks       []SprintTask `json:"tasks"`
		TaskCount   int          `json:"task_count"`
	}
	var result []BacklogGroup
	for name, tasks := range groups {
		result = append(result, BacklogGroup{ProjectName: name, Tasks: tasks, TaskCount: len(tasks)})
	}
	if result == nil {
		result = []BacklogGroup{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"backlog": result})
}

func (h *SprintHandler) RefreshSprint(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var s Sprint
	err := h.DB.QueryRow(`SELECT id, project_name, category_name, auto_fill_category
		FROM sprints WHERE id = $1 AND user_id = $2`, sprintID, userID).Scan(
		&s.ID, &s.ProjectName, &s.CategoryName, &s.AutoFillCategory)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	if s.AutoFillCategory && s.ProjectName != nil && s.CategoryName != nil {
		h.autoFill(s.ID, *s.ProjectName, *s.CategoryName)
	}

	utils.Success(w)
}

func (h *SprintHandler) getSprintTasks(sprintID int) []SprintTask {
	rows, err := h.DB.Query(`SELECT i.external_id, i.subject, i.project_name, i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours
		FROM sprint_issues si JOIN issues i ON si.issue_external_id = i.external_id
		WHERE si.sprint_id = $1 ORDER BY i.priority_id DESC`, sprintID)
	if err != nil {
		return []SprintTask{}
	}
	defer rows.Close()

	var tasks []SprintTask
	for rows.Next() {
		var t SprintTask
		if rows.Scan(&t.ExternalID, &t.Subject, &t.ProjectName, &t.StatusName,
			&t.PriorityName, &t.PriorityID, &t.AssignedToName, &t.DueDate, &t.EstimatedHours) == nil {
			tasks = append(tasks, t)
		}
	}
	if tasks == nil {
		return []SprintTask{}
	}
	return tasks
}

func (h *SprintHandler) autoFill(sprintID int, projectName, categoryName string) {
	// Find issues matching project + category not already in sprint
	rows, err := h.DB.Query(`SELECT external_id FROM issues
		WHERE project_name = $1 AND category_name = $2
		AND LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')
		AND external_id NOT IN (SELECT issue_external_id FROM sprint_issues WHERE sprint_id = $3)`,
		projectName, categoryName, sprintID)
	if err != nil {
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var issueID int
		if rows.Scan(&issueID) == nil {
			h.DB.Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				sprintID, issueID)
			count++
		}
	}
}

func (s *Sprint) calculateProgress() {
	for _, t := range s.Tasks {
		sn := t.StatusName
		switch {
		case isClosedStatus(sn):
			s.ClosedCount++
		case isTestingStatus(sn):
			s.TestingCount++
			s.OpenCount++ // testing counts as 0.5 open
		default:
			s.OpenCount++
		}
	}

	total := len(s.Tasks)
	if total > 0 {
		s.Progress = (float64(s.ClosedCount) + float64(s.TestingCount)*0.5) / float64(total) * 100
	}
}

func isClosedStatus(name string) bool {
	closed := []string{"closed", "rejected", "resolved", "tested"}
	for _, c := range closed {
		if name == c {
			return true
		}
	}
	return false
}

func isTestingStatus(name string) bool {
	return len(name) >= 4 && (name[:4] == "test" || name[:4] == "Test")
}
