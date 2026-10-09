package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pm-dashboard/access"
)

func serve(h http.Handler, method, path string, scope *access.Scope) int {
	req := httptest.NewRequest(method, path, nil)
	if scope != nil {
		req = WithScope(req, scope)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRequireTabs(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := RequireTabs([]TabRule{
		{Path: "/api/tasks", Exact: true, Tab: "tasks"},
		{Path: "/api/payments", Tab: "payments"},
	})(ok)

	tasksOnly := &access.Scope{VisibleTabs: &[]string{"tasks"}}
	paymentsOnly := &access.Scope{VisibleTabs: &[]string{"payments"}}
	admin := &access.Scope{Admin: true, VisibleTabs: &[]string{}}

	tests := []struct {
		name  string
		path  string
		scope *access.Scope
		want  int
	}{
		{"visible tab", "/api/tasks", tasksOnly, http.StatusOK},
		{"hidden tab", "/api/payments/stats", tasksOnly, http.StatusForbidden},
		{"hidden tab, exact path", "/api/payments", tasksOnly, http.StatusForbidden},
		{"task list is the tab", "/api/tasks", paymentsOnly, http.StatusForbidden},
		{"a task card is not the tab", "/api/tasks/15", paymentsOnly, http.StatusOK},
		{"a similar prefix is another path", "/api/payments-export", tasksOnly, http.StatusOK},
		{"administrator sees every tab", "/api/payments/stats", admin, http.StatusOK},
		{"no scope: nothing", "/api/tasks", nil, http.StatusForbidden},
	}
	for _, tt := range tests {
		if got := serve(h, "GET", tt.path, tt.scope); got != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestRequireWrite(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := RequireWrite(ok)

	if got := serve(h, "PUT", "/api/tasks/1", &access.Scope{}); got != http.StatusOK {
		t.Errorf("writer: status = %d", got)
	}
	if got := serve(h, "PUT", "/api/tasks/1", &access.Scope{ReadOnly: true}); got != http.StatusForbidden {
		t.Errorf("read-only user: status = %d", got)
	}
	if got := serve(h, "PUT", "/api/tasks/1", nil); got != http.StatusForbidden {
		t.Errorf("no scope: status = %d", got)
	}
}
