package handlers

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	now := time.Date(2026, 10, 13, 15, 30, 0, 0, time.UTC)
	day := func(s string) time.Time {
		d, _ := time.ParseInLocation("2006-01-02", s, time.UTC)
		return d
	}

	for _, q := range []string{"", "period=all"} {
		values, _ := url.ParseQuery(q)
		if p, ok := parsePeriod(values, now); !ok || !p.empty() {
			t.Errorf("%q: got %+v %v, want the whole period", q, p, ok)
		}
	}

	// 14 дней — сегодня и 13 дней до него
	values, _ := url.ParseQuery("period=14")
	p, ok := parsePeriod(values, now)
	if !ok || p.From == nil || !p.From.Equal(day("2026-09-30")) || p.To != nil {
		t.Errorf("period=14: %+v %v", p, ok)
	}

	// Диапазон: обе даты входят, To — начало следующего дня
	values, _ = url.ParseQuery("period=range&from=2026-09-01&to=2026-09-15")
	p, ok = parsePeriod(values, now)
	if !ok || !p.From.Equal(day("2026-09-01")) || !p.To.Equal(day("2026-09-16")) {
		t.Errorf("range: %+v %v", p, ok)
	}

	// Одной даты достаточно
	values, _ = url.ParseQuery("period=range&to=2026-09-15")
	if p, ok := parsePeriod(values, now); !ok || p.From != nil || p.To == nil {
		t.Errorf("range with to only: %+v %v", p, ok)
	}

	for _, bad := range []string{"period=week", "period=0", "period=range&from=01.09.2026", "period=range&from=2026-09-15&to=2026-09-01"} {
		values, _ := url.ParseQuery(bad)
		if _, ok := parsePeriod(values, now); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestPeriodClosedCond(t *testing.T) {
	args := []interface{}{"x"}
	if cond := (Period{}).closedCond(&args); cond != "" || len(args) != 1 {
		t.Errorf("whole period: cond %q, args %v", cond, args)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	cond := Period{From: &from, To: &to}.closedCond(&args)
	// Параметры нумеруются после уже имеющихся
	if !strings.Contains(cond, ">= $2") || !strings.Contains(cond, "< $3") || len(args) != 3 {
		t.Errorf("cond %q, args %v", cond, args)
	}
}

func TestVelocityWeeksOf(t *testing.T) {
	now := time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC) // вторник
	if got := len(velocityWeeksOf(Period{}, now)); got != velocityWeeks {
		t.Errorf("whole period: %d weeks, want %d", got, velocityWeeks)
	}
	// 14 дней с 30 сентября: недели с 28.09, 05.10 и 12.10
	from := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	weeks := velocityWeeksOf(Period{From: &from}, now)
	if len(weeks) != 3 || weeks[0].Format("2006-01-02") != "2026-09-28" || weeks[2].Format("2006-01-02") != "2026-10-12" {
		t.Errorf("14 days: %v", weeks)
	}
	// Длинный период обрезается до последних недель
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := len(velocityWeeksOf(Period{From: &old}, now)); got != maxVelocityWeeks {
		t.Errorf("long period: %d weeks, want %d", got, maxVelocityWeeks)
	}
	// Диапазон в прошлом кончается неделей своего последнего дня
	to := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	past := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	weeks = velocityWeeksOf(Period{From: &past, To: &to}, now)
	if len(weeks) != 3 || weeks[len(weeks)-1].Format("2006-01-02") != "2026-09-14" {
		t.Errorf("past range: %v", weeks)
	}
}

func TestPeriodUpdatedCond(t *testing.T) {
	args := []interface{}{"x"}
	if cond := (Period{}).updatedCond(&args); cond != "" || len(args) != 1 {
		t.Errorf("без периода: cond %q, args %v", cond, args)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cond := Period{From: &from}.updatedCond(&args)
	// Только нижняя граница; закрытые задачи отбираются так же, как открытые
	if cond != " AND updated_on IS NOT NULL AND updated_on >= $2" || len(args) != 2 {
		t.Errorf("cond %q, args %v", cond, args)
	}
}
