package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"pm-dashboard/db"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
)

type UserClaims struct {
	ID       int
	Username string
	Role     string
}

func RequireAuth(sessionStore db.SessionStore, pgDB *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			// Get user role from DB
			var role string
			err = pgDB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
			if err != nil {
				http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
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

func GetUserID(r *http.Request) int {
	id, _ := r.Context().Value(UserIDKey).(int)
	return id
}

func GetUserRole(r *http.Request) string {
	role, _ := r.Context().Value(UserRoleKey).(string)
	return role
}
