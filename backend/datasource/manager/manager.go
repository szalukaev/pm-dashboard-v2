// Package manager владеет подключением к источнику данных и фоновой
// синхронизацией, поэтому параметры подключения и интервал синхронизации
// можно менять на работающем сервере.
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

// ConfigStore — та часть хранилища конфигурации SQLite, которая нужна менеджеру.
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
	readOnly func() bool // nil — режима «только чтение» нет

	// newSyncer создаёт синхронизатор для клиента; в тестах подменяется.
	newSyncer func(*redmine.Client) syncer
	// snapshot читает отпечатки задач, по которым видны изменения; в тестах подменяется.
	snapshot func() (map[int]string, error)
	// projectSource решает, какие проекты синхронизируются; nil — как решит синхронизатор.
	projectSource func() ([]int, error)
	// beforeSync выполняется в начале каждой синхронизации; nil — ничего.
	beforeSync func()
	// afterSync выполняется после каждой завершившейся синхронизации; nil — ничего.
	afterSync func()
	// notify сообщает о событиях синхронизации открытым страницам; nil — слушать некому.
	notify func(event string, data interface{})

	mu      sync.Mutex
	client  *redmine.Client
	syncer  syncer
	running bool
	// cancel останавливает идущую синхронизацию; nil — ничего не идёт.
	cancel context.CancelFunc

	// wake заново запускает ожидание между синхронизациями (изменился интервал).
	wake chan struct{}
}

func New(store ConfigStore, db **sql.DB, readOnly func() bool) *Manager {
	m := &Manager{
		store:    store,
		db:       db,
		readOnly: readOnly,
		wake:     make(chan struct{}, 1),
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

// Reload перечитывает параметры подключения. Обработчики и следующая
// синхронизация сразу берут нового клиента; уже идущая заканчивает со старым.
func (m *Manager) Reload() {
	cfg := m.store.LoadAppConfig()

	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.RedmineURL == "" || cfg.RedmineAPIKey == "" {
		m.client, m.syncer = nil, nil
		return
	}
	m.client = redmine.NewClient(cfg.RedmineURL, cfg.RedmineAPIKey, cfg.RedmineBasicLogin, cfg.RedmineBasicPass)
	// Новый синхронизатор не помнит, что уже загружено, поэтому первая
	// синхронизация после смены параметров читает всё заново.
	m.syncer = m.newSyncer(m.client)
}

// Client возвращает текущего клиента источника данных; nil — не настроен.
func (m *Manager) Client() *redmine.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

// SourceType возвращает вид настроенного источника данных ("redmine") или
// "", если источник не настроен.
func (m *Manager) SourceType() string {
	if m.Client() == nil {
		return ""
	}
	return m.store.LoadAppConfig().DataSourceType
}

// ClientWithKey возвращает клиента источника данных, который действует с
// заданным личным ключом API вместо системного; nil — источник не настроен.
func (m *Manager) ClientWithKey(apiKey string) *redmine.Client {
	cfg := m.store.LoadAppConfig()
	if cfg.RedmineURL == "" {
		return nil
	}
	return redmine.NewClient(cfg.RedmineURL, apiKey, cfg.RedmineBasicLogin, cfg.RedmineBasicPass)
}

// Interval возвращает время между синхронизациями по расписанию, в минутах.
func (m *Manager) Interval() int {
	v, _ := m.store.Get(intervalKey)
	n, err := strconv.Atoi(v)
	if err != nil || n < MinInterval || n > MaxInterval {
		return DefaultInterval
	}
	return n
}

// SetInterval сохраняет интервал и применяет его к работающему циклу.
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

// TriggerSync запускает синхронизацию в фоне. Ошибка возвращается, если она
// не может начаться; итог самой синхронизации пишется в collection_log.
func (m *Manager) TriggerSync() error {
	return m.trigger(false)
}

// TriggerFullSync запускает синхронизацию, которая перечитывает каждый
// проект целиком, а не только изменения.
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

// Stop прерывает идущую синхронизацию и сообщает, была ли она.
func (m *Manager) Stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}

// Enabled сообщает, включена ли синхронизация по расписанию. Запущенная
// вручную работает в любом случае.
func (m *Manager) Enabled() bool {
	v, _ := m.store.Get(enabledKey)
	return v != "0"
}

// SetEnabled включает или выключает синхронизацию по расписанию.
func (m *Manager) SetEnabled(enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	return m.store.Set(enabledKey, value)
}

// FullSyncTime возвращает время суток ("HH:MM", время сервера), после
// которого проекты раз в день перечитываются целиком.
func (m *Manager) FullSyncTime() string {
	v, _ := m.store.Get(fullTimeKey)
	if _, _, ok := redmine.ParseTimeOfDay(v); !ok {
		return redmine.DefaultFullSyncTime
	}
	return v
}

// SetFullSyncTime сохраняет время ежедневного полного прохода.
func (m *Manager) SetFullSyncTime(timeOfDay string) error {
	if _, _, ok := redmine.ParseTimeOfDay(timeOfDay); !ok {
		return errors.New("time of day must be HH:MM")
	}
	return m.store.Set(fullTimeKey, timeOfDay)
}

// begin занимает единственное место для синхронизации.
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

	// Открытым страницам сообщается, что изменила синхронизация, даже
	// неудачная: часть задач могла обновиться до сбоя.
	if before != nil {
		if changes := DiffIssues(before, m.takeSnapshot()); !changes.Empty() {
			m.emit(EventIssuesChanged, changes)
		}
	}
	if errors.Is(err, context.Canceled) {
		// Остановлена администратором: это и не успех, и не сбой
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

// Running сообщает, идёт ли синхронизация.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// SetProjectSource задаёт, кто решает, какие проекты синхронизируются
// (права доступа: проекты, показанные пользователям). Вызывать до Run.
func (m *Manager) SetProjectSource(source func() ([]int, error)) {
	m.projectSource = source
	m.Reload()
}

// SetBeforeSync задаёт, что выполняется в начале каждой синхронизации, до
// выбора проектов. Вызывать до Run.
func (m *Manager) SetBeforeSync(hook func()) {
	m.beforeSync = hook
}

// SetAfterSync задаёт, что выполняется после каждой завершившейся
// синхронизации, до сообщения открытым страницам. Вызывать до Run.
func (m *Manager) SetAfterSync(hook func()) {
	m.afterSync = hook
}

// SetNotifier задаёт, куда менеджер сообщает о событиях синхронизации
// (хаб WebSocket). Вызывать до Run.
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

// issueFingerprints возвращает хэш видимых полей каждой задачи.
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

// Run синхронизирует один раз вскоре после запуска и затем по расписанию, пока жив ctx.
func (m *Manager) Run(ctx context.Context) {
	// Синхронизация, оборванная перезапуском, навсегда оставила в журнале запись «running».
	if *m.db != nil {
		if _, err := (*m.db).Exec(`UPDATE collection_log SET status = 'error', finished_at = NOW(),
			error_text = 'interrupted by a server restart' WHERE status = 'running'`); err != nil {
			slog.Warn("Database write failed", "at", "Run", "error", err)
		}
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
				// Выключена администратором; запуск вручную по-прежнему работает
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
