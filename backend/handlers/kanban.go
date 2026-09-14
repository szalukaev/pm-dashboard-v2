package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type KanbanHandler struct {
	DB *sql.DB
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
	if h.DB == nil {
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
	h.DB.QueryRow(`SELECT kanban_column_order_statuses, kanban_column_order_users
		FROM user_settings WHERE user_id = $1`, userID).Scan(&columnOrderStatuses, &columnOrderUsers)

	// Get user's selected projects
	var selectedProjects []int64
	var spJSON []byte
	h.DB.QueryRow("SELECT selected_projects FROM user_settings WHERE user_id = $1", userID).Scan(&spJSON)
	if len(spJSON) > 0 {
		json.Unmarshal(spJSON, &selectedProjects)
	}

	// Build project filter
	where := []string{"LOWER(status_name) NOT IN ('closed', 'rejected', 'resolved', 'tested')"}
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
		ORDER BY priority_id DESC, external_id`

	rows, err := h.DB.Query(query, args...)
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
		key := c.StatusName
		if _, ok := colMap[key]; !ok {
			colMap[key] = &KanbanColumn{
				ID:    strconv.Itoa(c.StatusID),
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
		key := c.AssignedToName
		if key == "" {
			key = "Неназначенные"
		}
		if _, ok := colMap[key]; !ok {
			colMap[key] = &KanbanColumn{
				ID:    key,
				Name:  key,
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

	// Remove empty "Неназначенные" if no unassigned tasks
	if col, ok := colMap["Неназначенные"]; ok && len(col.Tasks) == 0 {
		delete(colMap, "Неназначенные")
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
	if h.DB == nil {
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

	switch body.Mode {
	case "statuses":
		// Update status
		var statusName string
		err := h.DB.QueryRow("SELECT name FROM statuses WHERE external_id = $1", body.TargetID).Scan(&statusName)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, "STATUS_NOT_FOUND")
			return
		}
		_, err = h.DB.Exec("UPDATE issues SET status_name = $1, status_id = $2, synced_at = NOW() WHERE external_id = $3",
			statusName, body.TargetID, body.IssueID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}

	case "users":
		// Reassign
		if body.TargetID == "Неназначенные" {
			h.DB.Exec("UPDATE issues SET assigned_to_name = '', assigned_to_id = NULL, synced_at = NOW() WHERE external_id = $1", body.IssueID)
		} else {
			// Find member by name
			var memberID *int
			h.DB.QueryRow("SELECT external_id FROM members WHERE name = $1 LIMIT 1", body.TargetID).Scan(&memberID)
			h.DB.Exec("UPDATE issues SET assigned_to_name = $1, assigned_to_id = $2, synced_at = NOW() WHERE external_id = $3",
				body.TargetID, memberID, body.IssueID)
		}
	}

	utils.Success(w)
}

func (h *KanbanHandler) SaveColumnOrder(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
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

	h.DB.Exec(`INSERT INTO user_settings (user_id, updated_at) VALUES ($1, NOW())
		ON CONFLICT (user_id) DO NOTHING`, userID)

	switch body.Mode {
	case "statuses":
		h.DB.Exec("UPDATE user_settings SET kanban_column_order_statuses = $1 WHERE user_id = $2", orderJSON, userID)
	case "users":
		h.DB.Exec("UPDATE user_settings SET kanban_column_order_users = $1 WHERE user_id = $2", orderJSON, userID)
	}

	utils.Success(w)
}
