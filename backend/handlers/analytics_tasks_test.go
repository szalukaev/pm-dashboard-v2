package handlers

import (
	"strings"
	"testing"
)

func TestAnalyticsTaskCond(t *testing.T) {
	// Every figure that can be clicked has its list of tasks
	for _, metric := range []string{"open", "testing", "closed", "overdue", "bugs", "high_priority", "no_estimate"} {
		cond, ok := analyticsTaskCond(metric, "$3")
		if !ok || cond == "" {
			t.Errorf("%s: no condition", metric)
		}
	}

	for _, metric := range []string{"", "completion", "active_projects", "open; DROP TABLE issues"} {
		if cond, ok := analyticsTaskCond(metric, "$3"); ok || cond != "" {
			t.Errorf("%q must not be accepted, got %q", metric, cond)
		}
	}

	// Overdue is counted against today's date over the tasks still in work
	overdue, _ := analyticsTaskCond("overdue", "$3")
	if !strings.Contains(overdue, "due_date < $3") || !strings.Contains(overdue, "'open','testing'") {
		t.Errorf("overdue: %s", overdue)
	}
	// A task without an estimate counts only while it is open, as on the card
	noEstimate, _ := analyticsTaskCond("no_estimate", "$1")
	if !strings.Contains(noEstimate, "estimated_hours IS NULL OR estimated_hours = 0") || strings.Contains(noEstimate, "testing") {
		t.Errorf("no_estimate: %s", noEstimate)
	}
}
