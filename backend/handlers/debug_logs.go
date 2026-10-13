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

	"github.com/gorilla/mux"
)

// Отладочное логирование по пользователю: включение, журнал и его очистка.
// Сама запись запросов — в middleware/debug.go.

type DebugHandler struct {
	DB  **sql.DB
	Log *middleware.DebugLog
}

// SetUserDebug включает режим на сутки или выключает его.
func (h *DebugHandler) SetUserDebug(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var until *time.Time
	if body.Enabled {
		t := time.Now().Add(middleware.DebugPeriod)
		until = &t
	}
	res, err := (*h.DB).Exec("UPDATE users SET debug_until = $1 WHERE id = $2", until, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}
	// Действует сразу, без ожидания очередного перечитывания
	h.Log.Refresh()
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "debug_until": until})
}

// debugFilter собирает условие отбора записей из параметров запроса:
// user_id, method, path (часть адреса), from и to (даты ГГГГ-ММ-ДД включительно).
func debugFilter(r *http.Request) (where string, args []interface{}, ok bool) {
	q := r.URL.Query()
	conds := []string{"1=1"}
	add := func(cond string, value interface{}) {
		args = append(args, value)
		conds = append(conds, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if v := q.Get("user_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return "", nil, false
		}
		add("d.user_id = ?", id)
	}
	if v := strings.ToUpper(q.Get("method")); v != "" {
		add("d.method = ?", v)
	}
	if v := strings.TrimSpace(q.Get("path")); v != "" {
		add("d.path ILIKE '%' || ? || '%'", v)
	}
	if v := q.Get("from"); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", nil, false
		}
		add("d.occurred_at >= ?::date", v)
	}
	if v := q.Get("to"); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", nil, false
		}
		add("d.occurred_at < ?::date + 1", v)
	}
	return strings.Join(conds, " AND "), args, true
}

type debugLogRow struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurred_at"`
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Query      string `json:"query"`
	StatusCode int    `json:"status_code"`
	DurationMs int    `json:"duration_ms"`
	IPAddress  string `json:"ip_address"`
	// Тела отдаются только в записи, открытой отдельно
	RequestBody  *string `json:"request_body,omitempty"`
	ResponseBody *string `json:"response_body,omitempty"`
}

// List — страница журнала без тел запросов и ответов.
func (h *DebugHandler) List(w http.ResponseWriter, r *http.Request) {
	where, args, ok := debugFilter(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_FILTER")
		return
	}
	limit, offset := pageOf(r)
	if limit == 0 {
		limit = 25
	}
	total := 0
	if err := (*h.DB).QueryRow("SELECT COUNT(*) FROM debug_logs d WHERE "+where, args...).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	rows, err := (*h.DB).Query(`SELECT d.id, d.occurred_at, d.user_id, COALESCE(u.username, ''), d.method, d.path,
			d.query, d.status_code, d.duration_ms, d.ip_address
		FROM debug_logs d LEFT JOIN users u ON u.id = d.user_id
		WHERE `+where+` ORDER BY d.occurred_at DESC, d.id DESC`+pageClause(limit, offset), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()
	logs := []debugLogRow{}
	for rows.Next() {
		var l debugLogRow
		if rows.Scan(&l.ID, &l.OccurredAt, &l.UserID, &l.Username, &l.Method, &l.Path,
			&l.Query, &l.StatusCode, &l.DurationMs, &l.IPAddress) == nil {
			logs = append(logs, l)
		}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"logs": logs, "total": total})
}

// Get — одна запись целиком, с телами запроса и ответа.
func (h *DebugHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var l debugLogRow
	var request, response string
	err = (*h.DB).QueryRow(`SELECT d.id, d.occurred_at, d.user_id, COALESCE(u.username, ''), d.method, d.path,
			d.query, d.status_code, d.duration_ms, d.ip_address, d.request_body, d.response_body
		FROM debug_logs d LEFT JOIN users u ON u.id = d.user_id WHERE d.id = $1`, id).
		Scan(&l.ID, &l.OccurredAt, &l.UserID, &l.Username, &l.Method, &l.Path,
			&l.Query, &l.StatusCode, &l.DurationMs, &l.IPAddress, &request, &response)
	if err == sql.ErrNoRows {
		utils.Error(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	l.RequestBody, l.ResponseBody = &request, &response
	utils.JSON(w, http.StatusOK, map[string]interface{}{"log": l})
}

// Clear удаляет записи выбранного пользователя. Без пользователя не
// работает: стереть журналы всех разом одним запросом нельзя.
func (h *DebugHandler) Clear(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(r.URL.Query().Get("user_id"))
	if err != nil || userID <= 0 {
		utils.Error(w, http.StatusBadRequest, "USER_REQUIRED")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM debug_logs WHERE user_id = $1", userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	deleted, _ := res.RowsAffected()
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "deleted": deleted})
}
