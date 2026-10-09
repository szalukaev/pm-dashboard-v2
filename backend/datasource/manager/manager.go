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
	enabledKey      = "sync_enabled"
	fullTimeKey     = "full_sync_time"
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
	// beforeSync runs at the start of every sync; nil = nothing.
	beforeSync func()
	// afterSync runs after every sync that finished; nil = nothing.
	afterSync func()
	// notify reports sync events to open pages; nil = nobody listens.
	notify func(event string, data interface{})

	mu      sync.Mutex
	client  *redmine.Client
	syncer  syncer
	running bool
	// cancel stops the sync that is running; nil when none is.
	cancel context.CancelFunc

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
		s.FullSyncTime = m.FullSyncTime
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
	return m.trigger(false)
}

// TriggerFullSync starts a sync that re-reads every project completely
// instead of only what changed.
func (m *Manager) TriggerFullSync() error {
	return m.trigger(true)
}

func (m *Manager) trigger(full bool) error {
	s, err := m.begin()
	if err != nil {
		return err
	}
	go m.run(s, full)
	return nil
}

// Stop interrupts the sync that is running. It reports whether there was one.
func (m *Manager) Stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}

// Enabled reports whether the periodic sync is on. A sync started by hand
// runs either way.
func (m *Manager) Enabled() bool {
	v, _ := m.store.Get(enabledKey)
	return v != "0"
}

// SetEnabled switches the periodic sync on or off.
func (m *Manager) SetEnabled(enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	return m.store.Set(enabledKey, value)
}

// FullSyncTime returns the time of day ("HH:MM", server time) after which
// the projects are re-read completely once a day.
func (m *Manager) FullSyncTime() string {
	v, _ := m.store.Get(fullTimeKey)
	if _, _, ok := redmine.ParseTimeOfDay(v); !ok {
		return redmine.DefaultFullSyncTime
	}
	return v
}

// SetFullSyncTime saves the time of the daily complete pass.
func (m *Manager) SetFullSyncTime(timeOfDay string) error {
	if _, _, ok := redmine.ParseTimeOfDay(timeOfDay); !ok {
		return errors.New("time of day must be HH:MM")
	}
	return m.store.Set(fullTimeKey, timeOfDay)
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

func (m *Manager) run(s syncer, full bool) {
	ctx, cancel := context.WithCancel(context.Background())
	if full {
		ctx = redmine.WithFullSync(ctx)
	}
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		m.running = false
		m.cancel = nil
		m.mu.Unlock()
	}()

	m.emit(EventSyncStatus, SyncStatus{Status: "running", At: time.Now()})
	if m.beforeSync != nil {
		m.beforeSync()
	}
	before := m.takeSnapshot()

	err := s.SyncAll(ctx, m.db)

	// Tell open pages what the sync changed, even a failed one: a part of
	// the issues may have been updated before the failure.
	if before != nil {
		if changes := DiffIssues(before, m.takeSnapshot()); !changes.Empty() {
			m.emit(EventIssuesChanged, changes)
		}
	}
	if errors.Is(err, context.Canceled) {
		// Stopped by the administrator: neither a success nor a failure
		m.emit(EventSyncStatus, SyncStatus{Status: "stopped", At: time.Now()})
		return
	}
	if err != nil {
		slog.Error("Sync failed", "error", err)
		m.emit(EventSyncStatus, SyncStatus{Status: "error", At: time.Now()})
		return
	}
	if m.afterSync != nil {
		m.afterSync()
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

// SetBeforeSync sets what runs at the start of every sync, before the
// projects to sync are chosen. Call it before Run.
func (m *Manager) SetBeforeSync(hook func()) {
	m.beforeSync = hook
}

// SetAfterSync sets what runs after every sync that finished, before open
// pages are told about it. Call it before Run.
func (m *Manager) SetAfterSync(hook func()) {
	m.afterSync = hook
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
			if !m.Enabled() {
				// Switched off by the administrator; a run by hand still works
				break
			}
			if s, err := m.begin(); err == nil {
				m.run(s, false)
			} else if !errors.Is(err, ErrNotConfigured) && !errors.Is(err, ErrNoDatabase) {
				slog.Warn("Skipping periodic sync", "reason", err)
			}
		}
		wait = time.Duration(m.Interval()) * time.Minute
	}
}
