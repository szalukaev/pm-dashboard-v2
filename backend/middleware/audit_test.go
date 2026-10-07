package middleware

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskSecrets(t *testing.T) {
	body := `{"username":"bob","password":"p1","nested":{"api_key":"k1","url":"http://x"},"list":[{"basic_password":"p2"}],"DSN":"postgres://u:p3@h/db"}`
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(maskSecrets(v))
	got := string(out)
	for _, secret := range []string{"p1", "k1", "p2", "p3"} {
		if strings.Contains(got, secret) {
			t.Errorf("secret %q leaked into audit state: %s", secret, got)
		}
	}
	for _, kept := range []string{"bob", "http://x"} {
		if !strings.Contains(got, kept) {
			t.Errorf("non-secret %q was lost: %s", kept, got)
		}
	}
}

func TestMethodToAction(t *testing.T) {
	tests := []struct {
		method   string
		expected string
	}{
		{"POST", "created"},
		{"PUT", "updated"},
		{"PATCH", "updated"},
		{"DELETE", "deleted"},
		{"GET", "GET"},
	}
	for _, tt := range tests {
		result := methodToAction(tt.method)
		if result != tt.expected {
			t.Errorf("methodToAction(%q) = %q, want %q", tt.method, result, tt.expected)
		}
	}
}

func TestParseEntity(t *testing.T) {
	tests := []struct {
		path         string
		expectedType string
		expectedID   string
	}{
		{"/api/organizations/5", "organization", "5"},
		{"/api/contracts/12", "contract", "12"},
		{"/api/sprints/3", "sprint", "3"},
		{"/api/tasks/42", "task", "42"},
		{"/api/settings", "settings", ""},
		{"/api/admin/users/2", "admin_users", "2"},
	}
	for _, tt := range tests {
		entityType, entityID := parseEntity(tt.path)
		if entityType != tt.expectedType || entityID != tt.expectedID {
			t.Errorf("parseEntity(%q) = (%q, %q), want (%q, %q)",
				tt.path, entityType, entityID, tt.expectedType, tt.expectedID)
		}
	}
}
