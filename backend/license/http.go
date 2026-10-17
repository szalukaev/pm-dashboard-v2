package license

import (
	"net/http"
	"strings"
	"time"
)

// recheckAfter — через сколько неактивированная установка проверяется снова
// при запросе: база могла только что появиться (мастер настройки), а
// лицензия — только что сохраниться.
const recheckAfter = 10 * time.Second

// Guard применяет состояние лицензии к API:
//   - не активирована — работают только вход и экран активации;
//   - только чтение — данные можно читать, но не менять.
//
// Запросы к /api/license и /api/auth проходят всегда: экран активации
// должен быть доступен в любом состоянии.
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
