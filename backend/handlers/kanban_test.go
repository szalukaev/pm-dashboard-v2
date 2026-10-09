package handlers

import (
	"reflect"
	"strconv"
	"testing"

	"pm-dashboard/datasource/redmine"
)

func columnIDs(columns []KanbanColumn) []string {
	ids := make([]string, len(columns))
	for i, c := range columns {
		ids[i] = c.ID
	}
	return ids
}

func TestFillColumns(t *testing.T) {
	ivan, stranger := 1, 99
	columns := fillColumns(
		[]KanbanColumn{{ID: "1"}, {ID: "2"}, {ID: unassignedColumn}},
		[]KanbanCard{
			{ExternalID: 10, AssignedToID: &ivan},
			{ExternalID: 11},
			{ExternalID: 12, AssignedToID: &stranger}, // nobody's column: dropped
			{ExternalID: 13, AssignedToID: &ivan},
		},
		func(c KanbanCard) string {
			if c.AssignedToID == nil {
				return unassignedColumn
			}
			return strconv.Itoa(*c.AssignedToID)
		},
	)

	if got := len(columns[0].Tasks); got != 2 {
		t.Errorf("column 1 has %d cards, want 2", got)
	}
	// A member without issues still has a column, and its list is not null in JSON
	if columns[1].Tasks == nil || len(columns[1].Tasks) != 0 {
		t.Errorf("empty column tasks = %v, want an empty list", columns[1].Tasks)
	}
	if got := len(columns[2].Tasks); got != 1 {
		t.Errorf("unassigned column has %d cards, want 1", got)
	}
}

func TestOrderColumns(t *testing.T) {
	make4 := func() []KanbanColumn { return []KanbanColumn{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}} }
	tests := []struct {
		name  string
		order []string
		want  []string
	}{
		{"no saved order", nil, []string{"1", "2", "3", "4"}},
		{"full order", []string{"4", "3", "2", "1"}, []string{"4", "3", "2", "1"}},
		{"new columns go after the saved ones", []string{"3", "1"}, []string{"3", "1", "2", "4"}},
		{"unknown and legacy names are ignored", []string{"Иван Иванов", "2", "77"}, []string{"2", "1", "3", "4"}},
	}
	for _, tt := range tests {
		got := columnIDs(orderColumns(make4(), tt.order))
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

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
