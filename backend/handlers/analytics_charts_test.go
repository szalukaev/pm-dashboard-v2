package handlers

import (
	"testing"
	"time"
)

func TestWeekStart(t *testing.T) {
	loc := time.FixedZone("YEKT", 5*3600)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"a Monday is its own start", time.Date(2026, 10, 12, 0, 0, 0, 0, loc), "2026-10-12"},
		{"the middle of a week", time.Date(2026, 10, 14, 15, 30, 0, 0, loc), "2026-10-12"},
		{"a Sunday belongs to the week that began on Monday", time.Date(2026, 10, 18, 23, 59, 59, 0, loc), "2026-10-12"},
		{"the next Monday starts a new week", time.Date(2026, 10, 19, 0, 0, 0, 0, loc), "2026-10-19"},
		{"a week across the turn of a month", time.Date(2026, 10, 1, 9, 0, 0, 0, loc), "2026-09-28"},
		{"a week across the turn of a year", time.Date(2027, 1, 2, 9, 0, 0, 0, loc), "2026-12-28"},
	}
	for _, c := range cases {
		got := weekStart(c.at)
		if got.Format("2006-01-02") != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got.Format("2006-01-02"), c.want)
		}
		if got.Weekday() != time.Monday || got.Hour() != 0 || got.Location() != loc {
			t.Errorf("%s: %v is not a midnight of a Monday in the zone of the date", c.name, got)
		}
	}
}

// The moment a task was closed counts in the server's zone: late Sunday in
// UTC is already Monday in Yekaterinburg.
func TestWeekStartFollowsZone(t *testing.T) {
	closed := time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	if got := weekStart(closed).Format("2006-01-02"); got != "2026-10-05" {
		t.Errorf("in UTC: got %s, want 2026-10-05", got)
	}
	local := closed.In(time.FixedZone("YEKT", 5*3600))
	if got := weekStart(local).Format("2006-01-02"); got != "2026-10-12" {
		t.Errorf("in Yekaterinburg: got %s, want 2026-10-12", got)
	}
}

func TestLastWeeks(t *testing.T) {
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	weeks := lastWeeks(now, 12)
	if len(weeks) != 12 {
		t.Fatalf("got %d weeks, want 12", len(weeks))
	}
	if got := weeks[11].Format("2006-01-02"); got != "2026-10-12" {
		t.Errorf("the last week must be the current one, got %s", got)
	}
	if got := weeks[0].Format("2006-01-02"); got != "2026-07-27" {
		t.Errorf("the first week: got %s, want 2026-07-27", got)
	}
	for i, w := range weeks {
		if w.Weekday() != time.Monday {
			t.Errorf("week %d starts on %s", i, w.Weekday())
		}
		if i > 0 && w.Sub(weeks[i-1]) != 7*24*time.Hour {
			t.Errorf("weeks %d and %d are not a week apart", i-1, i)
		}
	}
}

func TestBugPercent(t *testing.T) {
	cases := []struct {
		bug, spent, want float64
	}{
		{5, 20, 25},
		{1, 3, 33.3},
		{0, 10, 0},
		{4, 0, 0},
		{10, 10, 100},
	}
	for _, c := range cases {
		if got := bugPercent(c.bug, c.spent); got != c.want {
			t.Errorf("bugPercent(%v, %v) = %v, want %v", c.bug, c.spent, got, c.want)
		}
	}
}

func TestProjectRoot(t *testing.T) {
	// 1 ← 2 ← 3, 4 stands alone, 5 points to a project that is not known,
	// 6 and 7 point to each other
	parents := map[int]int{1: 0, 2: 1, 3: 2, 4: 0, 5: 99, 6: 7, 7: 6}
	cases := map[int]int{
		1:  1, // a top project is its own root
		2:  1,
		3:  1, // through every level
		4:  4,
		5:  5,  // an unknown parent cannot be the root
		42: 42, // an unknown project
	}
	for id, want := range cases {
		if got := projectRoot(id, parents); got != want {
			t.Errorf("projectRoot(%d) = %d, want %d", id, got, want)
		}
	}
	// A loop must end, wherever it ends
	if got := projectRoot(6, parents); got != 6 && got != 7 {
		t.Errorf("a loop: got %d", got)
	}
}
