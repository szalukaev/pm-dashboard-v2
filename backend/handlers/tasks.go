package handlers

import (
	"log/slog"
	"database/sql"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type TaskHandler struct {
	DB     **sql.DB
	Source *manager.Manager
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
}

type TaskGroup struct {
	Name          string         `json:"name"`
	TaskCount     int            `json:"task_count"`
	EstimateTotal float64        `json:"estimate_total"`
	FactTotal     float64        `json:"fact_total"`
	Tasks         []TaskResponse `json:"tasks"`
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
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

	// Get user's selected projects and team
	var selectedProjects, selectedTeam []int64
	row := (*h.DB).QueryRow("SELECT selected_projects, selected_team FROM user_settings WHERE user_id = $1", userID)
	var spJSON, stJSON []byte
	if err := row.Scan(&spJSON, &stJSON); err == nil {
		if len(spJSON) > 0 {
			json.Unmarshal(spJSON, &selectedProjects)
		}
		if len(stJSON) > 0 {
			json.Unmarshal(stJSON, &selectedTeam)
		}
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

	// Filter by user's selected team (only show tasks assigned to team members or unassigned)
	if len(selectedTeam) > 0 {
		placeholders := make([]string, len(selectedTeam))
		for i, tid := range selectedTeam {
			placeholders[i] = "$" + strconv.Itoa(argIdx)
			args = append(args, tid)
			argIdx++
		}
		where = append(where, "(assigned_to_id IN ("+strings.Join(placeholders, ",")+") OR assigned_to_id IS NULL)")
	}

	// Filter by task type using status groups from statuses table
	switch taskType {
	case "open", "testing", "closed":
		where = append(where, "status_id IN (SELECT external_id FROM statuses WHERE group_name = $"+strconv.Itoa(argIdx)+" AND data_source = 'redmine')")
		args = append(args, taskType)
		argIdx++
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
	orderBy := "external_id DESC"
	if sortBy != "" {
		validSorts := map[string]string{
			"subject":        "subject",
			"external_id":    "external_id",
			"priority_name":  priorityRank("priority_id"),
			"assigned_to":    "assigned_to_name",
			"estimate":       "estimated_hours",
			"fact":           "spent_hours",
			"status_name":    "status_name",
			"bug_fix_hours":  "bug_fix_hours",
			"bug_fix_pct":    "COALESCE(bug_fix_hours, 0) / NULLIF(spent_hours, 0)",
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

	rows, err := (*h.DB).Query(query, args...)
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
				key = "Без исполнителя"
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
	if *h.DB == nil {
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
	err = (*h.DB).QueryRow(`SELECT external_id, project_id, project_name, subject, description,
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

	utils.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
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

	// The category is resolved within the project of the issue.
	var projectID int
	if err := (*h.DB).QueryRow("SELECT COALESCE(project_id, 0) FROM issues WHERE external_id = $1", externalID).Scan(&projectID); err != nil {
		utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
		return
	}

	client := h.getRedmineClient()

	// localSets: column -> new value for the local cache;
	// redmineFields: payload for the data source. An empty string clears a field in Redmine.
	localSets := map[string]interface{}{}
	redmineFields := map[string]interface{}{}
	editable := []string{}

	for field, value := range body {
		strValue, _ := value.(string)
		switch field {
		case "status_name":
			var statusID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM statuses WHERE name = $1 AND data_source = 'redmine'", strValue).Scan(&statusID); err != nil {
				utils.Error(w, http.StatusBadRequest, "STATUS_NOT_FOUND")
				return
			}
			localSets["status_name"] = strValue
			localSets["status_id"] = statusID
			redmineFields["status_id"] = statusID
		case "priority_name":
			var priorityID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM priorities WHERE name = $1 AND data_source = 'redmine'", strValue).Scan(&priorityID); err != nil {
				utils.Error(w, http.StatusBadRequest, "PRIORITY_NOT_FOUND")
				return
			}
			localSets["priority_name"] = strValue
			localSets["priority_id"] = priorityID
			redmineFields["priority_id"] = priorityID
		case "assigned_to_name":
			if strValue == "" {
				localSets["assigned_to_name"] = ""
				localSets["assigned_to_id"] = nil
				redmineFields["assigned_to_id"] = ""
				break
			}
			var assigneeID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM members WHERE name = $1 AND data_source = 'redmine' LIMIT 1", strValue).Scan(&assigneeID); err != nil {
				utils.Error(w, http.StatusBadRequest, "MEMBER_NOT_FOUND")
				return
			}
			localSets["assigned_to_name"] = strValue
			localSets["assigned_to_id"] = assigneeID
			redmineFields["assigned_to_id"] = assigneeID
		case "start_date", "due_date":
			if strValue == "" {
				localSets[field] = nil
			} else {
				localSets[field] = strValue
			}
			redmineFields[field] = strValue
		case "estimated_hours":
			hours, ok := parseHours(value)
			if !ok {
				utils.Error(w, http.StatusBadRequest, "INVALID_VALUE")
				return
			}
			if hours == nil {
				localSets[field] = nil
			} else {
				localSets[field] = *hours
			}
			if client == nil {
				break
			}
			// Goes to the developer estimate if the issue already has one,
			// otherwise to the standard estimated_hours.
			fields, err := client.EstimateFields(externalID, hours)
			if err != nil {
				slog.Warn("Failed to read issue estimate fields", "issue", externalID, "error", err)
				utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
				return
			}
			for k, v := range fields {
				redmineFields[k] = v
			}
		case "category_name":
			localSets["category_name"] = strValue
			if strValue == "" {
				redmineFields["category_id"] = ""
				break
			}
			if client == nil {
				break
			}
			categoryID, err := client.FindCategoryID(projectID, strValue)
			if err != nil {
				utils.Error(w, http.StatusBadRequest, "CATEGORY_NOT_FOUND")
				return
			}
			redmineFields["category_id"] = categoryID
		default:
			continue
		}
		editable = append(editable, field)
	}

	if len(localSets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS_TO_UPDATE")
		return
	}

	// Old values for the change notification. Field names come from the switch above.
	oldValues := make(map[string]interface{})
	for _, field := range editable {
		var oldVal sql.NullString
		(*h.DB).QueryRow("SELECT "+field+"::text FROM issues WHERE external_id = $1", externalID).Scan(&oldVal)
		if oldVal.Valid {
			oldValues[field] = oldVal.String
		} else {
			oldValues[field] = nil
		}
	}

	// The data source is the system of record: write there first and touch
	// the local cache only on success, otherwise the next sync would silently
	// roll the change back.
	if client != nil {
		if err := client.UpdateIssue(externalID, redmineFields); err != nil {
			slog.Warn("Failed to update issue in Redmine", "issue", externalID, "error", err)
			utils.JSON(w, http.StatusOK, map[string]interface{}{
				"success":       false,
				"old_values":    oldValues,
				"new_values":    body,
				"redmine_ok":    false,
				"redmine_error": err.Error(),
			})
			return
		}
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1
	for col, value := range localSets {
		setClauses = append(setClauses, col+" = $"+strconv.Itoa(argIdx))
		args = append(args, value)
		argIdx++
	}
	args = append(args, externalID)
	query := "UPDATE issues SET " + strings.Join(setClauses, ", ") + ", synced_at = NOW() WHERE external_id = $" + strconv.Itoa(argIdx)
	if _, err := (*h.DB).Exec(query, args...); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"old_values":    oldValues,
		"new_values":    body,
		"redmine_ok":    true,
		"redmine_error": "",
	})
}

// parseHours accepts a JSON number, a numeric string ("1.5" / "1,5") or an
// empty value. A nil result with ok=true means "clear the estimate".
func parseHours(value interface{}) (*float64, bool) {
	switch v := value.(type) {
	case nil:
		return nil, true
	case float64:
		if v < 0 {
			return nil, false
		}
		return &v, true
	case string:
		s := strings.TrimSpace(strings.Replace(v, ",", ".", 1))
		if s == "" {
			return nil, true
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 0 {
			return nil, false
		}
		return &f, true
	default:
		return nil, false
	}
}

// getRedmineClient returns the current data source client, nil when the
// source is not configured.
func (h *TaskHandler) getRedmineClient() *redmine.Client {
	if h.Source == nil {
		return nil
	}
	return h.Source.Client()
}

func (h *TaskHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	client := h.getRedmineClient()
	if client == nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"comments": []interface{}{}})
		return
	}

	comments, err := client.GetIssueComments(externalID)
	if err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	if comments == nil {
		comments = []redmine.Comment{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"comments": comments})
}

// GetTaskDetails returns the live part of the task card from Redmine:
// raw description markup, attached files and comment history.
func (h *TaskHandler) GetTaskDetails(w http.ResponseWriter, r *http.Request) {
	externalID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}
	details, err := client.GetIssueDetails(externalID)
	if err != nil {
		slog.Warn("Failed to get issue details", "issue", externalID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	utils.JSON(w, http.StatusOK, details)
}

// inlineImageTypes may be shown in the page; anything else is served as a
// download so that an uploaded HTML/SVG file cannot run in our origin.
var inlineImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
}

// GetTaskAttachment proxies a file of the task from Redmine (the browser has
// no Redmine credentials). Only attachments of that task are served.
func (h *TaskHandler) GetTaskAttachment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	externalID, err1 := strconv.Atoi(vars["id"])
	attachmentID, err2 := strconv.Atoi(vars["attachment_id"])
	if err1 != nil || err2 != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}
	details, err := client.GetIssueDetails(externalID)
	if err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	var att *redmine.Attachment
	for i := range details.Attachments {
		if details.Attachments[i].ID == attachmentID {
			att = &details.Attachments[i]
			break
		}
	}
	if att == nil {
		utils.Error(w, http.StatusNotFound, "ATTACHMENT_NOT_FOUND")
		return
	}

	body, err := client.DownloadAttachment(attachmentID)
	if err != nil {
		slog.Warn("Failed to download attachment", "issue", externalID, "attachment", attachmentID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	defer body.Close()

	contentType := strings.ToLower(att.ContentType)
	disposition := "attachment"
	if inlineImageTypes[contentType] {
		disposition = "inline"
	} else {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if att.Filesize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(att.Filesize, 10))
	}
	io.Copy(w, body)
}

func (h *TaskHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}

	if err := client.AddIssueComment(externalID, body.Text); err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *TaskHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectID := r.URL.Query().Get("project_id")
	var rows *sql.Rows
	var err error
	if projectID != "" {
		rows, err = (*h.DB).Query("SELECT DISTINCT category_name FROM issues WHERE category_name IS NOT NULL AND category_name != '' AND project_id = $1 ORDER BY category_name", projectID)
	} else {
		rows, err = (*h.DB).Query("SELECT DISTINCT category_name FROM issues WHERE category_name IS NOT NULL AND category_name != '' ORDER BY category_name")
	}
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

// GetProjectCategories returns every category defined in the project (not
// only the ones already used by issues) — the choices for editing a task.
func (h *TaskHandler) GetProjectCategories(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(r.URL.Query().Get("project_id"))
	if err != nil || projectID <= 0 {
		utils.Error(w, http.StatusBadRequest, "INVALID_PROJECT_ID")
		return
	}

	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}

	list, err := client.GetCategories(projectID)
	if err != nil {
		slog.Warn("Failed to get project categories", "project_id", projectID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}

	categories := make([]string, 0, len(list))
	for _, c := range list {
		categories = append(categories, c.Name)
	}
	sort.Strings(categories)

	utils.JSON(w, http.StatusOK, map[string]interface{}{"categories": categories})
}

func (h *TaskHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, parent_id FROM projects ORDER BY name")
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

func (h *TaskHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := (*h.DB).Query("SELECT external_id, name FROM members WHERE data_source = 'redmine' AND name != '' ORDER BY name")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DB_ERROR")
		return
	}
	defer rows.Close()

	members := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		members = append(members, map[string]interface{}{
			"id":   id,
			"name": name,
		})
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"members": members})
}

func (h *TaskHandler) GetPriorities(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, sort_order FROM priorities WHERE data_source = 'redmine' ORDER BY external_id DESC")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type PriorityInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		SortOrder int   `json:"sort_order"`
	}
	var priorities []PriorityInfo
	for rows.Next() {
		var p PriorityInfo
		if rows.Scan(&p.ID, &p.Name, &p.SortOrder) == nil {
			priorities = append(priorities, p)
		}
	}
	if priorities == nil {
		priorities = []PriorityInfo{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"priorities": priorities})
}

func (h *TaskHandler) GetStatuses(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, is_closed, group_name, is_bug FROM statuses ORDER BY external_id")
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
		IsBug    bool   `json:"is_bug"`
	}
	var statuses []StatusInfo
	for rows.Next() {
		var s StatusInfo
		if rows.Scan(&s.ID, &s.Name, &s.IsClosed, &s.Group, &s.IsBug) == nil {
			statuses = append(statuses, s)
		}
	}
	if statuses == nil {
		statuses = []StatusInfo{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"statuses": statuses})
}
