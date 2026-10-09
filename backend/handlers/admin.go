package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/config"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	DB     **sql.DB
	SQLite *config.SQLiteStore
	Source *manager.Manager
}

// ─── User Management ───

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT id, username, role, created_at FROM users ORDER BY id")
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
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

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
	if !isValidRole(body.Role) {
		utils.Error(w, http.StatusBadRequest, "INVALID_ROLE")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}

	_, err = (*h.DB).Exec("INSERT INTO users (username, password_hash, role, force_password_change) VALUES ($1, $2, $3, true)",
		body.Username, string(hash), body.Role)
	if err != nil {
		utils.Error(w, http.StatusConflict, "USER_EXISTS")
		return
	}
	utils.Success(w)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Role != nil && !isValidRole(*body.Role) {
		utils.Error(w, http.StatusBadRequest, "INVALID_ROLE")
		return
	}
	if body.Password != nil && len(*body.Password) < 6 {
		utils.Error(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT")
		return
	}
	// An administrator cannot demote themselves — protects against losing the last admin.
	if body.Role != nil && *body.Role != "admin" && userID == middleware.GetUserID(r) {
		utils.Error(w, http.StatusBadRequest, "CANNOT_DEMOTE_SELF")
		return
	}

	if body.Role != nil {
		res, err := (*h.DB).Exec("UPDATE users SET role=$1 WHERE id=$2", *body.Role, userID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
			return
		}
	}
	if body.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcrypt.DefaultCost)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
			return
		}
		// A password set by an administrator is temporary: the user must
		// replace it on the next login (unless the admin resets their own).
		forceChange := userID != middleware.GetUserID(r)
		res, err := (*h.DB).Exec("UPDATE users SET password_hash=$1, force_password_change=$2 WHERE id=$3", string(hash), forceChange, userID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
			return
		}
	}
	utils.Success(w)
}

func isValidRole(role string) bool {
	return role == "admin" || role == "user"
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Don't delete yourself
	currentUserID := middleware.GetUserID(r)
	if userID == currentUserID {
		utils.Error(w, http.StatusBadRequest, "CANNOT_DELETE_SELF")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM users WHERE id=$1", userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Status Mapping ───

func (h *AdminHandler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, is_closed, group_name, is_bug FROM statuses ORDER BY name")
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
		IsBug      bool   `json:"is_bug"`
	}
	var statuses []StatusMapping
	for rows.Next() {
		var s StatusMapping
		if rows.Scan(&s.ExternalID, &s.Name, &s.IsClosed, &s.Group, &s.IsBug) == nil {
			statuses = append(statuses, s)
		}
	}
	if statuses == nil {
		statuses = []StatusMapping{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"statuses": statuses})
}

func (h *AdminHandler) UpdateStatusGroup(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	statusID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Either field may be sent alone: moving a status between groups and
	// marking it as a bug status are separate actions in the settings.
	var body struct {
		Group *string `json:"group"` // "open", "testing", "closed"
		IsBug *bool   `json:"is_bug"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Group == nil && body.IsBug == nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if body.Group != nil && *body.Group != GroupOpen && *body.Group != GroupTesting && *body.Group != GroupClosed {
		utils.Error(w, http.StatusBadRequest, "INVALID_GROUP")
		return
	}

	res, err := (*h.DB).Exec(`UPDATE statuses SET
		group_name = COALESCE($1::varchar, group_name),
		is_closed = COALESCE($1::varchar = 'closed', is_closed),
		is_bug = COALESCE($2::boolean, is_bug)
		WHERE external_id = $3`, body.Group, body.IsBug, statusID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "STATUS_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Priorities ───

func (h *AdminHandler) ListPriorities(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, sort_order, color FROM priorities ORDER BY sort_order")
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
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	priorityID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body struct {
		SortOrder *int    `json:"sort_order"`
		Color     *string `json:"color"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.SortOrder != nil {
		res, err := (*h.DB).Exec("UPDATE priorities SET sort_order=$1 WHERE external_id=$2", *body.SortOrder, priorityID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			utils.Error(w, http.StatusNotFound, "PRIORITY_NOT_FOUND")
			return
		}
	}
	if body.Color != nil {
		res, err := (*h.DB).Exec("UPDATE priorities SET color=$1 WHERE external_id=$2", *body.Color, priorityID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			utils.Error(w, http.StatusNotFound, "PRIORITY_NOT_FOUND")
			return
		}
	}
	utils.Success(w)
}

// ─── Data Source Config ───

func (h *AdminHandler) TestDataSourceConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string `json:"type"`
		URL         string `json:"url"`
		APIKey      string `json:"api_key"`
		BasicLogin  string `json:"basic_login"`
		BasicPasswd string `json:"basic_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Validate URL scheme — only http/https are allowed.
	warning := ""
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "URL must start with http:// or https://"})
		return
	}
	if u.Scheme == "http" {
		warning = "HTTP is not encrypted; use HTTPS in production"
	}

	client := &http.Client{Timeout: 8 * time.Second}
	httpReq, err := http.NewRequest("GET", req.URL+"/projects.json?limit=1", nil)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	if req.APIKey != "" {
		httpReq.Header.Set("X-Redmine-API-Key", req.APIKey)
	}
	if req.BasicLogin != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(req.BasicLogin + ":" + req.BasicPasswd))
		httpReq.Header.Set("Authorization", "Basic "+cred)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "HTTP " + resp.Status})
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "warning": warning})
}

// secretMask is returned for stored secrets so the UI can round-trip the form
// without exposing the real value. Save* ignores this mask and keeps the old value.
const secretMask = "••••••••"

// maskSecret returns secretMask if v is set, otherwise an empty string.
func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	return secretMask
}

// isSecretMask reports whether the incoming value is the UI placeholder.
func isSecretMask(v string) bool {
	return v == secretMask
}

// maskDSNPassword replaces the password part of a postgres URL DSN with ****.
// Non-URL DSNs are returned as-is (cannot reliably locate the password).
func maskDSNPassword(dsn string) string {
	if dsn == "" {
		return ""
	}
	// postgres://user:pass@host:port/db?opts
	if i := strings.Index(dsn, "://"); i >= 0 {
		rest := dsn[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			cred := rest[:at]
			if colon := strings.Index(cred, ":"); colon >= 0 {
				return dsn[:i+3] + cred[:colon+1] + "****" + rest[at:]
			}
			// no password in userinfo
			return dsn
		}
	}
	return dsn
}

func (h *AdminHandler) GetDataSourceConfig(w http.ResponseWriter, r *http.Request) {
	get := func(key string) string {
		v, _ := h.SQLite.Get(key)
		return v
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"config": map[string]interface{}{
			"type":           get("data_source_type"),
			"url":            get("redmine_url"),
			"api_key":        maskSecret(get("redmine_api_key")),
			"basic_login":    get("redmine_basic_login"),
			"basic_password": maskSecret(get("redmine_basic_password")),
		},
	})
}

func (h *AdminHandler) SaveDataSourceConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type        string `json:"type"`
		URL         string `json:"url"`
		APIKey      string `json:"api_key"`
		BasicLogin  string `json:"basic_login"`
		BasicPasswd string `json:"basic_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	h.SQLite.Set("data_source_type", body.Type)
	h.SQLite.Set("redmine_url", body.URL)
	h.SQLite.Set("redmine_basic_login", body.BasicLogin)
	// Masked values mean "unchanged" — don't overwrite real secrets.
	if !isSecretMask(body.APIKey) {
		h.SQLite.Set("redmine_api_key", body.APIKey)
	}
	if !isSecretMask(body.BasicPasswd) {
		h.SQLite.Set("redmine_basic_password", body.BasicPasswd)
	}

	// Apply on the fly: handlers use the new connection at once and a sync
	// with it starts right away.
	if h.Source != nil {
		h.Source.Reload()
		h.Source.TriggerSync()
	}

	utils.Success(w)
}

// ─── DB Config ───

func (h *AdminHandler) GetDBConfig(w http.ResponseWriter, r *http.Request) {
	dsn, _ := h.SQLite.Get("db_dsn")
	// Never return the raw password — mask it inside the DSN.
	cfg := map[string]string{"dsn": maskDSNPassword(dsn)}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"config": cfg})
}

func (h *AdminHandler) TestDBConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DSN string `json:"dsn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	// Cap connection setup time so a black-hole host can't hang the handler.
	dsn := req.DSN
	if strings.Contains(dsn, "connect_timeout=") {
		// already set by caller — leave as-is
	} else if strings.Contains(dsn, "?") {
		dsn += "&connect_timeout=5"
	} else {
		dsn += "?connect_timeout=5"
	}
	testDB, err := sql.Open("postgres", dsn)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	defer testDB.Close()
	if err := testDB.Ping(); err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ─── Sync Log ───

func (h *AdminHandler) GetSyncLog(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query(`SELECT id, started_at, finished_at, duration_ms, issues_collected, status, COALESCE(error_text, '')
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

// RunSync starts an unscheduled sync. The sync runs in the background; its
// outcome appears in the sync log.
func (h *AdminHandler) RunSync(w http.ResponseWriter, r *http.Request) {
	if h.Source == nil {
		utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
		return
	}
	switch err := h.Source.TriggerSync(); {
	case err == nil:
		utils.Success(w)
	case errors.Is(err, manager.ErrAlreadyRunning):
		utils.Error(w, http.StatusConflict, "SYNC_ALREADY_RUNNING")
	case errors.Is(err, manager.ErrNotConfigured):
		utils.Error(w, http.StatusBadRequest, "DATA_SOURCE_NOT_CONFIGURED")
	case errors.Is(err, manager.ErrReadOnly):
		utils.Error(w, http.StatusForbidden, "READ_ONLY_MODE")
	default:
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
	}
}

func (h *AdminHandler) GetSyncSettings(w http.ResponseWriter, r *http.Request) {
	if h.Source == nil {
		utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"interval_minutes": h.Source.Interval(),
		"min":              manager.MinInterval,
		"max":              manager.MaxInterval,
	})
}

func (h *AdminHandler) UpdateSyncSettings(w http.ResponseWriter, r *http.Request) {
	if h.Source == nil {
		utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
		return
	}
	var body struct {
		IntervalMinutes int `json:"interval_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if body.IntervalMinutes < manager.MinInterval || body.IntervalMinutes > manager.MaxInterval {
		utils.Error(w, http.StatusBadRequest, "INVALID_INTERVAL")
		return
	}
	if err := h.Source.SetInterval(body.IntervalMinutes); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}
	utils.Success(w)
}

// ─── Audit Log ───

func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

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

	rows, err := (*h.DB).Query(query, args...)
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
