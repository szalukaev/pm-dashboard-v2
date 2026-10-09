package snapshots

import (
	"testing"
	"time"
)

func TestCutoff(t *testing.T) {
	today := time.Date(2026, 10, 9, 15, 30, 0, 0, time.UTC)
	cases := []struct {
		keep int
		want string
	}{
		{7, "2026-10-02"},
		{90, "2026-07-11"},
		{365, "2025-10-09"},
	}
	for _, c := range cases {
		if got := Cutoff(today, c.keep); got != c.want {
			t.Errorf("keep %d days: the first kept day is %s, want %s", c.keep, got, c.want)
		}
	}
}

func TestCleanupRefusesNoRetention(t *testing.T) {
	// A retention of zero days would delete today's snapshot as well
	if _, err := Cleanup(nil, time.Now(), 0); err == nil {
		t.Error("a retention of 0 days must be refused before touching the database")
	}
}
