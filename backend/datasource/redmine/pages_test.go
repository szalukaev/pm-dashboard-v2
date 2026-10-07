package redmine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
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
