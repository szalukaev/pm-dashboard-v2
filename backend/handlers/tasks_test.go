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
