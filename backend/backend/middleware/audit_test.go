package middleware

import (
	"testing"
)

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
