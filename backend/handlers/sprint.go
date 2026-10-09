package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type SprintHandler struct {
	DB **sql.DB
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

	statusGroup string // group of the status, see status_groups.go
}

func (h *SprintHandler) ListSprints(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	rows, err := (*h.DB).Query(`SELECT id, name, project_name, status, start_date, due_date,
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
		s.Tasks = h.getSprintTasks(s.ID, middleware.GetScope(r))
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
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var s Sprint
	err = (*h.DB).QueryRow(`SELECT id, name, project_name, status, start_date, due_date,
		description, category_name, auto_fill_category
		FROM sprints WHERE id = $1 AND user_id = $2`, sprintID, userID).Scan(
		&s.ID, &s.Name, &s.ProjectName, &s.Status, &s.StartDate, &s.DueDate,
		&s.Description, &s.CategoryName, &s.AutoFillCategory)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	s.Tasks = h.getSprintTasks(s.ID, middleware.GetScope(r))
	s.TaskCount = len(s.Tasks)
	s.calculateProgress()

	utils.JSON(w, http.StatusOK, s)
}

func (h *SprintHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
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

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		utils.Error(w, http.StatusBadRequest, "NAME_REQUIRED")
		return
	}
	body.StartDate = emptyToNil(body.StartDate)
	if body.StartDate == nil {
		utils.Error(w, http.StatusBadRequest, "START_DATE_REQUIRED")
		return
	}
	body.DueDate = emptyToNil(body.DueDate)
	body.ProjectName = emptyToNil(body.ProjectName)
	body.CategoryName = emptyToNil(body.CategoryName)
	body.Description = emptyToNil(body.Description)

	var sprintID int
	err := (*h.DB).QueryRow(`INSERT INTO sprints (user_id, name, project_name, start_date, due_date,
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
		h.autoFill(sprintID, *body.ProjectName, *body.CategoryName, middleware.GetScope(r))
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": sprintID, "success": true})
}

func (h *SprintHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

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

	// Changing the status closes or reopens the sprint; anything else edits
	// it. The two are separate rights, and a request may need both.
	scope := middleware.GetScope(r)
	var currentStatus string
	if err := (*h.DB).QueryRow("SELECT status FROM sprints WHERE id = $1 AND user_id = $2", sprintID, userID).Scan(&currentStatus); err != nil {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}
	for _, action := range sprintUpdateActions(body, allowed, currentStatus) {
		if !scope.CanSprint(action) {
			slog.Warn("Sprint action outside of user rights", "user", userID, "action", action, "sprint", sprintID)
			utils.Error(w, http.StatusForbidden, "SPRINT_ACTION_FORBIDDEN")
			return
		}
	}

	// The form sends an empty string for a field left blank: the name and
	// the start date are required, the rest is stored as NULL.
	for k, v := range body {
		text, isText := v.(string)
		blank := v == nil || (isText && strings.TrimSpace(text) == "")
		switch {
		case !blank || !allowed[k]:
		case k == "name":
			utils.Error(w, http.StatusBadRequest, "NAME_REQUIRED")
			return
		case k == "start_date":
			utils.Error(w, http.StatusBadRequest, "START_DATE_REQUIRED")
			return
		default:
			body[k] = nil
		}
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
	res, err := (*h.DB).Exec(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	utils.Success(w)
}

// sprintUpdateActions returns the rights an update of a sprint needs: "close"
// when the status changes, "edit" when any other known field is sent. The
// edit form sends the status along with everything else, so a status equal
// to the current one is not a change.
func sprintUpdateActions(body map[string]interface{}, known map[string]bool, currentStatus string) []string {
	var edit, status bool
	for field, value := range body {
		switch {
		case field == "status":
			status = value != currentStatus
		case known[field]:
			edit = true
		}
	}
	var actions []string
	if edit {
		actions = append(actions, access.SprintEdit)
	}
	if status {
		actions = append(actions, access.SprintClose)
	}
	return actions
}

func (h *SprintHandler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	res, err := (*h.DB).Exec("DELETE FROM sprints WHERE id = $1 AND user_id = $2", sprintID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if !h.ownsSprint(sprintID, middleware.GetUserID(r)) {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	var body struct {
		IssueExternalID int `json:"issue_external_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if !requireIssue(w, r, *h.DB, body.IssueExternalID) {
		return
	}

	_, err = (*h.DB).Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, sprintID, body.IssueExternalID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) UnassignTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	issueID, err := strconv.Atoi(mux.Vars(r)["issueId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if !h.ownsSprint(sprintID, middleware.GetUserID(r)) {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	res, err := (*h.DB).Exec("DELETE FROM sprint_issues WHERE sprint_id = $1 AND issue_external_id = $2", sprintID, issueID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UNASSIGN_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) GetBacklog(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	// Issues of the user's projects (within rights) that are in no sprint
	args := []interface{}{}
	where := []string{statusNotIn("i.status_id", GroupClosed), middleware.GetScope(r).ProjectIssueCond("i.", &args)}

	query := `SELECT i.external_id, i.subject, i.project_name, i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours
		FROM issues i
		WHERE ` + utils.JoinStrings(where, " AND ") + `
		AND i.external_id NOT IN (SELECT issue_external_id FROM sprint_issues)
		ORDER BY i.project_name, ` + priorityRank("i.priority_id") + ` DESC NULLS LAST`

	rows, err := (*h.DB).Query(query, args...)
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
	// A map has no order; without sorting the projects would be shuffled on
	// every refresh
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].ProjectName) < strings.ToLower(result[j].ProjectName)
	})
	if result == nil {
		result = []BacklogGroup{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"backlog": result})
}

func (h *SprintHandler) RefreshSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var s Sprint
	err = (*h.DB).QueryRow(`SELECT id, project_name, category_name, auto_fill_category
		FROM sprints WHERE id = $1 AND user_id = $2`, sprintID, userID).Scan(
		&s.ID, &s.ProjectName, &s.CategoryName, &s.AutoFillCategory)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	if s.AutoFillCategory && s.ProjectName != nil && s.CategoryName != nil {
		h.autoFill(s.ID, *s.ProjectName, *s.CategoryName, middleware.GetScope(r))
	}

	utils.Success(w)
}

// emptyToNil turns a blank value into NULL: an empty date is not a date, and
// an empty project or category means none.
func emptyToNil(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

// ownsSprint reports whether the sprint belongs to the given user.
func (h *SprintHandler) ownsSprint(sprintID, userID int) bool {
	var exists bool
	err := (*h.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM sprints WHERE id = $1 AND user_id = $2)", sprintID, userID).Scan(&exists)
	return err == nil && exists
}

// getSprintTasks returns the issues of a sprint the user may still see: an
// issue put into a sprint stays there, but rights may have been narrowed since.
func (h *SprintHandler) getSprintTasks(sprintID int, scope *access.Scope) []SprintTask {
	args := []interface{}{sprintID}
	visible := scope.AllowedIssueCond("i.", &args)
	rows, err := (*h.DB).Query(`SELECT i.external_id, i.subject, i.project_name, i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours,
		COALESCE(st.group_name, 'open')
		FROM sprint_issues si JOIN issues i ON si.issue_external_id = i.external_id
		LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'
		WHERE si.sprint_id = $1 AND `+visible+`
		ORDER BY `+priorityRank("i.priority_id")+` DESC NULLS LAST`, args...)
	if err != nil {
		return []SprintTask{}
	}
	defer rows.Close()

	var tasks []SprintTask
	for rows.Next() {
		var t SprintTask
		if rows.Scan(&t.ExternalID, &t.Subject, &t.ProjectName, &t.StatusName,
			&t.PriorityName, &t.PriorityID, &t.AssignedToName, &t.DueDate, &t.EstimatedHours,
			&t.statusGroup) == nil {
			tasks = append(tasks, t)
		}
	}
	if tasks == nil {
		return []SprintTask{}
	}
	return tasks
}

func (h *SprintHandler) autoFill(sprintID int, projectName, categoryName string, scope *access.Scope) {
	// Find issues matching project + category not already in sprint,
	// among those the user may see
	args := []interface{}{projectName, categoryName, sprintID}
	visible := scope.AllowedIssueCond("", &args)
	rows, err := (*h.DB).Query(`SELECT external_id FROM issues
		WHERE project_name = $1 AND category_name = $2
		AND `+statusNotIn("status_id", GroupClosed)+`
		AND external_id NOT IN (SELECT issue_external_id FROM sprint_issues WHERE sprint_id = $3)
		AND `+visible, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var issueID int
		if rows.Scan(&issueID) == nil {
			(*h.DB).Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				sprintID, issueID)
			count++
		}
	}
}

func (s *Sprint) calculateProgress() {
	for _, t := range s.Tasks {
		switch t.statusGroup {
		case GroupClosed:
			s.ClosedCount++
		case GroupTesting:
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
