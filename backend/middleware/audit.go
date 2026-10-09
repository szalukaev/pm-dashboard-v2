package middleware

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxAuditBodyBytes caps how much of a request body is read for the audit trail.
const maxAuditBodyBytes = 1 << 20 // 1 MiB

type AuditMiddleware struct {
	DB **sql.DB
}

type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (w *auditResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// ClientIP extracts the real client address:
// X-Real-IP → first X-Forwarded-For hop → host part of RemoteAddr.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First hop is the original client when the chain is built correctly.
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
		// Only audit mutating requests
		if r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "HEAD" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip login/logout (handled separately)
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Read request body (capped — a huge payload must not OOM the process)
		var bodyBytes []byte
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxAuditBodyBytes)
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Get user from context
		userID, _ := r.Context().Value(UserIDKey).(int)

		// Determine action and entity from method + path
		action := methodToAction(r.Method)
		entityType, entityID := parseEntity(r.URL.Path)

		// Wrap response writer
		aw := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(aw, r)

		// Log after response
		// before_state is a reserved column (JSONB) kept for future use;
		// it is not populated yet and defaults to NULL in the DB.
		var afterState interface{}
		if len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &afterState)
			afterState = maskSecrets(afterState)
		}
		clientIP := ClientIP(r)
		userAgent := r.UserAgent()
		statusCode := aw.statusCode

		// entity_id is BIGINT: a path without a numeric id (POST /api/contracts)
		// must be stored as NULL, otherwise the INSERT fails and the record is lost.
		var entityIDArg interface{}
		if n, err := strconv.ParseInt(entityID, 10, 64); err == nil {
			entityIDArg = n
		}

		go func() {
			afterJSON, _ := json.Marshal(afterState)

			(*m.DB).Exec(`INSERT INTO audit_log
				(occurred_at, user_id, action, entity_type, entity_id, after_state, status_code, ip_address, user_agent)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				time.Now(), userID, action, entityType, entityIDArg,
				string(afterJSON), statusCode, clientIP, userAgent)
		}()
	})
}

func (m *AuditMiddleware) LogLogin(userID int, username string, success bool, r *http.Request) {
	action := "login"
	if !success {
		action = "login_failed"
	}
	afterData, _ := json.Marshal(map[string]interface{}{"username": username, "success": success})
	// user_id references users(id): an unknown user (failed login) is stored as NULL.
	var userIDArg interface{}
	if userID > 0 {
		userIDArg = userID
	}
	(*m.DB).Exec(`INSERT INTO audit_log
		(occurred_at, user_id, action, entity_type, after_state, ip_address, user_agent)
		VALUES ($1, $2, $3, 'auth', $4, $5, $6)`,
		time.Now(), userIDArg, action, string(afterData), ClientIP(r), r.UserAgent())
}

func (m *AuditMiddleware) LogLogout(userID int, r *http.Request) {
	(*m.DB).Exec(`INSERT INTO audit_log
		(occurred_at, user_id, action, entity_type, ip_address, user_agent)
		VALUES ($1, $2, 'logout', 'auth', $3, $4)`,
		time.Now(), userID, ClientIP(r), r.UserAgent())
}

// auditSecretKeys are request body fields that must never reach audit_log.
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

// maskSecrets replaces secret values in a decoded JSON body (recursively).
func maskSecrets(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, inner := range val {
			if auditSecretKeys[strings.ToLower(k)] {
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
	// Clean up entity name
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
