package handlers

import (
	"encoding/json"
	"testing"
)

func TestValidDashboardLayout(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"placed and hidden widgets", `{"widgets": [{"id": "stats", "cols": 12}, {"id": "team_load", "cols": 6, "collapsed": true}], "hidden_widgets": ["deadlines"]}`, true},
		{"reset to the standard layout", `{}`, true},
		{"every widget hidden", `{"widgets": [], "hidden_widgets": ["stats"]}`, true},
		{"every allowed width", `{"widgets": [{"id": "a", "cols": 4}, {"id": "b", "cols": 6}, {"id": "c", "cols": 8}, {"id": "d", "cols": 12}]}`, true},
		{"not an object", `[{"id": "stats", "cols": 12}]`, false},
		{"widgets is not a list", `{"widgets": {"id": "stats", "cols": 12}}`, false},
		{"a width outside of the grid steps", `{"widgets": [{"id": "stats", "cols": 5}]}`, false},
		{"a width is not a number", `{"widgets": [{"id": "stats", "cols": "12"}]}`, false},
		{"a width is missing", `{"widgets": [{"id": "stats"}]}`, false},
		{"an empty id", `{"widgets": [{"id": "", "cols": 12}]}`, false},
		{"an id is not a string", `{"widgets": [{"id": 1, "cols": 12}]}`, false},
		{"collapsed is not a flag", `{"widgets": [{"id": "stats", "cols": 12, "collapsed": 1}]}`, false},
		{"an extra field of a widget", `{"widgets": [{"id": "stats", "cols": 12, "row": 1}]}`, false},
		{"an extra field of the layout", `{"widgets": [], "theme": "dark"}`, false},
		{"a widget placed twice", `{"widgets": [{"id": "stats", "cols": 12}, {"id": "stats", "cols": 6}]}`, false},
		{"a widget both placed and hidden", `{"widgets": [{"id": "stats", "cols": 12}], "hidden_widgets": ["stats"]}`, false},
		{"a hidden id is not a string", `{"hidden_widgets": [1]}`, false},
	}
	for _, c := range cases {
		var v interface{}
		if err := json.Unmarshal([]byte(c.body), &v); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := validDashboardLayout(v); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
