package handlers

import (
	"testing"
	"time"
)

func TestDeadlineBucket(t *testing.T) {
	today := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		due, want string
	}{
		{"2026-10-08", "overdue"},
		{"2026-10-09", "today"},
		{"2026-10-09T00:00:00Z", "today"},
		{"2026-10-10", "three_days"},
		{"2026-10-12", "three_days"},
		{"2026-10-13", "week"},
		{"2026-10-16", "week"},
		{"2026-10-17", ""},
	}
	for _, tt := range tests {
		if got := deadlineBucket(tt.due, today); got != tt.want {
			t.Errorf("deadlineBucket(%q) = %q, want %q", tt.due, got, tt.want)
		}
	}
}
