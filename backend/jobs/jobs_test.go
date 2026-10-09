package jobs

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, loc) }

	cases := []struct {
		name      string
		now       time.Time
		timeOfDay string
		want      time.Time
	}{
		{"later today", at(9, 1, 30), "02:00", at(9, 2, 0)},
		{"the time has passed: tomorrow", at(9, 2, 1), "02:00", at(10, 2, 0)},
		{"exactly at the time: the next one is tomorrow", at(9, 2, 0), "02:00", at(10, 2, 0)},
		{"over the end of the month", time.Date(2026, 10, 31, 23, 0, 0, 0, loc), "02:00", time.Date(2026, 11, 1, 2, 0, 0, 0, loc)},
		{"not a time of day: the default", at(9, 1, 0), "soon", at(9, 2, 0)},
	}
	for _, c := range cases {
		if got := NextRun(c.now, c.timeOfDay); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNextAttemptAfterFailure(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	failedAt := time.Date(2026, 10, 9, 2, 0, 5, 0, loc)

	// A failed run is tried again in an hour, not a day later
	retry := failedAt.Add(retryAfterFailure)
	if got := nextAttempt(failedAt, "02:00", retry); !got.Equal(retry) {
		t.Errorf("after a failure: next attempt at %v, want %v", got, retry)
	}
	// Without a failure the next run is the scheduled one
	want := time.Date(2026, 10, 10, 2, 0, 0, 0, loc)
	if got := nextAttempt(failedAt, "02:00", time.Time{}); !got.Equal(want) {
		t.Errorf("without a failure: next attempt at %v, want %v", got, want)
	}
	// A retry that would come after the scheduled run does not delay it
	late := time.Date(2026, 10, 9, 1, 30, 0, 0, loc)
	if got := nextAttempt(late, "02:00", late.Add(retryAfterFailure)); !got.Equal(time.Date(2026, 10, 9, 2, 0, 0, 0, loc)) {
		t.Errorf("a late retry: next attempt at %v, want the scheduled time", got)
	}
}

func TestMemoryStore(t *testing.T) {
	store := NewStore("") // no Redis: in memory
	if _, ok := store.Load(CleanupKey); ok {
		t.Error("nothing is known about a worker that never ran")
	}
	started := time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC)
	store.Save(CleanupKey, Status{State: StateIdle, LastStart: &started, DurationMs: 1200, Processed: 5123})
	got, ok := store.Load(CleanupKey)
	if !ok || got.Processed != 5123 || got.DurationMs != 1200 || !got.LastStart.Equal(started) {
		t.Errorf("loaded %+v", got)
	}
}

func TestCleanerWithoutDatabase(t *testing.T) {
	cleaner := NewCleaner(nil, &Settings{}, NewStore(""))
	if _, err := cleaner.RunNow(); err != ErrNoDatabase {
		t.Errorf("without a database: err = %v, want ErrNoDatabase", err)
	}
	// Nothing ran, so nothing is reported as running or failed
	if status := cleaner.Status(); status.State != StateIdle || status.LastStart != nil {
		t.Errorf("status = %+v", status)
	}
}

func TestSettingsDefaultsAndLimits(t *testing.T) {
	settings := &Settings{} // no database: the defaults
	if settings.RetentionDays() != DefaultRetentionDays || !settings.CleanupEnabled() || settings.CleanupTime() != DefaultCleanupTime {
		t.Errorf("defaults: %d days, enabled %v, at %s", settings.RetentionDays(), settings.CleanupEnabled(), settings.CleanupTime())
	}
	for _, days := range []int{6, 0, -1, 366} {
		if ValidRetention(days) {
			t.Errorf("%d days must be refused", days)
		}
	}
	for _, days := range []int{7, 90, 365} {
		if !ValidRetention(days) {
			t.Errorf("%d days must be accepted", days)
		}
	}
	if err := settings.SetRetentionDays(3); err == nil {
		t.Error("a retention below the minimum must be refused")
	}
	if ValidTimeOfDay("24:00") || ValidTimeOfDay("") || !ValidTimeOfDay("02:00") {
		t.Error("time of day is checked wrongly")
	}
}
