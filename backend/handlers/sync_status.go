package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/utils"
)

// SyncStatusHandler отдаёт состояние фоновой синхронизации каждому
// пользователю (индикатор в шапке). Подробный журнал — только администратору.
type SyncStatusHandler struct {
	DB     **sql.DB
	Source *manager.Manager
}

// GetStatus возвращает состояние последней синхронизации:
// running | success | error | unknown (синхронизации ещё не было).
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
		// Индикатор знает три состояния. Синхронизация, не прочитавшая часть
		// проектов, — проблема, на которую стоит посмотреть; остановленная
		// администратором о данных ничего не говорит.
		switch status {
		case redmine.StatusPartial:
			status = redmine.StatusError
		case redmine.StatusStopped:
			status = "unknown"
		}
		result["status"] = status
		result["at"] = at
	}
	// Менеджер точно знает, идёт ли синхронизация прямо сейчас.
	if h.Source != nil && h.Source.Running() {
		result["status"] = "running"
	}
	utils.JSON(w, http.StatusOK, result)
}

// notifyIssuesUpdated сообщает открытым страницам, что пользователь изменил
// задачи, чтобы они обновились, не дожидаясь синхронизации. events может
// быть nil.
func notifyIssuesUpdated(events func(event string, data interface{}), ids ...int) {
	if events == nil {
		return
	}
	events(manager.EventIssuesChanged, manager.IssueChanges{Added: []int{}, Updated: ids, Removed: []int{}})
}
