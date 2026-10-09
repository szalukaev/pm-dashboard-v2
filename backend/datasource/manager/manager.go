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
	// snapshot reads issue fingerprints for change detection; replaced in tests.
	snapshot func() (map[int]string, error)
	// projectSource decides which projects are synced; nil = the syncer's default.
	projectSource func() ([]int, error)
	// notify reports sync events to open pages; nil = nobody listens.
	notify func(event string, data interface{})

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
		wake:      make(chan struct{}, 1),
	}
	m.newSyncer = func(c *redmine.Client) syncer {
		s := redmine.NewSyncer(c)
		s.Projects = m.projectSource
		return s
	}
	m.snapshot = m.issueFingerprints
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

// SourceType returns the kind of the configured data source ("redmine"),
// or "" when none is configured.
func (m *Manager) SourceType() string {
	if m.Client() == nil {
		return ""
	}
	return m.store.LoadAppConfig().DataSourceType
}

// ClientWithKey returns a client of the data source that acts with the given
// personal API key instead of the system one; nil when not configured.
func (m *Manager) ClientWithKey(apiKey string) *redmine.Client {
	cfg := m.store.LoadAppConfig()
	if cfg.RedmineURL == "" {
		return nil
	}
	return redmine.NewClient(cfg.RedmineURL, apiKey, cfg.RedmineBasicLogin, cfg.RedmineBasicPass)
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

	m.emit(EventSyncStatus, SyncStatus{Status: "running", At: time.Now()})
	before := m.takeSnapshot()

	err := s.SyncAll(context.Background(), m.db)

	// Tell open pages what the sync changed, even a failed one: a part of
	// the issues may have been updated before the failure.
	if before != nil {
		if changes := DiffIssues(before, m.takeSnapshot()); !changes.Empty() {
			m.emit(EventIssuesChanged, changes)
		}
	}
	if err != nil {
		slog.Error("Sync failed", "error", err)
		m.emit(EventSyncStatus, SyncStatus{Status: "error", At: time.Now()})
		return
	}
	m.emit(EventCollectionComplete, nil)
	m.emit(EventSyncStatus, SyncStatus{Status: "success", At: time.Now()})
}

// Running reports whether a sync is in progress.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// SetProjectSource sets what decides which projects are synced (access
// control: the projects the users are shown). Call it before Run.
func (m *Manager) SetProjectSource(source func() ([]int, error)) {
	m.projectSource = source
	m.Reload()
}

// SetNotifier sets where the manager reports sync events (the WebSocket hub).
// Call it before Run.
func (m *Manager) SetNotifier(notify func(event string, data interface{})) {
	m.notify = notify
}

func (m *Manager) emit(event string, data interface{}) {
	if m.notify != nil {
		m.notify(event, data)
	}
}

func (m *Manager) takeSnapshot() map[int]string {
	if m.snapshot == nil {
		return nil
	}
	snap, err := m.snapshot()
	if err != nil {
		slog.Warn("Could not snapshot issues for change detection", "error", err)
		return nil
	}
	return snap
}

// issueFingerprints returns a hash of the visible fields of every issue.
func (m *Manager) issueFingerprints() (map[int]string, error) {
	rows, err := (*m.db).Query(`SELECT external_id, md5(ROW(
			project_id, subject, status_id, priority_id, assigned_to_id, category_name,
			start_date, due_date, estimated_hours, spent_hours, bug_fix_hours, done_ratio, tracker_name
		)::text) FROM issues WHERE data_source = 'redmine'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	snap := make(map[int]string)
	for rows.Next() {
		var id int
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		snap[id] = hash
	}
	return snap, rows.Err()
}

// Run syncs once shortly after start and then periodically until ctx is done.
func (m *Manager) Run(ctx context.Context) {
	// A sync cut short by a restart left its log entry "running" forever.
	if *m.db != nil {
		(*m.db).Exec(`UPDATE collection_log SET status = 'error', finished_at = NOW(),
			error_text = 'interrupted by a server restart' WHERE status = 'running'`)
	}

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
