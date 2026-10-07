package redmine

import "testing"

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
