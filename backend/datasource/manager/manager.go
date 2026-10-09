// Package manager owns the connection to the data source and its background
// sync, so connection settings and the sync interval can change while the
// server is running.
package manager

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"pm-dashboard/config"
	"pm-dashboard/datasource/redmine"
)

const (
	intervalKey     = "sync_interval_minutes"
	DefaultInterval = 5
	MinInterval     = 1
	MaxInterval     = 1440
)

var (
	ErrNotConfigured  = errors.New("data source is not configured")
	ErrAlreadyRunning = errors.New("sync is already running")
	ErrNoDatabase     = errors.New("database is not available")
	ErrReadOnly       = errors.New("system is in read-only mode")
)

// ConfigStore is the part of the SQLite config store the manager needs.
type ConfigStore interface {
	LoadAppConfig() *config.AppConfig
	Get(key string) (string, error)
	Set(key, value string) error
}

type syncer interface {
	SyncAll(ctx context.Context, db **sql.DB) error
}

type Manager struct {
	store    ConfigStore
	db       **sql.DB
	readOnly func() bool // nil = never read-only

	// newSyncer builds the syncer for a client; replaced in tests.
	newSyncer func(*redmine.Client) syncer

	mu      sync.Mutex
	client  *redmine.Client
	syncer  syncer
	running bool

	// wake restarts the wait between periodic syncs (the interval changed).
	wake chan struct{}
}

func New(store ConfigStore, db **sql.DB, readOnly func() bool) *Manager {
	m := &Manager{
		store:     store,
		db:        db,
		readOnly:  readOnly,
		newSyncer: func(c *redmine.Client) syncer { return redmine.NewSyncer(c) },
		wake:      make(chan struct{}, 1),
	}
	m.Reload()
	return m
}

// Reload re-reads the connection settings. Handlers and the next sync use the
// new client at once; a sync that is already running finishes with the old one.
func (m *Manager) Reload() {
	cfg := m.store.LoadAppConfig()

	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.RedmineURL == "" || cfg.RedmineAPIKey == "" {
		m.client, m.syncer = nil, nil
		return
	}
	m.client = redmine.NewClient(cfg.RedmineURL, cfg.RedmineAPIKey, cfg.RedmineBasicLogin, cfg.RedmineBasicPass)
	// A new syncer drops the incremental state, so the first sync after a
	// change of settings reads everything again.
	m.syncer = m.newSyncer(m.client)
}

// Client returns the current data source client, nil when not configured.
func (m *Manager) Client() *redmine.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

// Interval returns the time between periodic syncs in minutes.
func (m *Manager) Interval() int {
	v, _ := m.store.Get(intervalKey)
	n, err := strconv.Atoi(v)
	if err != nil || n < MinInterval || n > MaxInterval {
		return DefaultInterval
	}
	return n
}

// SetInterval saves the interval and applies it to the running loop.
func (m *Manager) SetInterval(minutes int) error {
	if minutes < MinInterval || minutes > MaxInterval {
		return errors.New("interval out of range")
	}
	if err := m.store.Set(intervalKey, strconv.Itoa(minutes)); err != nil {
		return err
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return nil
}

// TriggerSync starts a sync in the background. It returns an error when the
// sync cannot start; the outcome of the sync itself goes to collection_log.
func (m *Manager) TriggerSync() error {
	s, err := m.begin()
	if err != nil {
		return err
	}
	go m.run(s)
	return nil
}

// begin claims the single sync slot.
func (m *Manager) begin() (syncer, error) {
	if *m.db == nil {
		return nil, ErrNoDatabase
	}
	if m.readOnly != nil && m.readOnly() {
		return nil, ErrReadOnly
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.syncer == nil {
		return nil, ErrNotConfigured
	}
	if m.running {
		return nil, ErrAlreadyRunning
	}
	m.running = true
	return m.syncer, nil
}

func (m *Manager) run(s syncer) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()
	if err := s.SyncAll(context.Background(), m.db); err != nil {
		slog.Error("Sync failed", "error", err)
	}
}

// Run syncs once shortly after start and then periodically until ctx is done.
func (m *Manager) Run(ctx context.Context) {
	wait := 2 * time.Second
	for {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-m.wake:
			timer.Stop()
		case <-timer.C:
			if s, err := m.begin(); err == nil {
				m.run(s)
			} else if !errors.Is(err, ErrNotConfigured) && !errors.Is(err, ErrNoDatabase) {
				slog.Warn("Skipping periodic sync", "reason", err)
			}
		}
		wait = time.Duration(m.Interval()) * time.Minute
	}
}
