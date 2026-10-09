package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/datasource"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type KanbanHandler struct {
	DB     **sql.DB
	Source *manager.Manager
}

type KanbanCard struct {
	ExternalID     int     `json:"external_id"`
	Subject        string  `json:"subject"`
	ProjectName    string  `json:"project_name"`
	StatusName     string  `json:"status_name"`
	StatusID       int     `json:"status_id"`
	PriorityName   string  `json:"priority_name"`
	PriorityID     int     `json:"priority_id"`
	AssignedToName string  `json:"assigned_to_name"`
	AssignedToID   *int    `json:"assigned_to_id"`
	CategoryName   string  `json:"category_name"`
	DueDate        *string `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	IsOverdue      bool    `json:"is_overdue"`
}

type KanbanColumn struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Icon  string       `json:"icon,omitempty"`
	Tasks []KanbanCard `json:"tasks"`
}

type KanbanBoard struct {
	Mode    string         `json:"mode"` // "users" or "statuses"
	Columns []KanbanColumn `json:"columns"`
	Total   int            `json:"total"`
}

func (h *KanbanHandler) GetBoard(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	mode := r.URL.Query().Get("mode") // "users" or "statuses"
	projectID := r.URL.Query().Get("project_id")

	if mode == "" {
		mode = "users"
	}

	// Get user settings for column order
	var columnOrderStatuses, columnOrderUsers []byte
	(*h.DB).QueryRow(`SELECT kanban_column_order_statuses, kanban_column_order_users
		FROM user_settings WHERE user_id = $1`, userID).Scan(&columnOrderStatuses, &columnOrderUsers)

	// Get user's selected projects
	var selectedProjects []int64
	var spJSON []byte
	(*h.DB).QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID).Scan(&spJSON)
	if len(spJSON) > 0 {
		json.Unmarshal(spJSON, &selectedProjects)
	}

	// Build project filter
	where := []string{statusNotIn("status_id", GroupClosed)}
	args := []interface{}{}
	argIdx := 1

	if projectID != "" {
		where = append(where, "project_id = $"+strconv.Itoa(argIdx))
		args = append(args, projectID)
		argIdx++
	} else if len(selectedProjects) > 0 {
		placeholders := make([]string, len(selectedProjects))
		for i, pid := range selectedProjects {
			placeholders[i] = "$" + strconv.Itoa(argIdx)
			args = append(args, pid)
			argIdx++
		}
		where = append(where, "project_id IN ("+strings.Join(placeholders, ",")+")")
	}

	query := `SELECT external_id, subject, project_name, status_name, status_id,
		priority_name, priority_id, assigned_to_name, assigned_to_id,
		category_name, due_date, estimated_hours
		FROM issues WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + priorityRank("priority_id") + ` DESC NULLS LAST, external_id`

	rows, err := (*h.DB).Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	todayStr := time.Now().Format("2006-01-02")
	var cards []KanbanCard
	for rows.Next() {
		var c KanbanCard
		if rows.Scan(&c.ExternalID, &c.Subject, &c.ProjectName, &c.StatusName, &c.StatusID,
			&c.PriorityName, &c.PriorityID, &c.AssignedToName, &c.AssignedToID,
			&c.CategoryName, &c.DueDate, &c.EstimatedHours) == nil {
			// Check overdue
			if c.DueDate != nil && *c.DueDate < todayStr {
				c.IsOverdue = true
			}
			cards = append(cards, c)
		}
	}
	if cards == nil {
		cards = []KanbanCard{}
	}

	// Build columns based on mode
	board := KanbanBoard{Mode: mode, Total: len(cards)}

	switch mode {
	case "statuses":
		board.Columns = h.buildStatusColumns(cards, columnOrderStatuses)
	default: // "users"
		board.Columns = h.buildUserColumns(cards, columnOrderUsers)
	}

	utils.JSON(w, http.StatusOK, board)
}

func (h *KanbanHandler) buildStatusColumns(cards []KanbanCard, orderJSON []byte) []KanbanColumn {
	colMap := make(map[string]*KanbanColumn)
	for _, c := range cards {
		key := strconv.Itoa(c.StatusID)
		if _, ok := colMap[key]; !ok {
			colMap[key] = &KanbanColumn{
				ID:    key,
				Name:  c.StatusName,
				Tasks: []KanbanCard{},
			}
		}
		colMap[key].Tasks = append(colMap[key].Tasks, c)
	}

	// Apply order if saved
	var order []string
	if len(orderJSON) > 0 {
		json.Unmarshal(orderJSON, &order)
	}

	return applyColumnOrder(colMap, order)
}

func (h *KanbanHandler) buildUserColumns(cards []KanbanCard, orderJSON []byte) []KanbanColumn {
	colMap := make(map[string]*KanbanColumn)
	for _, c := range cards {
		// Columns are identified by the member id, not by a display name;
		// the "no assignee" column gets its title on the client.
		key, name := unassignedColumn, ""
		if c.AssignedToID != nil {
			key, name = strconv.Itoa(*c.AssignedToID), c.AssignedToName
		}
		if _, ok := colMap[key]; !ok {
			colMap[key] = &KanbanColumn{
				ID:    key,
				Name:  name,
				Tasks: []KanbanCard{},
			}
		}
		colMap[key].Tasks = append(colMap[key].Tasks, c)
	}

	// Apply order if saved
	var order []string
	if len(orderJSON) > 0 {
		json.Unmarshal(orderJSON, &order)
	}

	return applyColumnOrder(colMap, order)
}

func applyColumnOrder(colMap map[string]*KanbanColumn, order []string) []KanbanColumn {
	var columns []KanbanColumn

	// Add columns in order
	for _, name := range order {
		if col, ok := colMap[name]; ok {
			columns = append(columns, *col)
			delete(colMap, name)
		}
	}

	// Add remaining columns
	for _, col := range colMap {
		columns = append(columns, *col)
	}

	return columns
}

func (h *KanbanHandler) MoveCard(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	var body struct {
		IssueID    int    `json:"issue_id"`
		TargetID   string `json:"target_id"` // status_id or user name
		Mode       string `json:"mode"`       // "users" or "statuses"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// The issue must be on a project the user works with.
	var projectID int
	if err := (*h.DB).QueryRow("SELECT project_id FROM issues WHERE external_id = $1 AND data_source = 'redmine'",
		body.IssueID).Scan(&projectID); err != nil {
		utils.Error(w, http.StatusNotFound, "ISSUE_NOT_FOUND")
		return
	}
	if allowed := h.userProjects(middleware.GetUserID(r)); len(allowed) > 0 && !containsInt(allowed, projectID) {
		slog.Warn("Kanban move outside of user projects", "user", middleware.GetUserID(r), "issue", body.IssueID)
		utils.Error(w, http.StatusForbidden, "FORBIDDEN")
		return
	}

	// localSet: new values for the local cache; redmineFields: payload for
	// the data source (an empty string clears a field in Redmine).
	var localSet string
	var localArgs []interface{}
	redmineFields := map[string]interface{}{}

	switch body.Mode {
	case "statuses":
		var statusID int
		var statusName string
		if err := (*h.DB).QueryRow("SELECT external_id, name FROM statuses WHERE external_id::text = $1 AND data_source = 'redmine'",
			body.TargetID).Scan(&statusID, &statusName); err != nil {
			utils.Error(w, http.StatusBadRequest, "STATUS_NOT_FOUND")
			return
		}
		localSet = "status_name = $1, status_id = $2"
		localArgs = []interface{}{statusName, statusID}
		redmineFields["status_id"] = statusID

	case "users":
		if body.TargetID == unassignedColumn {
			localSet = "assigned_to_name = $1, assigned_to_id = $2"
			localArgs = []interface{}{"", nil}
			redmineFields["assigned_to_id"] = ""
			break
		}
		var memberID int
		var memberName string
		if err := (*h.DB).QueryRow("SELECT external_id, name FROM members WHERE external_id::text = $1 AND data_source = 'redmine'",
			body.TargetID).Scan(&memberID, &memberName); err != nil {
			utils.Error(w, http.StatusBadRequest, "MEMBER_NOT_FOUND")
			return
		}
		localSet = "assigned_to_name = $1, assigned_to_id = $2"
		localArgs = []interface{}{memberName, memberID}
		redmineFields["assigned_to_id"] = memberID

	default:
		utils.Error(w, http.StatusBadRequest, "INVALID_MODE")
		return
	}

	// The data source is the system of record: write there first and touch
	// the local cache only on success, otherwise the next sync would silently
	// roll the move back.
	if h.Source == nil || h.Source.Client() == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATA_SOURCE_NOT_CONFIGURED")
		return
	}
	client := h.Source.Client()
	if err := client.UpdateIssue(body.IssueID, redmineFields); err != nil {
		slog.Warn("Failed to move issue in Redmine", "issue", body.IssueID, "error", err)
		utils.JSON(w, http.StatusBadGateway, map[string]string{
			"error":   "REDMINE_UPDATE_FAILED",
			"message": err.Error(),
		})
		return
	}
	// Redmine accepts an update but silently skips a change its workflow
	// forbids: read the issue back before trusting the move.
	if state, err := client.GetIssueState(body.IssueID); err != nil {
		slog.Warn("Could not verify the move in Redmine", "issue", body.IssueID, "error", err)
	} else if !moveApplied(state, redmineFields) {
		utils.Error(w, http.StatusBadGateway, "REDMINE_CHANGE_REJECTED")
		return
	}

	localArgs = append(localArgs, body.IssueID)
	if _, err := (*h.DB).Exec("UPDATE issues SET "+localSet+", synced_at = NOW() WHERE external_id = $3 AND data_source = 'redmine'",
		localArgs...); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	utils.Success(w)
}

// unassignedColumn is the id of the "no assignee" column in the users mode.
const unassignedColumn = "unassigned"

// userProjects returns the projects selected by the user together with all
// their descendants; empty means no restriction.
func (h *KanbanHandler) userProjects(userID int) []int {
	var spJSON []byte
	(*h.DB).QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID).Scan(&spJSON)
	var selected []int
	if len(spJSON) > 0 {
		json.Unmarshal(spJSON, &selected)
	}
	if len(selected) == 0 {
		return nil
	}
	if expanded, err := datasource.ExpandProjects(*h.DB, selected); err == nil {
		return expanded
	}
	return selected
}

// moveApplied reports whether the issue in Redmine has the values a move
// asked for (fields as sent to UpdateIssue).
func moveApplied(state *redmine.IssueState, fields map[string]interface{}) bool {
	if want, ok := fields["status_id"]; ok && state.StatusID != want {
		return false
	}
	if want, ok := fields["assigned_to_id"]; ok {
		if want == "" {
			return state.AssignedToID == nil
		}
		return state.AssignedToID != nil && *state.AssignedToID == want
	}
	return true
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (h *KanbanHandler) SaveColumnOrder(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var body struct {
		Mode   string   `json:"mode"`
		Order  []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	orderJSON, _ := json.Marshal(body.Order)

	if _, err := (*h.DB).Exec(`INSERT INTO user_settings (user_id, updated_at) VALUES ($1, NOW())
		ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	switch body.Mode {
	case "statuses":
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_statuses = $1 WHERE user_id = $2", orderJSON, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	case "users":
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_users = $1 WHERE user_id = $2", orderJSON, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	utils.Success(w)
}
