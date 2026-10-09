package handlers

import (
	"encoding/json"
	"testing"
)

func TestValidTableColumns(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"one table", `{"tasks_table": {"visible": ["subject"], "order": ["external_id", "subject"]}}`, true},
		{"two tables", `{"a": {"visible": [], "order": []}, "b": {"visible": ["x"], "order": ["x"]}}`, true},
		{"nothing to save", `{}`, true},
		{"not an object", `["tasks_table"]`, false},
		{"setup is not an object", `{"tasks_table": ["subject"]}`, false},
		{"order is missing", `{"tasks_table": {"visible": ["subject"]}}`, false},
		{"an extra field", `{"tasks_table": {"visible": [], "order": [], "width": 5}}`, false},
		{"a key is not a string", `{"tasks_table": {"visible": [1], "order": []}}`, false},
		{"an empty key", `{"tasks_table": {"visible": [""], "order": []}}`, false},
		{"an empty table name", `{"": {"visible": [], "order": []}}`, false},
	}
	for _, c := range cases {
		var v interface{}
		if err := json.Unmarshal([]byte(c.body), &v); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := validTableColumns(v); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
