package license

import (
	"net/http"
	"strings"
	"time"
)

// recheckAfter is how soon an unactivated installation is checked again on
// a request: the database may have just appeared (setup wizard) or a
// license may have just been stored.
const recheckAfter = 10 * time.Second

// Guard enforces the license state on the API:
//   - not activated — nothing works except logging in and the activation screen;
//   - read-only — data can be read but not changed.
//
// Requests under /api/license and /api/auth always pass: the activation
// screen must stay reachable in every state.
func (s *Service) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/license") || strings.HasPrefix(path, "/api/auth") {
			next.ServeHTTP(w, r)
			return
		}

		status := s.Status()
		if status.State == NotActivated && s.now().Sub(status.CheckedAt) > recheckAfter {
			status = s.Check()
		}

		switch {
		case status.State == NotActivated:
			http.Error(w, `{"error":"LICENSE_NOT_ACTIVATED"}`, http.StatusForbidden)
		case status.State == ReadOnly && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions:
			http.Error(w, `{"error":"READ_ONLY_MODE"}`, http.StatusForbidden)
		default:
			next.ServeHTTP(w, r)
		}
	})
}
