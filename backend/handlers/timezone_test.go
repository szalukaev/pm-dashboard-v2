package handlers

import "testing"

func TestValidTimezone(t *testing.T) {
	cases := map[string]bool{
		"":                   true,
		"UTC":                true,
		"Asia/Yekaterinburg": true,
		"Europe/Moscow":      true,
		"Local":              false,
		"Mars/Olympus":       false,
		"../etc/passwd":      false,
		"/etc/localtime":     false,
		"Asia/Yekaterinburg_and_a_name_that_is_far_too_long_to_fit_the_column": false,
	}
	for name, want := range cases {
		if got := validTimezone(name); got != want {
			t.Errorf("validTimezone(%q) = %v, want %v", name, got, want)
		}
	}
}
