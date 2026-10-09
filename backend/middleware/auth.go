package middleware

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"pm-dashboard/access"
	"pm-dashboard/db"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	scopeKey    contextKey = "access_scope"
)

type UserClaims struct {
	ID       int
	Username string
	Role     string
}

func RequireAuth(sessionStore db.SessionStore, pgDB **sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pgDB == nil || *pgDB == nil {
				http.Error(w, `{"error":"DATABASE_NOT_AVAILABLE"}`, http.StatusServiceUnavailable)
				return
			}
			cookie, err := r.Cookie("session_id")
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			// Look up session in Redis/memory store
			userID, err := sessionStore.Get(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			// What the user may see is resolved once per request. A deleted or
			// blocked user has no scope: their sessions stop working at once.
			scope, err := (&access.Resolver{DB: pgDB}).Scope(userID)
			if err != nil {
				http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}
			role := "user"
			if scope.Admin {
				role = "admin"
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			ctx = context.WithValue(ctx, scopeKey, scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireWSSession authenticates a WebSocket upgrade via the session cookie.
// DB role lookup is skipped — presence of a valid session is enough for events.
func RequireWSSession(sessionStore db.SessionStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := sessionStore.Get(r.Context(), cookie.Value); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(UserRoleKey).(string)
		if !ok || role != "admin" {
			http.Error(w, `{"error":"FORBIDDEN"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetScope returns what the current user may see. Outside RequireAuth it is
// an empty scope that shows nothing.
func GetScope(r *http.Request) *access.Scope {
	if scope, ok := r.Context().Value(scopeKey).(*access.Scope); ok {
		return scope
	}
	return &access.Scope{ReadOnly: true, VisibleTabs: &[]string{}}
}

// WithScope returns the request with the given scope; for tests.
func WithScope(r *http.Request, scope *access.Scope) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), scopeKey, scope))
}

// RequireTab rejects requests to the data of a tab the administrator hid
// from the user.
func RequireTab(tab string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !GetScope(r).TabVisible(tab) {
				slog.Warn("Request to a hidden tab", "user", GetUserID(r), "tab", tab, "path", r.URL.Path)
				http.Error(w, `{"error":"FORBIDDEN"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TabRule ties a part of the API to the tab that shows its data.
type TabRule struct {
	// Path is matched exactly, and as a prefix (Path + "/") unless Exact.
	Path  string
	Exact bool
	Tab   string
}

// RequireTabs applies RequireTab to every request matching a rule, so a tab
// hidden from a user is closed on the server and not only in the menu.
func RequireTabs(rules []TabRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			for _, rule := range rules {
				if path == rule.Path || (!rule.Exact && strings.HasPrefix(path, rule.Path+"/")) {
					RequireTab(rule.Tab)(next).ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireWrite rejects requests of a read-only user.
func RequireWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetScope(r).ReadOnly {
			http.Error(w, `{"error":"READ_ONLY_USER"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GetUserID(r *http.Request) int {
	id, _ := r.Context().Value(UserIDKey).(int)
	return id
}

func GetUserRole(r *http.Request) string {
	role, _ := r.Context().Value(UserRoleKey).(string)
	return role
}
