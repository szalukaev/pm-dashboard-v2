package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/utils"
)

// SyncStatusHandler serves the state of the background sync to every user
// (the indicator in the header). The detailed log is admin-only.
type SyncStatusHandler struct {
	DB     **sql.DB
	Source *manager.Manager
}

// GetStatus returns the state of the last sync:
// running | success | error | unknown (no sync yet).
func (h *SyncStatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{"status": "unknown", "at": nil}
	if *h.DB == nil {
		utils.JSON(w, http.StatusOK, result)
		return
	}

	var status string
	var startedAt time.Time
	var finishedAt *time.Time
	err := (*h.DB).QueryRow(`SELECT status, started_at, finished_at FROM collection_log
		ORDER BY started_at DESC LIMIT 1`).Scan(&status, &startedAt, &finishedAt)
	if err == nil {
		at := startedAt
		if finishedAt != nil {
			at = *finishedAt
		}
		// The indicator knows three states. A sync that could not read a
		// part of the projects is a problem to look at; one stopped by the
		// administrator says nothing about the data.
		switch status {
		case redmine.StatusPartial:
			status = redmine.StatusError
		case redmine.StatusStopped:
			status = "unknown"
		}
		result["status"] = status
		result["at"] = at
	}
	// The manager knows for sure whether a sync is going on right now.
	if h.Source != nil && h.Source.Running() {
		result["status"] = "running"
	}
	utils.JSON(w, http.StatusOK, result)
}

// notifyIssuesUpdated tells open pages that a user changed issues, so they
// refresh without waiting for the next sync. events may be nil.
func notifyIssuesUpdated(events func(event string, data interface{}), ids ...int) {
	if events == nil {
		return
	}
	events(manager.EventIssuesChanged, manager.IssueChanges{Added: []int{}, Updated: ids, Removed: []int{}})
}
