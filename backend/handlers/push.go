package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"pm-dashboard/middleware"
	"pm-dashboard/push"
	"pm-dashboard/utils"
)

// PushHandler подписывает браузеры и устройства пользователя на push.
type PushHandler struct {
	Service *push.Service
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
