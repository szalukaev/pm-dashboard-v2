package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"pm-dashboard/middleware"
	"pm-dashboard/notify"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

// NotificationsHandler отдаёт колокольчик пользователя: журнал его
// уведомлений и отметки о прочтении.
type NotificationsHandler struct {
	DB **sql.DB
}

// notificationsPage — сколько уведомлений в странице колокольчика по умолчанию.
const notificationsPage = 20

// IssueEdited сообщает уведомлениям, что пользователь изменил задачу в самом
// дашборде: остальным об этом сообщается сразу, а ему самому — нет. Задаётся
// при запуске; nil — уведомления не подключены.
var IssueEdited func(issueID, actorID int, at time.Time)

// issueEdited вызывается обработчиками после сохранения изменения задачи.
func issueEdited(r *http.Request, issueID int) {
	if IssueEdited != nil {
		go IssueEdited(issueID, middleware.GetUserID(r), time.Now())
	}
}

// List возвращает страницу уведомлений, новые сверху, число всех и число
// непрочитанных.
func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	limit, offset := pageOf(r)
	if limit == 0 {
		limit = notificationsPage
	}
	list, total, unread, err := notify.List(*h.DB, middleware.GetUserID(r), limit, offset)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"notifications": list, "total": total, "unread": unread, "limit": limit, "offset": offset,
	})
}

// MarkRead помечает одно уведомление прочитанным.
func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || id <= 0 {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	h.markRead(w, r, id)
}

// MarkAllRead помечает прочитанными все уведомления пользователя.
func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	h.markRead(w, r, 0)
}

// Clear убирает все уведомления пользователя из колокольчика.
func (h *NotificationsHandler) Clear(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if err := notify.ClearBell(*h.DB, middleware.GetUserID(r)); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.Success(w)
}

func (h *NotificationsHandler) markRead(w http.ResponseWriter, r *http.Request, id int64) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if err := notify.MarkRead(*h.DB, middleware.GetUserID(r), id); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.Success(w)
}
