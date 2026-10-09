package manager

import (
	"context"
	"database/sql"
	"errors"
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
	m.Reload()
	return m
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
