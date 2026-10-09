package redmine

import (
	"reflect"
	"testing"
)

func TestParseIssueDetails(t *testing.T) {
	data := []byte(`{"issue": {
		"id": 100,
		"description": "h1. Text",
		"parent": {"id": 90},
		"children": [{"id": 101, "subject": "sub"}, {"id": 102}],
		"relations": [
			{"id": 1, "issue_id": 100, "issue_to_id": 200, "relation_type": "blocks"},
			{"id": 2, "issue_id": 300, "issue_to_id": 100, "relation_type": "blocks"},
			{"id": 3, "issue_id": 400, "issue_to_id": 100, "relation_type": "relates"},
			{"id": 4, "issue_id": 500, "issue_to_id": 100, "relation_type": "precedes"}
		],
		"journals": [
			{"id": 1, "user": {"name": "Ann"}, "notes": "", "created_on": "2026-01-01T10:00:00Z",
			 "details": [
				{"property": "attr", "name": "status_id", "old_value": "1", "new_value": "2"},
				{"property": "attr", "name": "due_date", "old_value": null, "new_value": "2026-02-01"}
			 ]},
			{"id": 2, "user": {"name": "Bob"}, "notes": "Looks fine", "created_on": "2026-01-02T10:00:00Z", "details": []},
			{"id": 3, "user": {"name": "Bob"}, "notes": "", "created_on": "2026-01-03T10:00:00Z",
			 "details": [
				{"property": "cf", "name": "12", "old_value": "a", "new_value": "b"},
				{"property": "attachment", "name": "55", "old_value": null, "new_value": "shot.png"},
				{"property": "relation", "name": "relates", "old_value": null, "new_value": "400"}
			 ]},
			{"id": 4, "user": {"name": "Ann"}, "notes": "", "created_on": "2026-01-04T10:00:00Z", "details": []}
		]
	}}`)

	d, err := parseIssueDetails(data, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Comments are the entries with a note, as before
	if len(d.Comments) != 1 || d.Comments[0].Text != "Looks fine" || d.Comments[0].Author != "Bob" {
		t.Errorf("comments = %+v", d.Comments)
	}

	// The history keeps everything except the empty entry
	if len(d.History) != 3 {
		t.Fatalf("history has %d entries, want 3: %+v", len(d.History), d.History)
	}
	wantFirst := []FieldChange{{Field: "status", Old: "1", New: "2"}, {Field: "due_date", Old: "", New: "2026-02-01"}}
	if !reflect.DeepEqual(d.History[0].Changes, wantFirst) {
		t.Errorf("first entry changes = %+v", d.History[0].Changes)
	}
	if d.History[1].Text != "Looks fine" || len(d.History[1].Changes) != 0 {
		t.Errorf("second entry = %+v", d.History[1])
	}
	wantThird := []FieldChange{{Field: "cf_12", Old: "a", New: "b"}, {Field: "attachment", New: "shot.png"}, {Field: "relation", New: "#400"}}
	if !reflect.DeepEqual(d.History[2].Changes, wantThird) {
		t.Errorf("third entry changes = %+v", d.History[2].Changes)
	}

	// Relations are told from the point of view of issue 100
	wantRelated := []RelatedIssue{
		{Relation: RelationParent, ID: 90},
		{Relation: RelationChild, ID: 101},
		{Relation: RelationChild, ID: 102},
		{Relation: "blocks", ID: 200},
		{Relation: "blocked", ID: 300},
		{Relation: "relates", ID: 400},
		{Relation: "follows", ID: 500},
	}
	if !reflect.DeepEqual(d.Related, wantRelated) {
		t.Errorf("related = %+v", d.Related)
	}
}

func TestParseIssueDetailsEmpty(t *testing.T) {
	d, err := parseIssueDetails([]byte(`{"issue": {"id": 1}}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	// The card expects lists, never null
	if d.Attachments == nil || d.Comments == nil || d.History == nil || d.Related == nil {
		t.Errorf("lists must not be nil: %+v", d)
	}
}
