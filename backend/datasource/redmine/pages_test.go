package redmine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetPagesReturnsAllPagesInOrder(t *testing.T) {
	const total = 1234
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		fmt.Fprintf(w, `{"total_count":%d,"offset":%d}`, total, offset)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "", "")
	pages, err := c.getPages("/issues.json?status_id=*")
	if err != nil {
		t.Fatal(err)
	}
	if want := (total + pageSize - 1) / pageSize; len(pages) != want {
		t.Fatalf("got %d pages, want %d", len(pages), want)
	}
	for i, p := range pages {
		want := fmt.Sprintf(`{"total_count":%d,"offset":%d}`, total, i*pageSize)
		if string(p) != want {
			t.Errorf("page %d = %s, want %s", i, p, want)
		}
	}
}

func TestGetPagesFailsOnPageError(t *testing.T) {
	pageRetryDelay = 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "300" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"total_count":500}`)
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, "key", "", "").getPages("/time_entries.json?project_id=1"); err == nil {
		t.Fatal("expected an error when a page fails")
	}
}

func TestGetPagesRetriesTransientError(t *testing.T) {
	pageRetryDelay = 0
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "100" && calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"total_count":150}`)
	}))
	defer srv.Close()

	pages, err := NewClient(srv.URL, "key", "", "").getPages("/issues.json?status_id=*")
	if err != nil || len(pages) != 2 {
		t.Fatalf("got %d pages, err %v; want 2 pages after a retry", len(pages), err)
	}
}

func TestGetIssuesUpdatedSinceFilter(t *testing.T) {
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		fmt.Fprint(w, `{"issues":[],"total_count":0}`)
	}))
	defer srv.Close()

	since := time.Date(2026, 10, 7, 11, 40, 0, 0, time.UTC)
	if _, err := NewClient(srv.URL, "key", "", "").GetIssues(5, &since); err != nil {
		t.Fatal(err)
	}
	if got := query.Get("updated_on"); got != ">=2026-10-07T11:40:00Z" {
		t.Errorf("updated_on = %q", got)
	}
	if query.Get("project_id") != "5" || query.Get("subproject_id") != "!*" {
		t.Errorf("unexpected project filter: %v", query)
	}
}
