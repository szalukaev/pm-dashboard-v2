package handlers

import (
	"net/http"
	"net/http/httptest"
	"database/sql"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "" {
		t.Fatalf("empty secret should stay empty, got %q", got)
	}
	if got := maskSecret("real-key"); got != secretMask {
		t.Fatalf("set secret must be masked, got %q", got)
	}
}

func TestIsSecretMask(t *testing.T) {
	if !isSecretMask(secretMask) {
		t.Fatal("mask placeholder must be recognised")
	}
	if isSecretMask("real-key") || isSecretMask("") {
		t.Fatal("real values must not be treated as the mask")
	}
}

func TestMaskDSNPassword(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"postgres://user:pass@host:5432/db?sslmode=disable", "postgres://user:****@host:5432/db?sslmode=disable"},
		{"postgres://user@host:5432/db", "postgres://user@host:5432/db"}, // no password
		{"host=localhost port=5432", "host=localhost port=5432"},         // keyword DSN — left as-is
	}
	for _, c := range cases {
		if got := maskDSNPassword(c.in); got != c.want {
			t.Errorf("maskDSNPassword(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNilDBReturns503 — handlers must not panic when PostgreSQL was never connected.
func TestNilDBReturns503(t *testing.T) {
	var nilDB *sql.DB
	paymentsH := &PaymentsHandler{DB: &nilDB}
	authH := &AuthHandler{DB: &nilDB}

	cases := []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"ListOrganizations", paymentsH.ListOrganizations},
		{"ListContracts", paymentsH.ListContracts},
		{"GetStats", paymentsH.GetStats},
		{"Login", authH.Login},
		{"Me", authH.Me},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rr := httptest.NewRecorder()
		c.fn(rr, req) // must not panic
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", c.name, rr.Code)
		}
	}
}
