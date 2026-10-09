package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/jobs"
	"pm-dashboard/snapshots"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

// WorkersHandler serves the screens where the administrator sees the
// background workers and manages the history they collect.
type WorkersHandler struct {
	DB       **sql.DB
	Source   *manager.Manager
	Cleaner  *jobs.Cleaner
	Settings *jobs.Settings
	// Events reports changes to open pages (WebSocket); may be nil.
	Events func(event string, data interface{})
}

// Keys of the workers.
const (
	workerSync    = "sync"
	workerCleanup = jobs.CleanupKey
)

// What the screen shows as the state of a worker.
const (
	workerWorking  = "working"  // on and its last run went well
	workerStopped  = "stopped"  // switched off
	workerError    = "error"    // its last run failed
	workerStarting = "starting" // running right now
)

type workerInfo struct {
	Key        string     `json:"key"`
	State      string     `json:"state"`
	Enabled    bool       `json:"enabled"`
	Running    bool       `json:"running"`
	LastStart  *time.Time `json:"last_start"`
	DurationMs *int64     `json:"duration_ms"`
	Processed  *int64     `json:"processed"`
	// Error is the text of the failure. ErrorKind "partial" means that Error
	// holds "<failed>/<total>" projects of a sync that finished.
	Error     string                 `json:"error"`
	ErrorKind string                 `json:"error_kind"`
	Schedule  map[string]interface{} `json:"schedule"`
}

func workerState(enabled, running, failed bool) string {
	switch {
	case running:
		return workerStarting
	case !enabled:
		return workerStopped
	case failed:
		return workerError
	default:
		return workerWorking
	}
}

func (h *WorkersHandler) notify() {
	if h.Events != nil {
		h.Events(jobs.EventWorkersChanged, nil)
	}
}

// ListWorkers returns the background workers with their state.
func (h *WorkersHandler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"workers":     []workerInfo{h.syncWorker(), h.cleanupWorker()},
		"server_time": now,
		"timezone":    now.Location().String(),
	})
}

func (h *WorkersHandler) syncWorker() workerInfo {
	info := workerInfo{Key: workerSync, Schedule: map[string]interface{}{}}
	if h.Source != nil {
		info.Enabled = h.Source.Enabled()
		info.Running = h.Source.Running()
		info.Schedule["interval_minutes"] = h.Source.Interval()
		info.Schedule["full_sync_time"] = h.Source.FullSyncTime()
		info.Schedule["interval_min"] = manager.MinInterval
		info.Schedule["interval_max"] = manager.MaxInterval
	}

	failed := false
	if *h.DB != nil {
		// The last run that is over; the one going on has nothing to show yet
		var started time.Time
		var duration, processed *int64
		var status, errorText string
		err := (*h.DB).QueryRow(`SELECT started_at, duration_ms, issues_collected, status, COALESCE(error_text, '')
			FROM collection_log ORDER BY (status = 'running'), started_at DESC LIMIT 1`).
			Scan(&started, &duration, &processed, &status, &errorText)
		if err == nil {
			info.LastStart, info.DurationMs, info.Processed = &started, duration, processed
			switch status {
			case redmine.StatusError:
				failed, info.Error = true, errorText
			case redmine.StatusPartial:
				failed, info.Error, info.ErrorKind = true, errorText, "partial"
			}
		}
	}
	info.State = workerState(info.Enabled, info.Running, failed)
	return info
}

func (h *WorkersHandler) cleanupWorker() workerInfo {
	status := h.Cleaner.Status()
	info := workerInfo{
		Key:       workerCleanup,
		Enabled:   h.Settings.CleanupEnabled(),
		Running:   h.Cleaner.Running(),
		LastStart: status.LastStart,
		Error:     status.Error,
		Schedule: map[string]interface{}{
			"time":           h.Settings.CleanupTime(),
			"retention_days": h.Settings.RetentionDays(),
		},
	}
	if status.LastStart != nil {
		info.DurationMs, info.Processed = &status.DurationMs, &status.Processed
	}
	info.State = workerState(info.Enabled, info.Running, status.State == jobs.StateError)
	return info
}

// RunWorker starts a worker now. For the sync, {"full": true} asks for a
// pass that re-reads every project completely.
func (h *WorkersHandler) RunWorker(w http.ResponseWriter, r *http.Request) {
	switch mux.Vars(r)["key"] {
	case workerSync:
		var body struct {
			Full bool `json:"full"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if h.Source == nil {
			utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
			return
		}
		start := h.Source.TriggerSync
		if body.Full {
			start = h.Source.TriggerFullSync
		}
		switch err := start(); {
		case err == nil:
			utils.Success(w)
		case errors.Is(err, manager.ErrAlreadyRunning):
			utils.Error(w, http.StatusConflict, "SYNC_ALREADY_RUNNING")
		case errors.Is(err, manager.ErrNotConfigured):
			utils.Error(w, http.StatusBadRequest, "DATA_SOURCE_NOT_CONFIGURED")
		case errors.Is(err, manager.ErrReadOnly):
			utils.Error(w, http.StatusForbidden, "READ_ONLY_MODE")
		default:
			utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		}
	case workerCleanup:
		h.cleanup(w, h.Cleaner.RunNow)
	default:
		utils.Error(w, http.StatusNotFound, "WORKER_NOT_FOUND")
	}
}

// StopWorker interrupts the run of a worker that is going on.
func (h *WorkersHandler) StopWorker(w http.ResponseWriter, r *http.Request) {
	switch mux.Vars(r)["key"] {
	case workerSync:
		if h.Source == nil || !h.Source.Stop() {
			utils.Error(w, http.StatusConflict, "WORKER_NOT_RUNNING")
			return
		}
		utils.Success(w)
	case workerCleanup:
		// A cleanup is one short statement: there is nothing to interrupt
		utils.Error(w, http.StatusConflict, "WORKER_NOT_RUNNING")
	default:
		utils.Error(w, http.StatusNotFound, "WORKER_NOT_FOUND")
	}
}

// EnableWorker and DisableWorker switch the schedule of a worker on and off.
func (h *WorkersHandler) EnableWorker(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}
func (h *WorkersHandler) DisableWorker(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *WorkersHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	var err error
	switch mux.Vars(r)["key"] {
	case workerSync:
		if h.Source == nil {
			utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
			return
		}
		err = h.Source.SetEnabled(enabled)
	case workerCleanup:
		err = h.Settings.SetCleanupEnabled(enabled)
		h.Cleaner.Rescheduled()
	default:
		utils.Error(w, http.StatusNotFound, "WORKER_NOT_FOUND")
		return
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}
	h.notify()
	utils.Success(w)
}

// UpdateSchedule saves when a worker runs.
func (h *WorkersHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IntervalMinutes *int    `json:"interval_minutes"`
		FullSyncTime    *string `json:"full_sync_time"`
		Time            *string `json:"time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	switch mux.Vars(r)["key"] {
	case workerSync:
		if h.Source == nil {
			utils.Error(w, http.StatusServiceUnavailable, "SYNC_NOT_AVAILABLE")
			return
		}
		// Everything is checked before anything is saved
		if body.IntervalMinutes != nil && (*body.IntervalMinutes < manager.MinInterval || *body.IntervalMinutes > manager.MaxInterval) {
			utils.Error(w, http.StatusBadRequest, "INVALID_INTERVAL")
			return
		}
		if body.FullSyncTime != nil && !jobs.ValidTimeOfDay(*body.FullSyncTime) {
			utils.Error(w, http.StatusBadRequest, "INVALID_TIME")
			return
		}
		if body.IntervalMinutes != nil {
			if err := h.Source.SetInterval(*body.IntervalMinutes); err != nil {
				utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
				return
			}
		}
		if body.FullSyncTime != nil {
			if err := h.Source.SetFullSyncTime(*body.FullSyncTime); err != nil {
				utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
				return
			}
		}
	case workerCleanup:
		if body.Time == nil || !jobs.ValidTimeOfDay(*body.Time) {
			utils.Error(w, http.StatusBadRequest, "INVALID_TIME")
			return
		}
		if err := h.Settings.SetCleanupTime(*body.Time); err != nil {
			utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
			return
		}
		h.Cleaner.Rescheduled()
	default:
		utils.Error(w, http.StatusNotFound, "WORKER_NOT_FOUND")
		return
	}
	h.notify()
	utils.Success(w)
}

// ─── History ───

// GetDataRetention returns how long the history is kept and how much of it
// is stored.
func (h *WorkersHandler) GetDataRetention(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	stats, err := snapshots.GetStats(*h.DB)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"retention_days":  h.Settings.RetentionDays(),
		"min":             jobs.MinRetentionDays,
		"max":             jobs.MaxRetentionDays,
		"cleanup_enabled": h.Settings.CleanupEnabled(),
		"cleanup_time":    h.Settings.CleanupTime(),
		"stats":           stats,
	})
}

// UpdateDataRetention saves the retention and whether the cleanup is on.
func (h *WorkersHandler) UpdateDataRetention(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RetentionDays  *int  `json:"retention_days"`
		CleanupEnabled *bool `json:"cleanup_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if body.RetentionDays != nil && !jobs.ValidRetention(*body.RetentionDays) {
		utils.Error(w, http.StatusBadRequest, "INVALID_RETENTION")
		return
	}
	if body.RetentionDays != nil {
		if err := h.Settings.SetRetentionDays(*body.RetentionDays); err != nil {
			utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
			return
		}
	}
	if body.CleanupEnabled != nil {
		if err := h.Settings.SetCleanupEnabled(*body.CleanupEnabled); err != nil {
			utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
			return
		}
		h.Cleaner.Rescheduled()
	}
	h.notify()
	utils.Success(w)
}

// CleanupSnapshots deletes the snapshots older than the retention now.
func (h *WorkersHandler) CleanupSnapshots(w http.ResponseWriter, r *http.Request) {
	h.cleanup(w, h.Cleaner.RunNow)
}

// PurgeSnapshots deletes the whole history.
func (h *WorkersHandler) PurgeSnapshots(w http.ResponseWriter, r *http.Request) {
	h.cleanup(w, h.Cleaner.Purge)
}

func (h *WorkersHandler) cleanup(w http.ResponseWriter, run func() (int64, error)) {
	deleted, err := run()
	switch {
	case err == nil:
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "deleted": deleted})
	case errors.Is(err, jobs.ErrBusy):
		utils.Error(w, http.StatusConflict, "CLEANUP_ALREADY_RUNNING")
	case errors.Is(err, jobs.ErrNoDatabase):
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
	default:
		utils.Error(w, http.StatusInternalServerError, "CLEANUP_FAILED")
	}
}
