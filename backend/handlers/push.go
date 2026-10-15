package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"pm-dashboard/middleware"
	"pm-dashboard/notify"
	"pm-dashboard/push"
	"pm-dashboard/utils"
)

// PushHandler подписывает браузеры и устройства пользователя на push.
type PushHandler struct {
	Service *push.Service
	// Notifications создаёт проверочное уведомление по задаче пользователя
	Notifications *notify.Engine
}

// Key отдаёт открытый ключ VAPID, нужный браузеру для подписки.
func (h *PushHandler) Key(w http.ResponseWriter, r *http.Request) {
	key, err := h.Service.PublicKey()
	if err != nil {
		utils.Error(w, http.StatusServiceUnavailable, "PUSH_NOT_AVAILABLE")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"key": key})
}

// Subscribe сохраняет подписку этого браузера или устройства.
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var sub push.Subscription
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&sub); err != nil ||
		!strings.HasPrefix(sub.Endpoint, "https://") || sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_SUBSCRIPTION")
		return
	}
	if err := h.Service.Subscribe(middleware.GetUserID(r), sub, r.UserAgent(), r.Header.Get("Origin")); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.Success(w)
}

// Test создаёт настоящее уведомление по задаче пользователя (оно приходит в
// колокольчик) и сразу отправляет его push на все устройства пользователя:
// нажатие на push открывает задачу и отмечает уведомление прочитанным.
// Возвращает ответы сервисов доставки по устройствам.
func (h *PushHandler) Test(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	n, title, body, link, err := h.Notifications.Test(userID)
	if errors.Is(err, notify.ErrNoIssue) {
		utils.Error(w, http.StatusConflict, "NO_TASK_FOR_TEST")
		return
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "PUSH_TEST_FAILED")
		return
	}
	results := h.Service.Test(userID, push.Message{Title: title, Body: body, URL: link, Tag: fmt.Sprintf("n%d", n.ID), Test: true})
	utils.JSON(w, http.StatusOK, map[string]interface{}{"results": results, "task_id": n.EntityID})
}

// Unsubscribe удаляет подписку этого браузера или устройства.
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil || body.Endpoint == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_SUBSCRIPTION")
		return
	}
	if err := h.Service.Unsubscribe(middleware.GetUserID(r), body.Endpoint); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.Success(w)
}
