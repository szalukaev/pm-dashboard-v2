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

// CleanupKey names the cleanup among the workers.
const CleanupKey = "cleanup"

// EventWorkersChanged tells open pages that the state of a worker changed.
const EventWorkersChanged = "workers-changed"

// retryAfterFailure: a failed cleanup is tried again this much later.
const retryAfterFailure = time.Hour

// ErrBusy: a cleanup is already running.
var ErrBusy = errors.New("cleanup is already running")

// Cleaner deletes the snapshots that are older than the retention: every
// day at the set time, and when the administrator asks for it.
type Cleaner struct {
	DB       **sql.DB
	Settings *Settings
	Store    Store
	// ReadOnly: no scheduled cleanup while it returns true; may be nil.
	ReadOnly func() bool
	// Notify reports a change of the state to open pages; may be nil.
	Notify func(event string, data interface{})

	mu      sync.Mutex
	running bool
	wake    chan struct{}
}

func NewCleaner(db **sql.DB, settings *Settings, store Store) *Cleaner {
	return &Cleaner{DB: db, Settings: settings, Store: store, wake: make(chan struct{}, 1)}
}

// NextRun returns the next moment after now when the clock shows the given
// time of day. A value that is not a time of day gives the default time.
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

// nextAttempt is when the loop wakes up: at the scheduled time, or earlier
// when a failed run is waiting for its retry.
func nextAttempt(now time.Time, timeOfDay string, retryAt time.Time) time.Time {
	next := NextRun(now, timeOfDay)
	if !retryAt.IsZero() && retryAt.Before(next) {
		return retryAt
	}
	return next
}

// Rescheduled makes the loop re-read the schedule (it was changed).
func (c *Cleaner) Rescheduled() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Run is the loop of the scheduled cleanup; it ends with ctx.
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
			// Not set up yet
		case !c.Settings.CleanupEnabled():
			// Switched off by the administrator
		case c.ReadOnly != nil && c.ReadOnly():
			// Nothing is changed without a working license
		default:
			if _, err := c.RunNow(); err != nil && !errors.Is(err, ErrBusy) {
				retryAt = time.Now().Add(retryAfterFailure)
				slog.Warn("Cleanup of snapshots failed, it will be tried again", "at", retryAt, "error", err)
			}
		}
	}
}

// RunNow deletes the snapshots older than the retention and returns how
// many rows were deleted.
func (c *Cleaner) RunNow() (int64, error) {
	return c.run(func(db *sql.DB) (int64, error) {
		return snapshots.Cleanup(db, time.Now(), c.Settings.RetentionDays())
	})
}

// Purge deletes every snapshot.
func (c *Cleaner) Purge() (int64, error) {
	return c.run(snapshots.Purge)
}

// Running reports whether a cleanup is going on.
func (c *Cleaner) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Status returns the state of the last run.
func (c *Cleaner) Status() Status {
	status, ok := c.Store.Load(CleanupKey)
	if !ok {
		status = Status{State: StateIdle}
	}
	// A run cut short by a restart left "running" behind
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
