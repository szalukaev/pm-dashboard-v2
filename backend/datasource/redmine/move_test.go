package redmine

import (
	"errors"
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

func TestGetCurrentUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" || r.Header.Get("X-Redmine-API-Key") != "personal-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"user":{"id":42,"login":"ivanov","firstname":"Иван","lastname":"Иванов"}}`))
	}))
	defer srv.Close()

	user, err := NewClient(srv.URL, "personal-key", "", "").GetCurrentUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.ExternalID != 42 || user.Login != "ivanov" || user.Name != "Иван Иванов" {
		t.Errorf("user = %+v", user)
	}

	_, err = NewClient(srv.URL, "wrong-key", "", "").GetCurrentUser()
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("wrong key: err = %v, want ErrUnauthorized", err)
	}
}

func TestGetCurrentProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include") != "memberships" {
			w.Write([]byte(`{"user":{"id":42}}`))
			return
		}
		w.Write([]byte(`{"user":{"id":42,"memberships":[
			{"id":1,"project":{"id":310,"name":"A"},"roles":[{"id":4,"name":"Developer"}]},
			{"id":2,"project":{"id":285,"name":"B"},"roles":[{"id":3,"name":"Manager"}]}
		]}}`))
	}))
	defer srv.Close()

	projects, err := NewClient(srv.URL, "key", "", "").GetCurrentProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0] != 310 || projects[1] != 285 {
		t.Errorf("projects = %v, want [310 285]", projects)
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
