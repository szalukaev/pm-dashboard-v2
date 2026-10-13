package jobs

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"pm-dashboard/snapshots"
)

// CleanupKey — имя очистки среди воркеров.
const CleanupKey = "cleanup"

// EventWorkersChanged сообщает открытым страницам, что состояние воркера изменилось.
const EventWorkersChanged = "workers-changed"

// retryAfterFailure: через сколько повторяется неудавшаяся очистка.
const retryAfterFailure = time.Hour

// ErrBusy: очистка уже идёт.
var ErrBusy = errors.New("cleanup is already running")

// Cleaner удаляет снимки старше срока хранения: каждый день в заданное
// время и по запросу администратора.
type Cleaner struct {
	DB       **sql.DB
	Settings *Settings
	Store    Store
	// ReadOnly: пока возвращает true, очистки по расписанию нет; может быть nil.
	ReadOnly func() bool
	// Notify сообщает открытым страницам о смене состояния; может быть nil.
	Notify func(event string, data interface{})

	mu      sync.Mutex
	running bool
	wake    chan struct{}
}

func NewCleaner(db **sql.DB, settings *Settings, store Store) *Cleaner {
	return &Cleaner{DB: db, Settings: settings, Store: store, wake: make(chan struct{}, 1)}
}

// NextRun возвращает ближайший момент после now, когда часы покажут
// заданное время суток. Значение, не являющееся временем суток, даёт время
// по умолчанию.
func NextRun(now time.Time, timeOfDay string) time.Time {
	t, err := time.Parse("15:04", timeOfDay)
	if err != nil {
		t, _ = time.Parse("15:04", DefaultCleanupTime)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// nextAttempt — когда цикл просыпается: по расписанию или раньше, если
// неудавшийся запуск ждёт повтора.
func nextAttempt(now time.Time, timeOfDay string, retryAt time.Time) time.Time {
	next := NextRun(now, timeOfDay)
	if !retryAt.IsZero() && retryAt.Before(next) {
		return retryAt
	}
	return next
}

// Rescheduled заставляет цикл перечитать расписание (оно изменилось).
func (c *Cleaner) Rescheduled() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Run — цикл очистки по расписанию; завершается вместе с ctx.
func (c *Cleaner) Run(ctx context.Context) {
	var retryAt time.Time
	for {
		timer := time.NewTimer(time.Until(nextAttempt(time.Now(), c.Settings.CleanupTime(), retryAt)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-c.wake:
			timer.Stop()
			continue
		case <-timer.C:
		}

		retryAt = time.Time{}
		switch {
		case c.DB == nil || *c.DB == nil:
			// Ещё не настроено
		case !c.Settings.CleanupEnabled():
			// Выключено администратором
		case c.ReadOnly != nil && c.ReadOnly():
			// Без действующей лицензии ничего не меняется
		default:
			if _, err := c.RunNow(); err != nil && !errors.Is(err, ErrBusy) {
				retryAt = time.Now().Add(retryAfterFailure)
				slog.Warn("Cleanup of snapshots failed, it will be tried again", "at", retryAt, "error", err)
			}
		}
	}
}

// RunNow удаляет снимки старше срока хранения и возвращает число удалённых
// строк.
func (c *Cleaner) RunNow() (int64, error) {
	return c.run(func(db *sql.DB) (int64, error) {
		return snapshots.Cleanup(db, time.Now(), c.Settings.RetentionDays())
	})
}

// Purge удаляет все снимки.
func (c *Cleaner) Purge() (int64, error) {
	return c.run(snapshots.Purge)
}

// Running сообщает, идёт ли очистка.
func (c *Cleaner) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Status возвращает состояние последнего запуска.
func (c *Cleaner) Status() Status {
	status, ok := c.Store.Load(CleanupKey)
	if !ok {
		status = Status{State: StateIdle}
	}
	// Запуск, оборванный перезапуском, оставил после себя «running»
	if status.State == StateRunning && !c.Running() {
		status.State = StateIdle
	}
	return status
}

func (c *Cleaner) run(work func(*sql.DB) (int64, error)) (int64, error) {
	if c.DB == nil || *c.DB == nil {
		return 0, ErrNoDatabase
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return 0, ErrBusy
	}
	c.running = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	started := time.Now()
	c.save(Status{State: StateRunning, LastStart: &started})

	deleted, err := work(*c.DB)
	status := Status{State: StateIdle, LastStart: &started, DurationMs: time.Since(started).Milliseconds(), Processed: deleted}
	if err != nil {
		status.State, status.Error = StateError, err.Error()
		slog.Error("Cleanup of snapshots failed", "duration", time.Since(started), "error", err)
	} else {
		slog.Info("Snapshots cleaned up", "deleted", deleted, "duration", time.Since(started))
	}
	c.save(status)
	return deleted, err
}

func (c *Cleaner) save(status Status) {
	c.Store.Save(CleanupKey, status)
	if c.Notify != nil {
		c.Notify(EventWorkersChanged, nil)
	}
}
