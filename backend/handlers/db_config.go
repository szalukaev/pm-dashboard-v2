package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"pm-dashboard/utils"
)

// Параметры подключения к базе на экране «Подключения». Строка подключения
// читается из SQLite один раз при старте, поэтому новая вступает в силу после
// перезапуска сервера: прежняя сохраняется, и если с новой база при старте
// недоступна, сервер возвращается к прежней (см. main.go).

// Ключи SQLite: действующая строка подключения и та, что была до замены.
const (
	dbDSNKey     = "db_dsn"
	dbDSNPrevKey = "db_dsn_prev"
)

// parseDSN раскладывает строку подключения на поля формы. Строку, которая
// не является адресом postgres://, разобрать нельзя — поля остаются пустыми.
func parseDSN(dsn string) dbTestRequest {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return dbTestRequest{}
	}
	req := dbTestRequest{Host: u.Hostname(), Port: portOrDefault(u.Port()), DBName: strings.TrimPrefix(u.Path, "/")}
	if u.User != nil {
		req.User = u.User.Username()
		req.Password, _ = u.User.Password()
	}
	return req
}

func (h *AdminHandler) GetDBConfig(w http.ResponseWriter, r *http.Request) {
	dsn, _ := h.SQLite.Get(dbDSNKey)
	cfg := parseDSN(dsn)
	// Пароль наружу не отдаётся: вместо него метка «не менялся»
	cfg.Password = maskSecret(cfg.Password)
	utils.JSON(w, http.StatusOK, map[string]interface{}{"config": cfg})
}

// dbConfigRequest читает поля формы. Пароль-метка заменяется сохранённым.
func (h *AdminHandler) dbConfigRequest(r *http.Request) (dbTestRequest, bool) {
	var req dbTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, false
	}
	req.Host = strings.TrimSpace(req.Host)
	req.Port = portOrDefault(strings.TrimSpace(req.Port))
	req.User = strings.TrimSpace(req.User)
	req.DBName = strings.TrimSpace(req.DBName)
	if isSecretMask(req.Password) {
		dsn, _ := h.SQLite.Get(dbDSNKey)
		req.Password = parseDSN(dsn).Password
	}
	return req, true
}

// checkDatabase проверяет, что с этими параметрами система сможет работать:
// база отвечает и в ней есть незаблокированный администратор. Пустая или
// чужая база оставила бы систему без единого входа. Возвращает код ошибки
// для интерфейса и её техническое описание; пустой код — всё в порядке.
func checkDatabase(req dbTestRequest) (code, detail string) {
	port, err := strconv.Atoi(req.Port)
	if req.Host == "" || req.User == "" || err != nil || port < 1 || port > 65535 {
		return "DB_FIELDS_REQUIRED", ""
	}
	if !dbNamePattern.MatchString(req.DBName) {
		return "INVALID_DB_NAME", ""
	}
	conn, err := sql.Open("postgres", buildDSN(req))
	if err != nil {
		return "DB_CONNECTION_FAILED", err.Error()
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return "DB_CONNECTION_FAILED", err.Error()
	}
	var admins int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND NOT COALESCE(is_blocked, false)`).Scan(&admins); err != nil || admins == 0 {
		return "DB_NO_ADMIN", ""
	}
	return "", ""
}

func (h *AdminHandler) TestDBConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := h.dbConfigRequest(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if code, detail := checkDatabase(req); code != "" {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": code, "detail": detail})
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// SaveDBConfig сохраняет новые параметры и перезапускает сервер. Параметры
// проверяются ещё раз: между проверкой и сохранением форма могла измениться.
func (h *AdminHandler) SaveDBConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := h.dbConfigRequest(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if code, detail := checkDatabase(req); code != "" {
		utils.JSON(w, http.StatusBadRequest, map[string]interface{}{"error": code, "detail": detail})
		return
	}
	current, _ := h.SQLite.Get(dbDSNKey)
	dsn := buildDSN(req)
	// Сравниваются и сами параметры: та же база, записанная в SQLite строкой
	// другого вида, перезапуска не требует
	if dsn == current || parseDSN(current) == req {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "restart": false})
		return
	}
	if err := h.SQLite.Set(dbDSNPrevKey, current); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}
	if err := h.SQLite.Set(dbDSNKey, dsn); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}
	slog.Warn("Database connection changed, the server restarts", "host", req.Host, "database", req.DBName)
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "restart": h.Restart != nil})
	if h.Restart != nil {
		h.Restart()
	}
}
