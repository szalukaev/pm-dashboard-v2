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

			// Сессия ищется в Redis или в хранилище в памяти
			userID, err := sessionStore.Get(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			// Что видит пользователь, определяется один раз на запрос. У
			// удалённого или заблокированного пользователя Scope нет: его
			// сессии перестают работать сразу.
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

// RequireWSSession проверяет подключение WebSocket по куке сессии. Роль в
// базе не запрашивается: для событий достаточно действующей сессии.
func RequireWSSession(sessionStore db.SessionStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := sessionStore.Get(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Соединение знает своего пользователя: ему уходят личные события
		next(w, r.WithContext(context.WithValue(r.Context(), UserIDKey, userID)))
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

// GetScope возвращает то, что видит текущий пользователь. Вне RequireAuth
// это пустой Scope, который не показывает ничего.
func GetScope(r *http.Request) *access.Scope {
	if scope, ok := r.Context().Value(scopeKey).(*access.Scope); ok {
		return scope
	}
	return &access.Scope{ReadOnly: true, VisibleTabs: &[]string{}}
}

// WithScope возвращает запрос с заданным Scope; для тестов.
func WithScope(r *http.Request, scope *access.Scope) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), scopeKey, scope))
}

// RequireTab отклоняет запросы к данным вкладки, которую администратор
// скрыл от пользователя.
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

// TabRule связывает часть API с вкладкой, которая показывает её данные.
type TabRule struct {
	// Path сравнивается точно, а если Exact не задан — ещё и как префикс (Path + "/").
	Path  string
	Exact bool
	Tab   string
}

// RequireTabs применяет RequireTab ко всем запросам, подходящим под правило:
// скрытая от пользователя вкладка закрыта и на сервере, а не только в меню.
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

// RequireSprint отклоняет запрос, если пользователю не разрешено это
// действие со спринтами (константы access.Sprint*).
func RequireSprint(action string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !GetScope(r).CanSprint(action) {
			slog.Warn("Sprint action outside of user rights", "user", GetUserID(r), "action", action, "path", r.URL.Path)
			http.Error(w, `{"error":"SPRINT_ACTION_FORBIDDEN"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// RequireWrite отклоняет запросы пользователя с правом только на чтение.
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
