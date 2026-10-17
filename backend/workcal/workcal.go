// Package workcal отдаёт производственный календарь России: какие дни года
// рабочие, а какие — выходные и праздники (с учётом переносов). Источник —
// сервис isDayOff; ответ хранится в базе и перечитывается раз в неделю.
package workcal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// refreshAfter — как часто перечитывать календарь года: правительство может
// перенести выходные в течение года.
const refreshAfter = 7 * 24 * time.Hour

// Calendar — дни года строкой: i-й символ — (i+1)-й день, '1' — выходной или
// праздник, '0' — рабочий. Source: isdayoff — настоящий календарь, weekends —
// сервис недоступен, выходными считаются только суббота и воскресенье.
type Calendar struct {
	Year   int    `json:"year"`
	Days   string `json:"days"`
	Source string `json:"source"`
}

type stored struct {
	Days      string    `json:"days"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Service хранит календари в таблице settings (workcal_<год>).
type Service struct {
	DB *(*sql.DB)
	// URL возвращает адрес календаря года; в тестах подменяется.
	URL    func(year int) string
	Client *http.Client

	mu sync.Mutex
}

func New(db **sql.DB) *Service {
	return &Service{
		DB:     db,
		URL:    func(year int) string { return fmt.Sprintf("https://isdayoff.ru/api/getdata?year=%d&cc=ru", year) },
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func daysInYear(year int) int {
	return time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC).YearDay()
}

// Weekends — запасной календарь: выходные только суббота и воскресенье.
func Weekends(year int) string {
	var b strings.Builder
	for d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// valid — ответ сервиса похож на календарь года: по цифре на день.
func valid(days string, year int) bool {
	if len(days) != daysInYear(year) {
		return false
	}
	for _, c := range days {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// normalize оставляет два значения: '1' — нерабочий, '0' — рабочий
// (сокращённый предпраздничный день — рабочий).
func normalize(days string) string {
	return strings.Map(func(r rune) rune {
		if r == '1' {
			return '1'
		}
		return '0'
	}, days)
}

// Get возвращает календарь года: сохранённый, если он свежий; иначе
// перечитывает его у сервиса. Сервис недоступен — сохранённый, даже старый,
// а если его нет — запасной из суббот и воскресений.
func (s *Service) Get(year int) Calendar {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("workcal_%d", year)
	var cached stored
	var raw []byte
	haveCache := false
	if *s.DB != nil && (*s.DB).QueryRow(`SELECT value FROM settings WHERE key = $1`, key).Scan(&raw) == nil &&
		json.Unmarshal(raw, &cached) == nil && valid(cached.Days, year) {
		haveCache = true
		if time.Since(cached.FetchedAt) < refreshAfter {
			return Calendar{Year: year, Days: cached.Days, Source: "isdayoff"}
		}
	}
	if days, err := s.fetch(year); err == nil {
		if *s.DB != nil {
			data, _ := json.Marshal(stored{Days: days, FetchedAt: time.Now()})
			if _, err := (*s.DB).Exec(`INSERT INTO settings (key, value) VALUES ($1, $2::jsonb)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, string(data)); err != nil {
				slog.Warn("Work calendar: cache write failed", "year", year, "error", err)
			}
		}
		return Calendar{Year: year, Days: days, Source: "isdayoff"}
	} else {
		slog.Warn("Work calendar: service unavailable", "year", year, "error", err)
	}
	if haveCache {
		return Calendar{Year: year, Days: cached.Days, Source: "isdayoff"}
	}
	return Calendar{Year: year, Days: Weekends(year), Source: "weekends"}
}

func (s *Service) fetch(year int) (string, error) {
	resp, err := s.Client.Get(s.URL(year))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	days := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK || !valid(days, year) {
		return "", fmt.Errorf("unexpected answer: %d %q", resp.StatusCode, days[:min(len(days), 20)])
	}
	return normalize(days), nil
}
