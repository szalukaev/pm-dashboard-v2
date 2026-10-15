package workcal

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestWeekends(t *testing.T) {
	days := Weekends(2026)
	if len(days) != 365 {
		t.Fatalf("len = %d", len(days))
	}
	// 1 января 2026 — четверг, 3 и 4 января — суббота и воскресенье
	if days[0] != '0' || days[2] != '1' || days[3] != '1' || days[4] != '0' {
		t.Fatalf("начало года: %s", days[:7])
	}
	if len(Weekends(2028)) != 366 {
		t.Fatal("високосный год")
	}
}

func TestGetUsesServiceAndFallsBack(t *testing.T) {
	var db *sql.DB
	answer := strings.Repeat("2", 10) + strings.Repeat("0", 355)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "2026") {
			w.Write([]byte(answer))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	s := &Service{DB: &db, URL: func(y int) string { return srv.URL + "/?year=" + strconv.Itoa(y) }, Client: srv.Client()}

	got := s.Get(2026)
	if got.Source != "isdayoff" || got.Days[:10] != strings.Repeat("0", 10) {
		t.Fatalf("сокращённые дни — рабочие: %+v", got.Days[:12])
	}
	fallback := s.Get(2027)
	if fallback.Source != "weekends" || fallback.Days != Weekends(2027) {
		t.Fatalf("запасной календарь: %s", fallback.Source)
	}
}
