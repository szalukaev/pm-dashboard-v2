package redmine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateIssueReportsRedmineErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"errors":["Assignee is invalid","Due date must be greater than start date"]}`))
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "key", "", "").UpdateIssue(1, map[string]interface{}{"assigned_to_id": 7})
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"422", "Assignee is invalid", "Due date must be greater than start date"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestGetIssueState(t *testing.T) {
	body := `{"issue":{"id":1,"status":{"id":3,"name":"Review"},"assigned_to":{"id":42,"name":"Ivan"}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "key", "", "")

	state, err := c.GetIssueState(1)
	if err != nil {
		t.Fatal(err)
	}
	if state.StatusID != 3 || state.AssignedToID == nil || *state.AssignedToID != 42 {
		t.Errorf("state = %+v, want status 3 and assignee 42", state)
	}

	body = `{"issue":{"id":1,"status":{"id":1,"name":"New"}}}`
	state, err = c.GetIssueState(1)
	if err != nil {
		t.Fatal(err)
	}
	if state.StatusID != 1 || state.AssignedToID != nil {
		t.Errorf("state = %+v, want status 1 and no assignee", state)
	}
}
