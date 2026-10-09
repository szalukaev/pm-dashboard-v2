package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestPageOf(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int
		clause        string
	}{
		{"", 0, 0, ""},
		{"limit=25&offset=50", 25, 50, " LIMIT 25 OFFSET 50"},
		{"limit=1000", 100, 0, " LIMIT 100 OFFSET 0"},
		{"limit=-5&offset=-1", 0, 0, ""},
		{"offset=30", 0, 0, ""},
		{"limit=abc&offset=10;drop", 0, 0, ""},
	}
	for _, c := range cases {
		limit, offset := pageOf(httptest.NewRequest("GET", "/api/tasks?"+c.query, nil))
		if limit != c.limit || offset != c.offset {
			t.Errorf("%q: got limit %d offset %d, want %d %d", c.query, limit, offset, c.limit, c.offset)
		}
		if got := pageClause(limit, offset); got != c.clause {
			t.Errorf("%q: clause %q, want %q", c.query, got, c.clause)
		}
	}
}

func TestValidPageSize(t *testing.T) {
	for _, n := range []int{10, 25, 50, 100} {
		if !validPageSize(n) {
			t.Errorf("%d must be a valid page size", n)
		}
	}
	for _, n := range []int{0, 5, 26, 1000} {
		if validPageSize(n) {
			t.Errorf("%d must not be a valid page size", n)
		}
	}
}
