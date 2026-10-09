package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"pm-dashboard/datasource"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/lib/pq"
)

type KanbanHandler struct {
	DB     **sql.DB
	Source *manager.Manager
	// Events reports changes to open pages (WebSocket); may be nil.
	Events func(event string, data interface{})
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
	mode := r.URL.Query().Get("mode")
	if mode != "statuses" {
		mode = "users"
	}

	var columns []KanbanColumn
	var err error
	if mode == "statuses" {
		columns, err = h.statusBoard(userID, r.URL.Query().Get("project_id"))
	} else {
		columns, err = h.userBoard(userID)
	}
	if err != nil {
		slog.Error("Failed to build the kanban board", "mode", mode, "error", err)
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	board := KanbanBoard{Mode: mode, Columns: columns}
	for _, col := range columns {
		board.Total += len(col.Tasks)
	}
	utils.JSON(w, http.StatusOK, board)
}

// statusBoard: a column for every status of the data source, with the issues
// of the chosen project and all its subprojects. No project — no columns, the
// client asks to choose one.
func (h *KanbanHandler) statusBoard(userID int, projectParam string) ([]KanbanColumn, error) {
	projectID, err := strconv.Atoi(projectParam)
	if err != nil {
		return []KanbanColumn{}, nil
	}
	projects, err := datasource.ExpandProjects(*h.DB, []int{projectID})
	if err != nil {
		return nil, err
	}
	cards, err := h.loadCards("project_id = ANY($1)", pq.Array(projects))
	if err != nil {
		return nil, err
	}

	rows, err := (*h.DB).Query("SELECT external_id, name FROM statuses WHERE data_source = 'redmine' ORDER BY external_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []KanbanColumn{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		columns = append(columns, KanbanColumn{ID: strconv.Itoa(id), Name: name})
	}

	columns = fillColumns(columns, cards, func(c KanbanCard) string { return strconv.Itoa(c.StatusID) })
	return orderColumns(columns, h.columnOrder(userID, "kanban_column_order_statuses")), nil
}

// userBoard: a column for every member of the user's team plus "no assignee"
// (hidden when empty), with the issues in open statuses only.
func (h *KanbanHandler) userBoard(userID int) ([]KanbanColumn, error) {
	var teamJSON []byte
	(*h.DB).QueryRow("SELECT selected_team FROM user_settings WHERE user_id = $1", userID).Scan(&teamJSON)
	var team []int
	if len(teamJSON) > 0 {
		json.Unmarshal(teamJSON, &team)
	}

	where := statusIn("status_id", GroupOpen)
	args := []interface{}{}
	if projects := h.userProjects(userID); len(projects) > 0 {
		args = append(args, pq.Array(projects))
		where += " AND project_id = ANY($" + strconv.Itoa(len(args)) + ")"
	}
	if len(team) > 0 {
		args = append(args, pq.Array(team))
		where += " AND (assigned_to_id IS NULL OR assigned_to_id = ANY($" + strconv.Itoa(len(args)) + "))"
	}
	cards, err := h.loadCards(where, args...)
	if err != nil {
		return nil, err
	}

	// With a team the columns are its members, including those without
	// issues; without one — everybody who has issues.
	columns := []KanbanColumn{}
	if len(team) > 0 {
		rows, err := (*h.DB).Query("SELECT external_id, name FROM members WHERE data_source = 'redmine' AND external_id = ANY($1) ORDER BY name",
			pq.Array(team))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return nil, err
			}
			columns = append(columns, KanbanColumn{ID: strconv.Itoa(id), Name: name})
		}
	} else {
		seen := map[int]bool{}
		for _, c := range cards {
			if c.AssignedToID != nil && !seen[*c.AssignedToID] {
				seen[*c.AssignedToID] = true
				columns = append(columns, KanbanColumn{ID: strconv.Itoa(*c.AssignedToID), Name: c.AssignedToName})
			}
		}
		sort.Slice(columns, func(i, j int) bool { return columns[i].Name < columns[j].Name })
	}
	// The title of this column is set on the client (it is translated).
	columns = append(columns, KanbanColumn{ID: unassignedColumn})

	columns = fillColumns(columns, cards, func(c KanbanCard) string {
		if c.AssignedToID == nil {
			return unassignedColumn
		}
		return strconv.Itoa(*c.AssignedToID)
	})
	if last := len(columns) - 1; len(columns[last].Tasks) == 0 {
		columns = columns[:last]
	}
	return orderColumns(columns, h.columnOrder(userID, "kanban_column_order_users")), nil
}

// loadCards reads issues for the board, most important first.
func (h *KanbanHandler) loadCards(where string, args ...interface{}) ([]KanbanCard, error) {
	rows, err := (*h.DB).Query(`SELECT external_id, subject, project_name, status_name, status_id,
		priority_name, priority_id, assigned_to_name, assigned_to_id,
		category_name, due_date, estimated_hours,
		COALESCE(`+statusNotIn("status_id", GroupClosed)+`, true)
		FROM issues WHERE data_source = 'redmine' AND `+where+`
		ORDER BY `+priorityRank("priority_id")+` DESC NULLS LAST, external_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().Format("2006-01-02")
	var cards []KanbanCard
	for rows.Next() {
		var c KanbanCard
		var notClosed bool
		if err := rows.Scan(&c.ExternalID, &c.Subject, &c.ProjectName, &c.StatusName, &c.StatusID,
			&c.PriorityName, &c.PriorityID, &c.AssignedToName, &c.AssignedToID,
			&c.CategoryName, &c.DueDate, &c.EstimatedHours, &notClosed); err != nil {
			return nil, err
		}
		// Overdue: the due date has passed and the issue is not closed
		c.IsOverdue = notClosed && c.DueDate != nil && (*c.DueDate)[:min(10, len(*c.DueDate))] < today
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// columnOrder returns the column ids saved by the user for a mode.
func (h *KanbanHandler) columnOrder(userID int, column string) []string {
	var data []byte
	(*h.DB).QueryRow("SELECT "+column+" FROM user_settings WHERE user_id = $1", userID).Scan(&data)
	var order []string
	if len(data) > 0 {
		json.Unmarshal(data, &order)
	}
	return order
}

// fillColumns puts every card into the column with the id key(card) returns;
// cards without a column are dropped. Tasks is never nil in the result.
func fillColumns(columns []KanbanColumn, cards []KanbanCard, key func(KanbanCard) string) []KanbanColumn {
	index := make(map[string]int, len(columns))
	for i := range columns {
		columns[i].Tasks = []KanbanCard{}
		index[columns[i].ID] = i
	}
	for _, c := range cards {
		if i, ok := index[key(c)]; ok {
			columns[i].Tasks = append(columns[i].Tasks, c)
		}
	}
	return columns
}

// orderColumns puts the columns listed in order (ids) first, in that order;
// the rest keep their relative position.
func orderColumns(columns []KanbanColumn, order []string) []KanbanColumn {
	position := make(map[string]int, len(order))
	for i, id := range order {
		if _, dup := position[id]; !dup {
			position[id] = i
		}
	}
	sort.SliceStable(columns, func(i, j int) bool {
		pi, okI := position[columns[i].ID]
		pj, okJ := position[columns[j].ID]
		if okI != okJ {
			return okI
		}
		return okI && pi < pj
	})
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

	notifyIssuesUpdated(h.Events, body.IssueID)
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
