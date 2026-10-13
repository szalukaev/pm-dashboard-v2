package handlers

import "testing"

func TestBuildChecklist(t *testing.T) {
	items := buildChecklist(checklistFacts{SourceOK: true, Projects: true, Schedule: true})
	want := []checklistItem{
		{Key: "source", Done: true, Required: true},
		{Key: "statuses", Done: false, Required: true},
		{Key: "projects", Done: true, Required: true},
		{Key: "users", Done: false, Required: true},
		{Key: "rights", Done: false, Required: false},
		{Key: "schedule", Done: true, Required: false},
	}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, items[i], want[i])
		}
	}
}
