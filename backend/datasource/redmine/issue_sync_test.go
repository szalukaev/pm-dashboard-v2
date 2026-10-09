package redmine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// memStore stands in for the database.
type memStore struct {
	mu      sync.Mutex
	states  map[int]projectState
	issues  map[int]int // issue → project
	entries map[int]int // time entry → issue
	deleted []int
}

func newMemStore() *memStore {
	return &memStore{states: map[int]projectState{}, issues: map[int]int{}, entries: map[int]int{}}
}

func (m *memStore) loadStates() (map[int]projectState, error) {
	copied := map[int]projectState{}
	for k, v := range m.states {
		copied[k] = v
	}
	return copied, nil
}

func (m *memStore) saveState(pid int, st projectState) error {
	m.states[pid] = st
	return nil
}

func (m *memStore) storeTimeEntries(pid int, entries []TimeEntry, replace bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if replace {
		for id, issue := range m.entries {
			if m.issues[issue] == pid {
				delete(m.entries, id)
			}
		}
	}
	for _, e := range entries {
		m.entries[e.ID] = e.IssueID
	}
	return nil
}

func (m *memStore) upsertIssues(batch []Issue) error {
	for _, iss := range batch {
		m.issues[iss.ExternalID] = iss.ProjectID
	}
	return nil
}

func (m *memStore) updateIssueHours([]int64) error { return nil }

func (m *memStore) countIssues(pid int) (int, error) {
	n := 0
	for _, project := range m.issues {
		if project == pid {
			n++
		}
	}
	return n, nil
}

func (m *memStore) deleteMissing(pid int, keep []int) ([]int, error) {
	kept := map[int]bool{}
	for _, id := range keep {
		kept[id] = true
	}
	var gone []int
	for id, project := range m.issues {
		if project == pid && !kept[id] {
			gone = append(gone, id)
		}
	}
	sort.Ints(gone)
	return gone, m.deleteIssues(gone)
}

func (m *memStore) deleteIssues(ids []int) error {
	for _, id := range ids {
		delete(m.issues, id)
		for entry, issue := range m.entries {
			if issue == id {
				delete(m.entries, entry)
			}
		}
		m.deleted = append(m.deleted, id)
	}
	return nil
}

func (m *memStore) cleanupUnselected([]int) error { return nil }

func (m *memStore) issueIDs() []int {
	ids := make([]int, 0, len(m.issues))
	for id := range m.issues {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// fakeRedmine serves issues and time entries per project and records the
// requests it got.
type fakeRedmine struct {
	mu sync.Mutex
	// project → issue numbers
	issues map[int][]int
	// project → time entries as "id:issue"
	entries map[int][]string
	// projects whose time entries the key may not read
	forbidden map[int]bool
	// projects whose issues fail with 500
	broken   map[int]bool
	requests []string
}

func (f *fakeRedmine) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		q := r.URL.Query()
		pid, _ := strconv.Atoi(q.Get("project_id"))
		f.requests = append(f.requests, fmt.Sprintf("%s project=%d", r.URL.Path, pid))

		projects := []int{pid}
		if pid == 0 {
			projects = projects[:0]
			for p := range f.issues {
				projects = append(projects, p)
			}
			for p := range f.entries {
				if _, ok := f.issues[p]; !ok {
					projects = append(projects, p)
				}
			}
			sort.Ints(projects)
		}

		var items []string
		switch r.URL.Path {
		case "/issues.json":
			if f.broken[pid] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for _, p := range projects {
				for _, id := range f.issues[p] {
					items = append(items, fmt.Sprintf(`{"id":%d,"project":{"id":%d}}`, id, p))
				}
			}
			fmt.Fprintf(w, `{"total_count":%d,"issues":[%s]}`, len(items), strings.Join(items, ","))
		case "/time_entries.json":
			if f.forbidden[pid] {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			for _, p := range projects {
				if f.forbidden[p] {
					continue
				}
				for _, e := range f.entries[p] {
					parts := strings.Split(e, ":")
					items = append(items, fmt.Sprintf(`{"id":%s,"issue":{"id":%s},"project":{"id":%d},"hours":1}`, parts[0], parts[1], p))
				}
			}
			fmt.Fprintf(w, `{"total_count":%d,"time_entries":[%s]}`, len(items), strings.Join(items, ","))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func (f *fakeRedmine) takeRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	got := f.requests
	f.requests = nil
	sort.Strings(got)
	return got
}

func TestSyncProjectIssues(t *testing.T) {
	pageRetryDelay = 0
	redmine := &fakeRedmine{
		issues:    map[int][]int{1: {11, 12}, 2: {21}, 3: {31}},
		entries:   map[int][]string{1: {"100:11"}, 3: {"300:31"}},
		forbidden: map[int]bool{2: true},
	}
	srv := redmine.server(t)
	defer srv.Close()
	syncer := NewSyncer(NewClient(srv.URL, "key", "", ""))
	store := newMemStore()
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	// First run: nothing is known, every selected project is read completely.
	// Project 2 denies its time entries, which must not count as a failure.
	if err := syncer.syncProjectIssues(store, []int{1, 2}, start); err != nil {
		t.Fatal(err)
	}
	if got := store.issueIDs(); !reflect.DeepEqual(got, []int{11, 12, 21}) {
		t.Errorf("issues after the first run = %v", got)
	}
	for _, pid := range []int{1, 2} {
		if st, ok := store.states[pid]; !ok || !st.lastSync.Equal(start) || !st.lastFull.Equal(start) {
			t.Errorf("project %d: state = %+v, want both marks at the start of the run", pid, store.states[pid])
		}
	}
	want := []string{
		"/issues.json project=1", "/issues.json project=2",
		"/time_entries.json project=1", "/time_entries.json project=2",
	}
	if got := redmine.takeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests of the first run = %v", got)
	}

	// Second run a few minutes later: two requests for everything. An issue
	// of project 3, which is not selected, is not taken.
	redmine.issues[1] = append(redmine.issues[1], 13)
	second := start.Add(5 * time.Minute)
	if err := syncer.syncProjectIssues(store, []int{1, 2}, second); err != nil {
		t.Fatal(err)
	}
	if got := redmine.takeRequests(); !reflect.DeepEqual(got, []string{"/issues.json project=0", "/time_entries.json project=0"}) {
		t.Errorf("requests of the second run = %v", got)
	}
	if got := store.issueIDs(); !reflect.DeepEqual(got, []int{11, 12, 13, 21}) {
		t.Errorf("issues after the second run = %v", got)
	}
	if st := store.states[1]; !st.lastSync.Equal(second) || !st.lastFull.Equal(start) {
		t.Errorf("project 1: state = %+v, want the sync mark moved and the full mark kept", st)
	}

	// A project selected later is read completely, the others are not.
	third := second.Add(5 * time.Minute)
	if err := syncer.syncProjectIssues(store, []int{1, 2, 3}, third); err != nil {
		t.Fatal(err)
	}
	want = []string{"/issues.json project=0", "/issues.json project=3", "/time_entries.json project=0", "/time_entries.json project=3"}
	if got := redmine.takeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests of the third run = %v", got)
	}
	if got := store.issueIDs(); !reflect.DeepEqual(got, []int{11, 12, 13, 21, 31}) {
		t.Errorf("issues after the third run = %v", got)
	}

	// The state survives a restart: a new syncer over the same store still
	// asks only for changes.
	restarted := NewSyncer(NewClient(srv.URL, "key", "", ""))
	if err := restarted.syncProjectIssues(store, []int{1, 2, 3}, third.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := redmine.takeRequests(); !reflect.DeepEqual(got, []string{"/issues.json project=0", "/time_entries.json project=0"}) {
		t.Errorf("requests after a restart = %v", got)
	}
}

func TestSyncDeletesIssuesGoneFromRedmine(t *testing.T) {
	pageRetryDelay = 0
	redmine := &fakeRedmine{
		issues:  map[int][]int{1: {11, 12, 13}, 2: {21}},
		entries: map[int][]string{1: {"100:11", "101:12"}},
	}
	srv := redmine.server(t)
	defer srv.Close()
	syncer := NewSyncer(NewClient(srv.URL, "key", "", ""))
	store := newMemStore()
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if err := syncer.syncProjectIssues(store, []int{1, 2}, start); err != nil {
		t.Fatal(err)
	}

	// Issue 12 is deleted in Redmine, issue 13 is moved to project 2
	redmine.issues[1] = []int{11}
	redmine.issues[2] = []int{21, 13}
	redmine.entries[1] = []string{"100:11"}

	// Changes alone do not show a deletion…
	if err := syncer.syncProjectIssues(store, []int{1, 2}, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 {
		t.Errorf("deleted on a pass of changes: %v", store.deleted)
	}

	// …the next complete pass does: the deleted issue goes with its time
	// entries, the moved one stays.
	if err := syncer.syncProjectIssues(store, []int{1, 2}, start.Add(fullSyncInterval)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.deleted, []int{12}) {
		t.Errorf("deleted = %v, want [12]", store.deleted)
	}
	if got := store.issueIDs(); !reflect.DeepEqual(got, []int{11, 13, 21}) {
		t.Errorf("issues = %v", got)
	}
	if store.issues[13] != 2 {
		t.Errorf("issue 13 is in project %d, want 2", store.issues[13])
	}
	if _, kept := store.entries[101]; kept {
		t.Error("the time entry of the deleted issue must be gone")
	}
}

func TestSyncDeletesNothingOnFailureOrSuspiciousAnswer(t *testing.T) {
	pageRetryDelay = 0
	many := make([]int, emptyAnswerGuard+1)
	for i := range many {
		many[i] = 1000 + i
	}
	redmine := &fakeRedmine{issues: map[int][]int{1: {11, 12}, 2: many}, entries: map[int][]string{}}
	srv := redmine.server(t)
	defer srv.Close()
	syncer := NewSyncer(NewClient(srv.URL, "key", "", ""))
	store := newMemStore()
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if err := syncer.syncProjectIssues(store, []int{1, 2}, start); err != nil {
		t.Fatal(err)
	}
	stored := len(store.issues)

	// Project 1 fails, project 2 suddenly answers with nothing
	redmine.broken = map[int]bool{1: true}
	redmine.issues[2] = nil
	next := start.Add(fullSyncInterval)
	if err := syncer.syncProjectIssues(store, []int{1, 2}, next); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 || len(store.issues) != stored {
		t.Errorf("deleted %v, %d issues left of %d", store.deleted, len(store.issues), stored)
	}
	// The failed project is read again next time; its marks have not moved
	if st := store.states[1]; !st.lastFull.Equal(start) {
		t.Errorf("project 1: full mark = %v, want it unchanged", st.lastFull)
	}
	// The other one was read, whatever came of it
	if st := store.states[2]; !st.lastFull.Equal(next) {
		t.Errorf("project 2: full mark = %v, want %v", st.lastFull, next)
	}
}

func TestSyncRemovesIssueMovedOutOfSyncedProjects(t *testing.T) {
	pageRetryDelay = 0
	redmine := &fakeRedmine{issues: map[int][]int{1: {11, 12}}, entries: map[int][]string{}}
	srv := redmine.server(t)
	defer srv.Close()
	syncer := NewSyncer(NewClient(srv.URL, "key", "", ""))
	store := newMemStore()
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if err := syncer.syncProjectIssues(store, []int{1}, start); err != nil {
		t.Fatal(err)
	}

	// Issue 12 is moved to project 9, which nobody selected: the change is
	// seen on the next pass and the issue is removed here at once.
	redmine.issues[1] = []int{11}
	redmine.issues[9] = []int{12}
	if err := syncer.syncProjectIssues(store, []int{1}, start.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := store.issueIDs(); !reflect.DeepEqual(got, []int{11}) {
		t.Errorf("issues = %v, want [11]", got)
	}
}
