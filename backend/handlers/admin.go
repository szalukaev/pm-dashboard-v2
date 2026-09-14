package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	DB *sql.DB
}

// ─── User Management ───

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type UserInfo struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	var users []UserInfo
	for rows.Next() {
		var u UserInfo
		if rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt) == nil {
			users = append(users, u)
		}
	}
	if users == nil {
		users = []UserInfo{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if len(body.Username) < 3 || len(body.Password) < 6 {
		utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
		return
	}
	if body.Role == "" {
		body.Role = "user"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}

	_, err = h.DB.Exec("INSERT INTO users (username, password_hash, role, force_password_change) VALUES ($1, $2, $3, true)",
		body.Username, string(hash), body.Role)
	if err != nil {
		utils.Error(w, http.StatusConflict, "USER_EXISTS")
		return
	}
	utils.Success(w)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.Atoi(mux.Vars(r)["id"])
	var body struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Role != nil {
		h.DB.Exec("UPDATE users SET role=$1 WHERE id=$2", *body.Role, userID)
	}
	if body.Password != nil && len(*body.Password) >= 6 {
		hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcrypt.DefaultCost)
		if err == nil {
			h.DB.Exec("UPDATE users SET password_hash=$1 WHERE id=$2", string(hash), userID)
		}
	}
	utils.Success(w)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.Atoi(mux.Vars(r)["id"])
	// Don't delete yourself
	currentUserID := middleware.GetUserID(r)
	if userID == currentUserID {
		utils.Error(w, http.StatusBadRequest, "CANNOT_DELETE_SELF")
		return
	}
	h.DB.Exec("DELETE FROM users WHERE id=$1", userID)
	utils.Success(w)
}

// ─── Status Mapping ───

func (h *AdminHandler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT external_id, name, is_closed, group_name FROM statuses ORDER BY name")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type StatusMapping struct {
		ExternalID int    `json:"external_id"`
		Name       string `json:"name"`
		IsClosed   bool   `json:"is_closed"`
		Group      string `json:"group"`
	}
	var statuses []StatusMapping
	for rows.Next() {
		var s StatusMapping
		if rows.Scan(&s.ExternalID, &s.Name, &s.IsClosed, &s.Group) == nil {
			statuses = append(statuses, s)
		}
	}
	if statuses == nil {
		statuses = []StatusMapping{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"statuses": statuses})
}

func (h *AdminHandler) UpdateStatusGroup(w http.ResponseWriter, r *http.Request) {
	statusID, _ := strconv.Atoi(mux.Vars(r)["id"])
	var body struct {
		Group string `json:"group"` // "open", "testing", "closed"
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Group != "open" && body.Group != "testing" && body.Group != "closed" {
		utils.Error(w, http.StatusBadRequest, "INVALID_GROUP")
		return
	}

	isClosed := body.Group == "closed"
	h.DB.Exec("UPDATE statuses SET group_name=$1, is_closed=$2 WHERE external_id=$3", body.Group, isClosed, statusID)
	utils.Success(w)
}

// ─── Priorities ───

func (h *AdminHandler) ListPriorities(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT external_id, name, sort_order, color FROM priorities ORDER BY sort_order")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type PriorityInfo struct {
		ExternalID int    `json:"external_id"`
		Name       string `json:"name"`
		SortOrder  int    `json:"sort_order"`
		Color      string `json:"color"`
	}
	var priorities []PriorityInfo
	for rows.Next() {
		var p PriorityInfo
		if rows.Scan(&p.ExternalID, &p.Name, &p.SortOrder, &p.Color) == nil {
			priorities = append(priorities, p)
		}
	}
	if priorities == nil {
		priorities = []PriorityInfo{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"priorities": priorities})
}

func (h *AdminHandler) UpdatePriority(w http.ResponseWriter, r *http.Request) {
	priorityID, _ := strconv.Atoi(mux.Vars(r)["id"])
	var body struct {
		SortOrder *int    `json:"sort_order"`
		Color     *string `json:"color"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.SortOrder != nil {
		h.DB.Exec("UPDATE priorities SET sort_order=$1 WHERE external_id=$2", *body.SortOrder, priorityID)
	}
	if body.Color != nil {
		h.DB.Exec("UPDATE priorities SET color=$1 WHERE external_id=$2", *body.Color, priorityID)
	}
	utils.Success(w)
}

// ─── Data Source Config ───

func (h *AdminHandler) GetDataSourceConfig(w http.ResponseWriter, r *http.Request) {
	// This reads from admin_settings
	var configJSON []byte
	err := h.DB.QueryRow("SELECT value FROM admin_settings WHERE key='data_source_config'").Scan(&configJSON)
	if err == sql.ErrNoRows {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"config": nil})
		return
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	var config interface{}
	json.Unmarshal(configJSON, &config)
	utils.JSON(w, http.StatusOK, map[string]interface{}{"config": config})
}

func (h *AdminHandler) SaveDataSourceConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	data, _ := json.Marshal(body)
	h.DB.Exec(`INSERT INTO admin_settings (key, value, updated_at) VALUES ('data_source_config', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value=$1, updated_at=NOW()`, data)
	utils.Success(w)
}

// ─── Sync Log ───

func (h *AdminHandler) GetSyncLog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`SELECT id, started_at, finished_at, duration_ms, issues_collected, status, COALESCE(error_text, '')
		FROM collection_log ORDER BY started_at DESC LIMIT 50`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type LogEntry struct {
		ID              int     `json:"id"`
		StartedAt       string  `json:"started_at"`
		FinishedAt      *string `json:"finished_at"`
		DurationMs      *int    `json:"duration_ms"`
		IssuesCollected int     `json:"issues_collected"`
		Status          string  `json:"status"`
		ErrorText       string  `json:"error_text"`
	}
	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		if rows.Scan(&l.ID, &l.StartedAt, &l.FinishedAt, &l.DurationMs, &l.IssuesCollected, &l.Status, &l.ErrorText) == nil {
			logs = append(logs, l)
		}
	}
	if logs == nil {
		logs = []LogEntry{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

// ─── Audit Log ───

func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	actionFilter := r.URL.Query().Get("action")
	entityFilter := r.URL.Query().Get("entity")

	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if actionFilter != "" {
		where = append(where, "a.action = $"+utils.Itoa(idx))
		args = append(args, actionFilter)
		idx++
	}
	if entityFilter != "" {
		where = append(where, "a.entity_type = $"+utils.Itoa(idx))
		args = append(args, entityFilter)
		idx++
	}

	query := `SELECT a.id, a.occurred_at, a.user_id, COALESCE(u.username, ''),
		a.action, COALESCE(a.entity_type, ''), COALESCE(CAST(a.entity_id AS TEXT), ''),
		a.before_state, a.after_state, COALESCE(a.ip_address, '')
		FROM audit_log a LEFT JOIN users u ON a.user_id = u.id
		WHERE ` + utils.JoinStrings(where, " AND ") + `
		ORDER BY a.occurred_at DESC LIMIT 100`

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type AuditEntry struct {
		ID          int     `json:"id"`
		OccurredAt  string  `json:"occurred_at"`
		UserID      *int    `json:"user_id"`
		Username    string  `json:"username"`
		Action      string  `json:"action"`
		EntityType  string  `json:"entity_type"`
		EntityID    string  `json:"entity_id"`
		BeforeState *string `json:"before_state"`
		AfterState  *string `json:"after_state"`
		IPAddress   string  `json:"ip_address"`
	}
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if rows.Scan(&e.ID, &e.OccurredAt, &e.UserID, &e.Username,
			&e.Action, &e.EntityType, &e.EntityID,
			&e.BeforeState, &e.AfterState, &e.IPAddress) == nil {
			entries = append(entries, e)
		}
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"logs": entries})
}
