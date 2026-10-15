package handlers

import "testing"

func TestParseHours(t *testing.T) {
	tests := []struct {
		name   string
		in     interface{}
		want   float64
		isNil  bool
		wantOK bool
	}{
		{"number", 2.5, 2.5, false, true},
		{"string with dot", "1.5", 1.5, false, true},
		{"string with comma", "1,5", 1.5, false, true},
		{"empty string clears", "", 0, true, true},
		{"null clears", nil, 0, true, true},
		{"garbage", "abc", 0, true, false},
		{"negative", -1.0, 0, true, false},
		{"wrong type", true, 0, true, false},
	}
	for _, tt := range tests {
		got, ok := parseHours(tt.in)
		if ok != tt.wantOK {
			t.Errorf("%s: ok = %v, want %v", tt.name, ok, tt.wantOK)
			continue
		}
		if tt.isNil {
			if got != nil {
				t.Errorf("%s: expected nil, got %v", tt.name, *got)
			}
			continue
		}
		if got == nil || *got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAttentionCond(t *testing.T) {
	if got := attentionCond(""); got != "" {
		t.Errorf("empty list: got %q, want no condition", got)
	}
	if got := attentionCond("bogus, ;DROP"); got != "" {
		t.Errorf("unknown values: got %q, want no condition", got)
	}
	got := attentionCond("unassigned, overdue")
	want := "(" + attentionSQL("overdue") + " OR " + attentionSQL("unassigned") + ")"
	if got != want {
		t.Errorf("two chips: got %q, want %q", got, want)
	}
	for _, key := range attentionKeys {
		if attentionSQL(key) == "" {
			t.Errorf("key %q has no condition", key)
		}
	}
}
