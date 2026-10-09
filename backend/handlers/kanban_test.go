package handlers

import (
	"testing"

	"pm-dashboard/datasource/redmine"
)

func TestMoveApplied(t *testing.T) {
	ivan := 42
	tests := []struct {
		name   string
		state  redmine.IssueState
		fields map[string]interface{}
		want   bool
	}{
		{"status changed", redmine.IssueState{StatusID: 3}, map[string]interface{}{"status_id": 3}, true},
		{"status ignored by workflow", redmine.IssueState{StatusID: 1}, map[string]interface{}{"status_id": 3}, false},
		{"reassigned", redmine.IssueState{AssignedToID: &ivan}, map[string]interface{}{"assigned_to_id": 42}, true},
		{"assignee not accepted", redmine.IssueState{AssignedToID: &ivan}, map[string]interface{}{"assigned_to_id": 7}, false},
		{"assignee still empty", redmine.IssueState{}, map[string]interface{}{"assigned_to_id": 42}, false},
		{"unassigned", redmine.IssueState{}, map[string]interface{}{"assigned_to_id": ""}, true},
		{"assignee not cleared", redmine.IssueState{AssignedToID: &ivan}, map[string]interface{}{"assigned_to_id": ""}, false},
	}
	for _, tt := range tests {
		if got := moveApplied(&tt.state, tt.fields); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
