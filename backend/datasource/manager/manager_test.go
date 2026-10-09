package manager

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"pm-dashboard/config"
	"pm-dashboard/datasource/redmine"
)

type fakeStore struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *fakeStore) LoadAppConfig() *config.AppConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &config.AppConfig{RedmineURL: s.values["redmine_url"], RedmineAPIKey: s.values["redmine_api_key"]}
}

func (s *fakeStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[key], nil
}

func (s *fakeStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

// blockingSyncer counts passes and holds each one until released.
type blockingSyncer struct {
	passes  atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (s *blockingSyncer) SyncAll(context.Context, **sql.DB) error {
	s.passes.Add(1)
	s.started <- struct{}{}
	<-s.release
	return nil
}

func newTestManager(store *fakeStore, s syncer) *Manager {
	db := &sql.DB{}
	m := New(store, &db, nil)
	m.newSyncer = func(*redmine.Client) syncer { return s }
	m.snapshot = nil // no real database in tests
	m.Reload()
	return m
}

func TestDiffIssues(t *testing.T) {
	before := map[int]string{1: "a", 2: "b", 3: "c"}
	after := map[int]string{1: "a", 2: "changed", 4: "new"}

	got := DiffIssues(before, after)
	want := IssueChanges{Added: []int{4}, Updated: []int{2}, Removed: []int{3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiffIssues = %+v, want %+v", got, want)
	}
	if !DiffIssues(before, before).Empty() {
		t.Error("identical snapshots must give no changes")
	}
}

// stepSyncer changes the "database" during its pass.
type stepSyncer struct {
	pass func()
	err  error
}

func (s *stepSyncer) SyncAll(context.Context, **sql.DB) error {
	s.pass()
	return s.err
}

func TestSyncReportsEvents(t *testing.T) {
	store := &fakeStore{values: map[string]string{"redmine_url": "http://redmine.test", "redmine_api_key": "key"}}
	issues := map[int]string{1: "a", 2: "b"}
	s := &stepSyncer{pass: func() { issues = map[int]string{1: "a", 2: "changed", 3: "new"} }}
	m := newTestManager(store, s)
	m.snapshot = func() (map[int]string, error) { return issues, nil }

	var events []string
	var changes IssueChanges
	m.SetNotifier(func(event string, data interface{}) {
		if status, ok := data.(SyncStatus); ok {
			event += ":" + status.Status
		}
		if c, ok := data.(IssueChanges); ok {
			changes = c
		}
		events = append(events, event)
	})

	current, err := m.begin()
	if err != nil {
		t.Fatal(err)
	}
	m.run(current, false)

	wantEvents := []string{"sync-status:running", "issues-changed", "collection-complete", "sync-status:success"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Errorf("events = %v, want %v", events, wantEvents)
	}
	if want := (IssueChanges{Added: []int{3}, Updated: []int{2}, Removed: []int{}}); !reflect.DeepEqual(changes, want) {
		t.Errorf("changes = %+v, want %+v", changes, want)
	}

	// A failed pass that changed nothing reports only its status.
	events = nil
	s.pass, s.err = func() {}, errors.New("redmine is down")
	current, _ = m.begin()
	m.run(current, false)
	if want := []string{"sync-status:running", "sync-status:error"}; !reflect.DeepEqual(events, want) {
		t.Errorf("events after a failure = %v, want %v", events, want)
	}
}

func TestTriggerSyncRunsOnePassAtATime(t *testing.T) {
	store := &fakeStore{values: map[string]string{"redmine_url": "http://redmine.test", "redmine_api_key": "key"}}
	s := &blockingSyncer{started: make(chan struct{}, 1), release: make(chan struct{})}
	m := newTestManager(store, s)

	if err := m.TriggerSync(); err != nil {
		t.Fatalf("first TriggerSync: %v", err)
	}
	<-s.started

	for i := 0; i < 5; i++ {
		if err := m.TriggerSync(); !errors.Is(err, ErrAlreadyRunning) {
			t.Fatalf("TriggerSync during a pass = %v, want ErrAlreadyRunning", err)
		}
	}
	close(s.release)

	// The slot is free again once the pass is over.
	for m.TriggerSync() != nil {
	}
	<-s.started
	if got := s.passes.Load(); got != 2 {
		t.Errorf("passes = %d, want 2", got)
	}
}

func TestReloadAppliesNewSettings(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	m := newTestManager(store, &blockingSyncer{})

	if m.Client() != nil {
		t.Fatal("client must be nil until the data source is configured")
	}
	if err := m.TriggerSync(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("TriggerSync = %v, want ErrNotConfigured", err)
	}

	store.Set("redmine_url", "http://redmine.test")
	store.Set("redmine_api_key", "key")
	m.Reload()
	first := m.Client()
	if first == nil {
		t.Fatal("client must exist after Reload")
	}

	m.Reload()
	if m.Client() == first {
		t.Error("Reload must replace the client")
	}
}

func TestInterval(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	m := newTestManager(store, &blockingSyncer{})

	if got := m.Interval(); got != DefaultInterval {
		t.Errorf("default interval = %d, want %d", got, DefaultInterval)
	}
	if err := m.SetInterval(15); err != nil || m.Interval() != 15 {
		t.Errorf("SetInterval(15): err = %v, interval = %d", err, m.Interval())
	}
	if err := m.SetInterval(0); err == nil {
		t.Error("SetInterval(0) must fail")
	}
	store.Set(intervalKey, "garbage")
	if got := m.Interval(); got != DefaultInterval {
		t.Errorf("interval for a broken value = %d, want %d", got, DefaultInterval)
	}
}

// waitingSyncer runs until its context ends and reports how it was called.
type waitingSyncer struct {
	started chan struct{}
}

func (s *waitingSyncer) SyncAll(ctx context.Context, _ **sql.DB) error {
	s.started <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}

func TestStopInterruptsRunningSync(t *testing.T) {
	store := &fakeStore{values: map[string]string{"redmine_url": "http://redmine.test", "redmine_api_key": "key"}}
	s := &waitingSyncer{started: make(chan struct{})}
	m := newTestManager(store, s)

	statuses := make(chan string, 4)
	m.SetNotifier(func(_ string, data interface{}) {
		if status, ok := data.(SyncStatus); ok {
			statuses <- status.Status
		}
	})
	after := atomic.Int32{}
	m.SetAfterSync(func() { after.Add(1) })

	if m.Stop() {
		t.Error("there is nothing to stop before a sync starts")
	}
	if err := m.TriggerSync(); err != nil {
		t.Fatal(err)
	}
	<-s.started
	if <-statuses != "running" {
		t.Fatal("a started sync must report that it runs")
	}
	if !m.Stop() {
		t.Fatal("a running sync must be stoppable")
	}
	if got := <-statuses; got != "stopped" {
		t.Errorf("status after a stop = %q, want stopped", got)
	}
	if after.Load() != 0 {
		t.Error("what follows a finished sync must not run after a stopped one")
	}
}

func TestAfterSyncRunsOnSuccessOnly(t *testing.T) {
	store := &fakeStore{values: map[string]string{"redmine_url": "http://redmine.test", "redmine_api_key": "key"}}
	s := &stepSyncer{pass: func() {}}
	m := newTestManager(store, s)
	calls := 0
	m.SetAfterSync(func() { calls++ })

	current, _ := m.begin()
	m.run(current, false)
	if calls != 1 {
		t.Fatalf("after a finished sync: %d calls, want 1", calls)
	}
	s.err = errors.New("redmine is down")
	current, _ = m.begin()
	m.run(current, false)
	if calls != 1 {
		t.Errorf("after a failed sync: %d calls, want still 1", calls)
	}
}

func TestScheduleSettings(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	m := newTestManager(store, &stepSyncer{pass: func() {}})

	if !m.Enabled() {
		t.Error("the periodic sync is on until it is switched off")
	}
	if err := m.SetEnabled(false); err != nil || m.Enabled() {
		t.Errorf("switched off: enabled = %v, err = %v", m.Enabled(), err)
	}
	if err := m.SetEnabled(true); err != nil || !m.Enabled() {
		t.Errorf("switched on again: enabled = %v, err = %v", m.Enabled(), err)
	}

	if got := m.FullSyncTime(); got != redmine.DefaultFullSyncTime {
		t.Errorf("default time of the full pass = %q", got)
	}
	if err := m.SetFullSyncTime("23:30"); err != nil || m.FullSyncTime() != "23:30" {
		t.Errorf("after setting 23:30: %q, err = %v", m.FullSyncTime(), err)
	}
	for _, bad := range []string{"", "24:00", "3 am", "12:60"} {
		if err := m.SetFullSyncTime(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
	if m.FullSyncTime() != "23:30" {
		t.Errorf("a refused value changed the setting to %q", m.FullSyncTime())
	}
}
