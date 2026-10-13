package middleware

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Отладочное логирование по пользователю: пока оно включено администратором,
// каждый запрос пользователя записывается в debug_logs — адрес, тело, ответ,
// код и время выполнения. Запись идёт в фоне и запрос не задерживает.

const (
	// DebugPeriod — на сколько включается режим: дольше он сам не живёт.
	DebugPeriod = 24 * time.Hour
	// debugKeepDays — сколько дней хранятся записи.
	debugKeepDays = 7
	// maxDebugBody — тело больше этого не сохраняется, пишется только размер.
	maxDebugBody = 64 << 10
	// maxDebugIP — длина колонки ip_address в debug_logs.
	maxDebugIP = 64
	// debugQueueSize — сколько записей ждут своей очереди; сверх того теряются.
	debugQueueSize = 256
	// debugCacheTTL — как часто перечитывается список пользователей с режимом.
	debugCacheTTL = time.Minute
	// debugLogsPath — запросы к самому журналу отладки в него не пишутся.
	debugLogsPath = "/api/admin/debug-logs"
)

type debugEntry struct {
	userID       int
	at           time.Time
	method       string
	path         string
	query        string
	status       int
	durationMs   int
	requestBody  string
	responseBody string
	ip           string
}

// DebugLog знает, у кого режим включён, и пишет запросы в базу.
type DebugLog struct {
	DB **sql.DB

	mu     sync.RWMutex
	until  map[int]time.Time
	loaded time.Time
	queue  chan debugEntry
}

// NewDebugLog запускает фоновую запись и ежечасную очистку старых записей.
func NewDebugLog(db **sql.DB) *DebugLog {
	d := &DebugLog{DB: db, until: map[int]time.Time{}, queue: make(chan debugEntry, debugQueueSize)}
	go d.write()
	return d
}

// Refresh перечитывает, у кого режим включён. Вызывается сразу после
// переключения, чтобы оно подействовало без ожидания.
func (d *DebugLog) Refresh() {
	if *d.DB == nil {
		return
	}
	rows, err := (*d.DB).Query(`SELECT id, debug_until FROM users WHERE debug_until > NOW()`)
	if err != nil {
		// База не ответила: прежний список действует ещё один срок, иначе
		// каждый запрос пользователя повторял бы этот запрос к лежащей базе
		d.mu.Lock()
		d.loaded = time.Now()
		d.mu.Unlock()
		return
	}
	defer rows.Close()
	until := map[int]time.Time{}
	for rows.Next() {
		var id int
		var t time.Time
		if rows.Scan(&id, &t) == nil {
			until[id] = t
		}
	}
	d.mu.Lock()
	d.until, d.loaded = until, time.Now()
	d.mu.Unlock()
}

// active сообщает, включён ли режим у пользователя сейчас.
func (d *DebugLog) active(userID int) bool {
	d.mu.RLock()
	stale := time.Since(d.loaded) > debugCacheTTL
	d.mu.RUnlock()
	if stale {
		d.Refresh()
	}
	d.mu.RLock()
	t, ok := d.until[userID]
	d.mu.RUnlock()
	return ok && time.Now().Before(t)
}

// write записывает накопленное и раз в час удаляет записи старше срока хранения.
func (d *DebugLog) write() {
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case e := <-d.queue:
			if *d.DB == nil {
				continue
			}
			(*d.DB).Exec(`INSERT INTO debug_logs
				(user_id, occurred_at, method, path, query, status_code, duration_ms, request_body, response_body, ip_address)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				e.userID, e.at, e.method, e.path, e.query, e.status, e.durationMs, e.requestBody, e.responseBody, e.ip)
		case <-cleanup.C:
			if *d.DB != nil {
				(*d.DB).Exec(`DELETE FROM debug_logs WHERE occurred_at < NOW() - make_interval(days => $1)`, debugKeepDays)
			}
		}
	}
}

// debugResponseWriter запоминает код и начало ответа.
type debugResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	size   int
}

func (w *debugResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *debugResponseWriter) Write(b []byte) (int, error) {
	w.size += len(b)
	// На один байт больше предела: по нему видно, что ответ не поместился
	if room := maxDebugBody + 1 - w.body.Len(); room > 0 {
		if len(b) < room {
			room = len(b)
		}
		w.body.Write(b[:room])
	}
	return w.ResponseWriter.Write(b)
}

// Log записывает запросы пользователей с включённым режимом. Ставится после
// авторизации: до неё пользователь неизвестен.
func (d *DebugLog) Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		if userID == 0 || strings.HasPrefix(r.URL.Path, debugLogsPath) || !d.active(userID) {
			next.ServeHTTP(w, r)
			return
		}

		// Тело читается целиком, только если оно заведомо невелико; файлы и
		// большие тела идут обработчику нетронутыми
		requestBody := ""
		contentType := r.Header.Get("Content-Type")
		switch {
		case r.Body == nil || r.ContentLength == 0:
		case strings.HasPrefix(contentType, "multipart/") || r.ContentLength < 0 || r.ContentLength > maxDebugBody:
			requestBody = bodyNote(contentType, r.ContentLength)
		default:
			raw, _ := io.ReadAll(io.LimitReader(r.Body, maxDebugBody+1))
			r.Body = io.NopCloser(bytes.NewReader(raw))
			requestBody = debugBody(raw, contentType)
		}

		started := time.Now()
		dw := &debugResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(dw, r)

		responseBody := ""
		if dw.size > 0 {
			responseType := dw.Header().Get("Content-Type")
			if dw.size > maxDebugBody {
				responseBody = bodyNote(responseType, int64(dw.size))
			} else {
				responseBody = debugBody(dw.body.Bytes(), responseType)
			}
		}

		entry := debugEntry{
			userID: userID, at: started, method: r.Method, path: r.URL.Path, query: maskQuery(r.URL.Query()),
			status: dw.status, durationMs: int(time.Since(started).Milliseconds()),
			requestBody: requestBody, responseBody: responseBody, ip: ClientIP(r),
		}
		// Адрес приходит из заголовка и может быть любой длины, а колонка — нет:
		// слишком длинный оборвал бы запись всей строки
		if len(entry.ip) > maxDebugIP {
			entry.ip = entry.ip[:maxDebugIP]
		}
		// Очередь полна — запись теряется: отладка не должна тормозить работу
		select {
		case d.queue <- entry:
		default:
		}
	})
}

// bodyNote — пометка вместо тела, которое не сохраняется.
func bodyNote(contentType string, size int64) string {
	if contentType == "" {
		contentType = "unknown type"
	}
	if size < 0 {
		return "[" + contentType + ", size unknown — not saved]"
	}
	return "[" + contentType + ", " + strconv.FormatInt(size, 10) + " bytes — not saved]"
}

// debugBody готовит тело к записи. Сохраняется только JSON: в нём секреты
// находятся по именам полей и заменяются. Всё остальное могло бы унести
// секрет в журнал, поэтому вместо него пишется пометка с размером.
func debugBody(raw []byte, contentType string) string {
	if len(raw) == 0 {
		return ""
	}
	var decoded interface{}
	if len(raw) > maxDebugBody || json.Unmarshal(raw, &decoded) != nil {
		return bodyNote(contentType, int64(len(raw)))
	}
	masked, err := json.Marshal(maskSecrets(decoded))
	if err != nil {
		return bodyNote(contentType, int64(len(raw)))
	}
	return string(masked)
}

// maskQuery возвращает строку параметров с заменёнными секретами.
func maskQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	masked := url.Values{}
	for key, list := range values {
		if isSecretKey(key) {
			masked[key] = []string{auditSecretMask}
			continue
		}
		masked[key] = list
	}
	return masked.Encode()
}
