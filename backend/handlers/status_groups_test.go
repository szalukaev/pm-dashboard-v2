package handlers

import "testing"

func TestStatusConditions(t *testing.T) {
	sub := "SELECT external_id FROM statuses WHERE data_source = 'redmine' AND group_name IN "
	if got, want := statusIn("status_id", GroupOpen, GroupTesting), "status_id IN ("+sub+"('open','testing'))"; got != want {
		t.Errorf("statusIn = %q, want %q", got, want)
	}
	if got, want := statusNotIn("i.status_id", GroupClosed), "i.status_id NOT IN ("+sub+"('closed'))"; got != want {
		t.Errorf("statusNotIn = %q, want %q", got, want)
	}
}

func TestSprintProgressUsesStatusGroups(t *testing.T) {
	// Status names say nothing: only the configured group counts.
	s := Sprint{Tasks: []SprintTask{
		{StatusName: "Closed", statusGroup: GroupOpen},
		{StatusName: "Готово", statusGroup: GroupClosed},
		{StatusName: "На проверке", statusGroup: GroupTesting},
		{StatusName: "В работе", statusGroup: GroupOpen},
	}}
	s.calculateProgress()

	if s.ClosedCount != 1 || s.TestingCount != 1 {
		t.Errorf("closed = %d, testing = %d, want 1 and 1", s.ClosedCount, s.TestingCount)
	}
	// (1 closed + 0.5 * 1 testing) / 4 tasks
	if s.Progress != 37.5 {
		t.Errorf("progress = %v, want 37.5", s.Progress)
	}
}
