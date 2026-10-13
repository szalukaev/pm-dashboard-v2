package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestDebugFilter(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/debug-logs?user_id=5&method=put&path=tasks&from=2026-10-01&to=2026-10-13", nil)
	where, args, ok := debugFilter(r)
	if !ok {
		t.Fatal("valid filter rejected")
	}
	want := "1=1 AND d.user_id = $1 AND d.method = $2 AND d.path ILIKE '%' || $3 || '%' AND d.occurred_at >= $4::date AND d.occurred_at < $5::date + 1"
	if where != want {
		t.Errorf("where = %q\nwant    %q", where, want)
	}
	if len(args) != 5 || args[0] != 5 || args[1] != "PUT" || args[2] != "tasks" {
		t.Errorf("args = %v", args)
	}

	for _, bad := range []string{"user_id=abc", "user_id=-1", "from=13.10.2026", "to=yesterday"} {
		if _, _, ok := debugFilter(httptest.NewRequest("GET", "/api/admin/debug-logs?"+bad, nil)); ok {
			t.Errorf("filter %q accepted", bad)
		}
	}

	if where, args, ok := debugFilter(httptest.NewRequest("GET", "/api/admin/debug-logs", nil)); !ok || where != "1=1" || len(args) != 0 {
		t.Errorf("empty filter: %q %v %v", where, args, ok)
	}
}
