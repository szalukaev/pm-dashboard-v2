package handlers

import (
	"testing"
	"time"
)

func TestReportMoment(t *testing.T) {
	loc := time.FixedZone("YEKT", 5*3600)
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, loc) }
	cases := []struct {
		name   string
		period string
		clock  string
		now    time.Time
		want   time.Time
	}{
		// 13.10.2026 — вторник
		{"каждый день, время уже прошло", ReportDaily, "09:00", at(2026, 10, 13, 10, 0), at(2026, 10, 13, 9, 0)},
		{"каждый день, время ещё не пришло", ReportDaily, "09:00", at(2026, 10, 13, 8, 0), at(2026, 10, 12, 9, 0)},
		{"каждый день, ровно в срок", ReportDaily, "09:00", at(2026, 10, 13, 9, 0), at(2026, 10, 13, 9, 0)},
		{"неделя: во вторник — вчерашний понедельник", ReportWeekly, "09:00", at(2026, 10, 13, 10, 0), at(2026, 10, 12, 9, 0)},
		{"неделя: в понедельник до срока — прошлый понедельник", ReportWeekly, "09:00", at(2026, 10, 12, 8, 0), at(2026, 10, 5, 9, 0)},
		{"неделя: в понедельник после срока", ReportWeekly, "09:00", at(2026, 10, 12, 9, 30), at(2026, 10, 12, 9, 0)},
		{"неделя: в воскресенье — понедельник той же недели", ReportWeekly, "09:00", at(2026, 10, 18, 23, 0), at(2026, 10, 12, 9, 0)},
		{"месяц: в середине месяца — 1-е число", ReportMonthly, "09:00", at(2026, 10, 13, 10, 0), at(2026, 10, 1, 9, 0)},
		{"месяц: 1-го до срока — 1-е прошлого месяца", ReportMonthly, "09:00", at(2026, 10, 1, 8, 0), at(2026, 9, 1, 9, 0)},
		{"месяц: через новый год", ReportMonthly, "09:00", at(2027, 1, 1, 7, 0), at(2026, 12, 1, 9, 0)},
		{"негодное время — как 09:00", ReportDaily, "утром", at(2026, 10, 13, 10, 0), at(2026, 10, 13, 9, 0)},
	}
	for _, c := range cases {
		if got := reportMoment(c.period, c.clock, c.now); !got.Equal(c.want) {
			t.Errorf("%s: получено %s, ожидалось %s", c.name, got, c.want)
		}
	}
}

func TestReportDue(t *testing.T) {
	loc := time.FixedZone("YEKT", 5*3600)
	at := func(d, h int) *time.Time {
		v := time.Date(2026, 10, d, h, 0, 0, 0, loc)
		return &v
	}
	weekly := ReportSettings{Enabled: true, Period: ReportWeekly, Time: "09:00"}

	// Включили во вторник 13-го: сводка за прошедший понедельник не уходит
	if reportDue(weekly, at(13, 10), *at(13, 11)) {
		t.Error("сразу после включения сводка уходить не должна")
	}
	if reportDue(weekly, at(13, 10), *at(18, 23)) {
		t.Error("до следующего понедельника сводка не уходит")
	}
	// В понедельник 19-го в 09:00 — пора
	if !reportDue(weekly, at(13, 10), *at(19, 9)) {
		t.Error("в ближайший понедельник сводка должна уйти")
	}
	// Ушла — до следующего срока больше не уходит
	if reportDue(weekly, at(19, 9), *at(19, 10)) {
		t.Error("сводка уходит один раз за срок")
	}
	// Сервер простоял срок: сводка уходит при первой возможности
	if !reportDue(weekly, at(13, 10), *at(21, 15)) {
		t.Error("пропущенная сводка должна уйти позже")
	}

	off := weekly
	off.Enabled = false
	if reportDue(off, at(1, 10), *at(19, 9)) {
		t.Error("выключенная сводка не уходит")
	}
	if reportDue(weekly, nil, *at(19, 9)) {
		t.Error("без момента включения считать срок не от чего")
	}

	daily := ReportSettings{Enabled: true, Period: ReportDaily, Time: "09:00"}
	if !reportDue(daily, at(12, 9), *at(13, 9)) || reportDue(daily, at(13, 9), *at(13, 18)) {
		t.Error("ежедневная сводка уходит раз в день")
	}
}

func TestReportSettings(t *testing.T) {
	def := defaultReportSettings()
	if def.Enabled || !def.valid() || def.Period != ReportWeekly || len(def.Sections) != 4 {
		t.Errorf("стандартная настройка: %+v", def)
	}

	valid := []ReportSettings{
		{Period: ReportDaily, Time: "00:00"},
		{Period: ReportMonthly, Time: "23:59", Email: "user@example.com", Sections: []string{"stats"}},
	}
	for _, s := range valid {
		if !s.valid() {
			t.Errorf("должно приниматься: %+v", s)
		}
	}
	invalid := []ReportSettings{
		{Period: "hourly", Time: "09:00"},
		{Period: ReportDaily, Time: "25:00"},
		{Period: ReportDaily, Time: "9 утра"},
		{Period: ReportDaily, Time: "09:00", Email: "не адрес"},
		{Period: ReportDaily, Time: "09:00", Sections: []string{"gantt"}},
		{Period: ReportDaily, Time: "09:00", Sections: []string{"stats", "stats", "stats", "stats", "stats"}},
	}
	for _, s := range invalid {
		if s.valid() {
			t.Errorf("не должно приниматься: %+v", s)
		}
	}

	// Сохранённое поверх стандартного; испорченное — стандартное
	got := parseReportSettings([]byte(`{"enabled": true, "period": "daily", "time": "08:30", "sections": ["stats"], "overdue_list": false}`))
	if !got.Enabled || got.Period != ReportDaily || got.Time != "08:30" || got.OverdueList || !got.has("stats") || got.has("deadlines") {
		t.Errorf("сохранённая настройка: %+v", got)
	}
	if got := parseReportSettings([]byte(`{"enabled": true, "period": "hourly"}`)); got.Enabled || got.Period != ReportWeekly {
		t.Errorf("негодная настройка должна заменяться стандартной: %+v", got)
	}
	if got := parseReportSettings(nil); got.Enabled || got.Sections == nil {
		t.Errorf("пустая настройка: %+v", got)
	}
}

func TestReportBar(t *testing.T) {
	if bar := reportBar(5, 10, "#000"); !contains(bar, "width:50%") {
		t.Errorf("половина: %s", bar)
	}
	if bar := reportBar(0, 10, "#000"); !contains(bar, "width:0%") {
		t.Errorf("ноль: %s", bar)
	}
	// Малое, но не нулевое значение должно быть видно
	if bar := reportBar(1, 1000, "#000"); !contains(bar, "width:2%") {
		t.Errorf("малое значение: %s", bar)
	}
	if bar := reportBar(3, 0, "#000"); !contains(bar, "width:2%") {
		t.Errorf("без наибольшего: %s", bar)
	}
}

func contains(text, part string) bool {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
