package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type TaskHandler struct {
	DB *sql.DB
}

type TaskResponse struct {
	ID             int      `json:"id"`
	ExternalID     int      `json:"external_id"`
	ProjectID      int      `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	Subject        string   `json:"subject"`
	Description    string   `json:"description,omitempty"`
	StatusName     string   `json:"status_name"`
	StatusID       int      `json:"status_id"`
	PriorityName   string   `json:"priority_name"`
	PriorityID     int      `json:"priority_id"`
	AssignedToName string   `json:"assigned_to_name"`
	AssignedToID   *int     `json:"assigned_to_id"`
	CategoryName   string   `json:"category_name"`
	StartDate      *string  `json:"start_date"`
	DueDate        *string  `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	SpentHours     float64  `json:"spent_hours"`
	DoneRatio      int      `json:"done_ratio"`
	TrackerName    string   `json:"tracker_name"`
	AuthorName     string   `json:"author_name"`
	BugFixHours    float64  `json:"bug_fix_hours"`
	BugFixPct      float64  `json:"bug_fix_pct"`
}

type TaskGroup struct {
	Name          string         `json:"name"`
	TaskCount     int            `json:"task_count"`
	EstimateTotal float64        `json:"estimate_total"`
	FactTotal     float64        `json:"fact_total"`
	Tasks         []TaskResponse `json:"tasks"`
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	// Parse query params
	taskType := r.URL.Query().Get("type") // "open", "testing", "closed", "all"
	projectID := r.URL.Query().Get("project_id")
	search := r.URL.Query().Get("search")
	groupBy := r.URL.Query().Get("group_by") // "project", "assignee"
	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")
	category := r.URL.Query().Get("category")

	// Get user's selected projects
	var selectedProjects []int64
	row := h.DB.QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID)
	var spJSON []byte
	if err := row.Scan(&spJSON); err == nil && len(spJSON) > 0 {
		json.Unmarshal(spJSON, &selectedProjects)
	}

	// Build query
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	// Filter by user's selected projects
	if len(selectedProjects) > 0 && projectID == "" {
		placeholders := make([]string, len(selectedProjects))
		for i, pid := range selectedProjects {
			placeholders[i] = "$" + strconv.Itoa(argIdx)
			args = append(args, pid)
			argIdx++
		}
		where = append(where, "project_id IN ("+strings.Join(placeholders, ",")+")")
	}

	// Filter by task type
	switch taskType {
	case "open":
		where = append(where, "LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')")
		where = append(where, "LOWER(status_name) NOT LIKE '%test%'")
	case "testing":
		where = append(where, "LOWER(status_name) LIKE '%test%'")
	case "closed":
		where = append(where, "(LOWER(status_name) IN ('closed', 'rejected', 'resolved', 'tested'))")
	case "all", "":
		// no filter
	}

	// Filter by project
	if projectID != "" {
		where = append(where, "project_id = $"+strconv.Itoa(argIdx))
		args = append(args, projectID)
		argIdx++
	}

	// Filter by category
	if category != "" {
		where = append(where, "category_name = $"+strconv.Itoa(argIdx))
		args = append(args, category)
		argIdx++
	}

	// Search
	if search != "" {
		where = append(where, "(LOWER(subject) LIKE $"+strconv.Itoa(argIdx)+" OR CAST(external_id AS TEXT) LIKE $"+strconv.Itoa(argIdx)+")")
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}

	// Sort
	orderBy := "project_name, external_id"
	if sortBy != "" {
		validSorts := map[string]string{
			"subject":        "subject",
			"external_id":    "external_id",
			"priority_name":  "priority_id",
			"assigned_to":    "assigned_to_name",
			"estimate":       "estimated_hours",
			"fact":           "spent_hours",
			"status_name":    "status_name",
			"bug_fix_hours":  "bug_fix_hours",
			"bug_fix_pct":    "bug_fix_pct",
			"start_date":     "start_date",
			"due_date":       "due_date",
			"category_name":  "category_name",
		}
		if col, ok := validSorts[sortBy]; ok {
			dir := "ASC"
			if sortDir == "desc" {
				dir = "DESC"
			}
			orderBy = col + " " + dir
		}
	}

	query := `SELECT external_id, project_id, project_name, subject, description,
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + orderBy

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var tasks []TaskResponse
	for rows.Next() {
		var t TaskResponse
		err := rows.Scan(
			&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
			&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
			&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
			&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
			&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
		)
		if err != nil {
			continue
		}
		// Calculate bug fix %
		if t.SpentHours > 0 {
			t.BugFixPct = (t.BugFixHours / t.SpentHours) * 100
		}
		tasks = append(tasks, t)
	}

	if tasks == nil {
		tasks = []TaskResponse{}
	}

	// Group if requested
	if groupBy != "" {
		groups := groupTasks(tasks, groupBy)
		utils.JSON(w, http.StatusOK, map[string]interface{}{
			"groups": groups,
			"total":  len(tasks),
		})
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func groupTasks(tasks []TaskResponse, groupBy string) []TaskGroup {
	groupMap := make(map[string]*TaskGroup)

	for _, t := range tasks {
		var key string
		switch groupBy {
		case "assignee":
			if t.AssignedToName != "" {
				key = t.AssignedToName
			} else {
				key = "Неназначенные"
			}
		default: // "project"
			key = t.ProjectName
			if key == "" {
				key = "Без проекта"
			}
		}

		if _, ok := groupMap[key]; !ok {
			groupMap[key] = &TaskGroup{Name: key}
		}
		g := groupMap[key]
		g.Tasks = append(g.Tasks, t)
		g.TaskCount++
		if t.EstimatedHours != nil {
			g.EstimateTotal += *t.EstimatedHours
		}
		g.FactTotal += t.SpentHours
	}

	groups := make([]TaskGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, *g)
	}
	return groups
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var t TaskResponse
	err = h.DB.QueryRow(`SELECT external_id, project_id, project_name, subject, description,
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE external_id = $1`, externalID).Scan(
		&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
		&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
		&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
		&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
		&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
	)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
		return
	}

	if t.SpentHours > 0 {
		t.BugFixPct = (t.BugFixHours / t.SpentHours) * 100
	}

	utils.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Build update fields
	allowed := map[string]string{
		"status_name":     "status_name",
		"priority_name":   "priority_name",
		"assigned_to_name": "assigned_to_name",
		"start_date":      "start_date",
		"due_date":        "due_date",
		"estimated_hours": "estimated_hours",
		"category_name":   "category_name",
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	for field, value := range body {
		if col, ok := allowed[field]; ok {
			setClauses = append(setClauses, col+" = $"+strconv.Itoa(argIdx))
			args = append(args, value)
			argIdx++
		}
	}

	if len(setClauses) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS_TO_UPDATE")
		return
	}

	args = append(args, externalID)
	query := "UPDATE issues SET " + strings.Join(setClauses, ", ") + ", synced_at = NOW() WHERE external_id = $" + strconv.Itoa(argIdx)

	_, err = h.DB.Exec(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	utils.Success(w)
}

func (h *TaskHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := h.DB.Query("SELECT DISTINCT category_name FROM issues WHERE category_name IS NOT NULL AND category_name != '' ORDER BY category_name")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			categories = append(categories, c)
		}
	}

	if categories == nil {
		categories = []string{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"categories": categories})
}

func (h *TaskHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := h.DB.Query("SELECT external_id, name, parent_id FROM projects ORDER BY name")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type ProjectInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		ParentID *int   `json:"parent_id"`
	}
	var projects []ProjectInfo
	for rows.Next() {
		var p ProjectInfo
		if rows.Scan(&p.ID, &p.Name, &p.ParentID) == nil {
			projects = append(projects, p)
		}
	}
	if projects == nil {
		projects = []ProjectInfo{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"projects": projects})
}

func (h *TaskHandler) GetStatuses(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := h.DB.Query("SELECT external_id, name, is_closed, group_name FROM statuses ORDER BY name")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type StatusInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		IsClosed bool   `json:"is_closed"`
		Group    string `json:"group"`
	}
	var statuses []StatusInfo
	for rows.Next() {
		var s StatusInfo
		if rows.Scan(&s.ID, &s.Name, &s.IsClosed, &s.Group) == nil {
			statuses = append(statuses, s)
		}
	}
	if statuses == nil {
		statuses = []StatusInfo{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"statuses": statuses})
}
