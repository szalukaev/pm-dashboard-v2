package redmine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueEstimate(t *testing.T) {
	std := 5.0
	cases := []struct {
		name      string
		estimated *float64
		fields    []customFieldValue
		want      *float64
	}{
		{"developer estimate wins", &std, []customFieldValue{{ID: 18, Value: "8"}}, ptr(8)},
		{"comma decimal", &std, []customFieldValue{{ID: 18, Value: "2,5"}}, ptr(2.5)},
		{"empty developer estimate", &std, []customFieldValue{{ID: 18, Value: ""}}, &std},
		{"zero developer estimate", &std, []customFieldValue{{ID: 18, Value: "0"}}, &std},
		{"negative developer estimate", &std, []customFieldValue{{ID: 18, Value: "-1"}}, &std},
		{"field missing", &std, []customFieldValue{{ID: 7, Value: "3"}}, &std},
		{"both empty", nil, nil, nil},
	}
	for _, c := range cases {
		got := issueEstimate(c.estimated, c.fields)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %v, want %v", c.name, deref(got), deref(c.want))
		}
	}
}

func ptr(v float64) *float64 { return &v }

func deref(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func TestEstimateFields(t *testing.T) {
	cases := []struct {
		name  string
		issue string
		hours *float64
		want  string
	}{
		{"developer estimate set", `{"issue":{"custom_fields":[{"id":18,"value":"4"}]}}`, ptr(6.5),
			`{"custom_fields":[{"id":18,"value":"6.5"}]}`},
		{"developer estimate set, clear", `{"issue":{"custom_fields":[{"id":18,"value":"4"}]}}`, nil,
			`{"custom_fields":[{"id":18,"value":""}]}`},
		{"developer estimate empty", `{"issue":{"custom_fields":[{"id":18,"value":""}]}}`, ptr(3),
			`{"estimated_hours":3}`},
		{"developer estimate zero", `{"issue":{"custom_fields":[{"id":18,"value":"0"}]}}`, ptr(3),
			`{"estimated_hours":3}`},
		{"no custom field, clear", `{"issue":{}}`, nil, `{"estimated_hours":""}`},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, c.issue)
		}))
		fields, err := NewClient(srv.URL, "key", "", "").EstimateFields(1, c.hours)
		srv.Close()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, _ := json.Marshal(fields)
		if string(got) != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
