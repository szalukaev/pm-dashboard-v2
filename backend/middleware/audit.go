package middleware

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxAuditBodyBytes — сколько байт тела запроса читается для журнала аудита.
const maxAuditBodyBytes = 1 << 20 // 1 МиБ

// maxAuditResponseBytes — сколько байт ответа запоминается, чтобы найти id
// созданной записи; обрезанный ответ просто не даст id.
const maxAuditResponseBytes = 64 << 10 // 64 КиБ

type AuditMiddleware struct {
	DB **sql.DB
}

type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

// Unwrap открывает исходный ResponseWriter для http.ResponseController.
func (w *auditResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *auditResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	// Из ответа нужен только id созданной записи: большой ответ целиком
	// в памяти не держится
	if room := maxAuditResponseBytes - w.body.Len(); room > 0 {
		if len(b) < room {
			room = len(b)
		}
		w.body.Write(b[:room])
	}
	return w.ResponseWriter.Write(b)
}

// ClientIP определяет настоящий адрес клиента:
// X-Real-IP → первый адрес в X-Forwarded-For → хост из RemoteAddr.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Первый адрес — исходный клиент, если цепочка собрана правильно.
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (m *AuditMiddleware) Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// В журнал идут только изменяющие запросы
		if r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "HEAD" {
			next.ServeHTTP(w, r)
			return
		}

		// Вход и выход записываются отдельно
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Тело читается с пределом: огромный запрос не должен исчерпать память
		var bodyBytes []byte
		// Загружаемый файл в журнале не нужен и может быть больше предела:
		// его читает сам обработчик.
		if r.Body != nil && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			r.Body = http.MaxBytesReader(w, r.Body, maxAuditBodyBytes)
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Пользователь — из контекста запроса
		userID, _ := r.Context().Value(UserIDKey).(int)

		// Действие и сущность определяются по методу и пути
		action := methodToAction(r.Method)
		entityType, entityID := parseEntity(r.URL.Path)

		// Обёртка запоминает код ответа
		aw := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Сам запрос
		next.ServeHTTP(aw, r)

		// Запись делается после ответа.
		// before_state — столбец (JSONB), оставленный на будущее: пока не
		// заполняется и в базе равен NULL.
		var afterState interface{}
		if len(bodyBytes) > 0 {
			_ = json.Unmarshal(bodyBytes, &afterState)
			afterState = maskSecrets(afterState)
		}
		clientIP := ClientIP(r)
		userAgent := r.UserAgent()
		statusCode := aw.statusCode

		// entity_id — BIGINT: путь без числового id (POST /api/contracts)
		// записывается как NULL, иначе INSERT падает и запись теряется.
		var entityIDArg interface{}
		if n, err := strconv.ParseInt(entityID, 10, 64); err == nil {
			entityIDArg = n
		} else if n, ok := createdID(aw.statusCode, aw.body.Bytes()); ok {
			// У создания id в пути нет — он есть в ответе сервера
			entityIDArg = n
		}

		go func() {
			afterJSON, _ := json.Marshal(afterState)

			if _, err := (*m.DB).Exec(`INSERT INTO audit_log
				(occurred_at, user_id, action, entity_type, entity_id, after_state, status_code, ip_address, user_agent)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				time.Now(), userID, action, entityType, entityIDArg,
				string(afterJSON), statusCode, clientIP, userAgent); err != nil {
				slog.Warn("Database write failed", "at", "Log", "error", err)
			}
		}()
	})
}

func (m *AuditMiddleware) LogLogin(userID int, username string, success bool, r *http.Request) {
	action := "login"
	if !success {
		action = "login_failed"
	}
	afterData, _ := json.Marshal(map[string]interface{}{"username": username, "success": success})
	// user_id ссылается на users(id): неизвестный пользователь (неудачный вход) записывается как NULL.
	var userIDArg interface{}
	if userID > 0 {
		userIDArg = userID
	}
	if _, err := (*m.DB).Exec(`INSERT INTO audit_log
		(occurred_at, user_id, action, entity_type, after_state, ip_address, user_agent)
		VALUES ($1, $2, $3, 'auth', $4, $5, $6)`,
		time.Now(), userIDArg, action, string(afterData), ClientIP(r), r.UserAgent()); err != nil {
		slog.Warn("Database write failed", "at", "LogLogin", "error", err)
	}
}

func (m *AuditMiddleware) LogLogout(userID int, r *http.Request) {
	if _, err := (*m.DB).Exec(`INSERT INTO audit_log
		(occurred_at, user_id, action, entity_type, ip_address, user_agent)
		VALUES ($1, $2, 'logout', 'auth', $3, $4)`,
		time.Now(), userID, ClientIP(r), r.UserAgent()); err != nil {
		slog.Warn("Database write failed", "at", "LogLogout", "error", err)
	}
}

// auditSecretKeys — поля тела запроса, которые не должны попасть в audit_log.
var auditSecretKeys = map[string]bool{
	"password":         true,
	"new_password":     true,
	"current_password": true,
	"api_key":          true,
	"token":            true,
	"basic_password":   true,
	"dsn":              true,
	"license_blob":     true,
}

const auditSecretMask = "***"

// isSecretKey сообщает, хранит ли поле с таким именем секрет: имя из списка
// либо любое, где есть password, token или secret (smtp_password, source_token).
func isSecretKey(name string) bool {
	name = strings.ToLower(name)
	if auditSecretKeys[name] {
		return true
	}
	for _, part := range []string{"password", "passwd", "token", "secret"} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

// maskSecrets заменяет секретные значения в разобранном теле JSON (рекурсивно).
func maskSecrets(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, inner := range val {
			if isSecretKey(k) {
				val[k] = auditSecretMask
				continue
			}
			val[k] = maskSecrets(inner)
		}
		return val
	case []interface{}:
		for i, inner := range val {
			val[i] = maskSecrets(inner)
		}
		return val
	default:
		return v
	}
}

func methodToAction(method string) string {
	switch method {
	case "POST":
		return "created"
	case "PUT", "PATCH":
		return "updated"
	case "DELETE":
		return "deleted"
	default:
		return method
	}
}

func parseEntity(path string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/"), "/")
	if len(parts) == 0 {
		return "", ""
	}
	entity := parts[0]
	entityID := ""
	if len(parts) > 1 {
		entityID = parts[1]
	}
	// Имя сущности приводится к принятому виду
	switch entity {
	case "organizations":
		return "organization", entityID
	case "contracts":
		return "contract", entityID
	case "sprints":
		return "sprint", entityID
	case "tasks":
		return "task", entityID
	case "settings":
		return "settings", entityID
	case "admin":
		if len(parts) > 2 {
			return "admin_" + parts[1], parts[2]
		}
		return "admin", ""
	default:
		return entity, entityID
	}
}

// createdID достаёт id созданной записи из успешного ответа: {"id": N}
// или {"data": {"id": N}}
func createdID(status int, body []byte) (int64, bool) {
	if status >= 300 || len(body) == 0 {
		return 0, false
	}
	var resp struct {
		ID   *int64 `json:"id"`
		Data struct {
			ID *int64 `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return 0, false
	}
	if resp.ID != nil {
		return *resp.ID, true
	}
	if resp.Data.ID != nil {
		return *resp.Data.ID, true
	}
	return 0, false
}
