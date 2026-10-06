package utils

import (
	"testing"
)

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-1, "-1"},
		{100, "100"},
	}
	for _, tt := range tests {
		result := Itoa(tt.input)
		if result != tt.expected {
			t.Errorf("Itoa(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		parts    []string
		sep      string
		expected string
	}{
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"a"}, ", ", "a"},
		{[]string{}, ", ", ""},
		{[]string{"a", "b"}, " AND ", "a AND b"},
	}
	for _, tt := range tests {
		result := JoinStrings(tt.parts, tt.sep)
		if result != tt.expected {
			t.Errorf("JoinStrings(%v, %q) = %q, want %q", tt.parts, tt.sep, result, tt.expected)
		}
	}
}

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{0.0, 0.0},
		{100.999, 101.0},
	}
	for _, tt := range tests {
		result := Round2(tt.input)
		if result != tt.expected {
			t.Errorf("Round2(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}
