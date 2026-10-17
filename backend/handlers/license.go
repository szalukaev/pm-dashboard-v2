package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"pm-dashboard/license"
	"pm-dashboard/utils"
)

// LicenseHandler — HTTP-часть лицензии: её состояние для каждого
// пользователя (плашка) и экран активации для администраторов. Вся логика —
// в пакете license.
type LicenseHandler struct {
	License *license.Service
	// OnActivated выполняется после успешной активации (начать сбор данных).
	OnActivated func()
}

// GetStatus возвращает состояние лицензии. Он открыт всем — плашке и экрану
// «не активирована» он нужен раньше, чем заработает что-либо ещё, — и
// потому сообщает только состояние и конец льготного периода.
func (h *LicenseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.License.Status()
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"state":         status.State,
		"grace_ends_at": status.GraceEndsAt,
	})
}

// GetDetails возвращает всё, что показывает экран активации: полное
// состояние, аппаратный отпечаток для отправки поставщику и журнал.
func (h *LicenseHandler) GetDetails(w http.ResponseWriter, r *http.Request) {
	// Экран открывают, чтобы увидеть текущее состояние, а не часовой давности
	status := h.License.Check()

	hwidText, hwidAvailable := "", true
	hwid, err := h.License.HWID()
	if err != nil {
		hwidAvailable = false
	} else {
		hwidText = hwid.String()
	}

	events, err := h.License.Events(50)
	if err != nil {
		events = []license.Event{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"status":         status,
		"hwid":           hwidText,
		"hwid_available": hwidAvailable,
		"events":         events,
	})
}

// Activate сохраняет токен лицензии.
func (h *LicenseHandler) Activate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LicenseBlob string `json:"license_blob"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.LicenseBlob) == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	status, err := h.License.Activate(body.LicenseBlob)
	switch {
	case err == nil:
		if h.OnActivated != nil {
			h.OnActivated()
		}
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "status": status})
	case errors.Is(err, license.ErrMalformed):
		utils.Error(w, http.StatusBadRequest, "LICENSE_FORMAT_INVALID")
	case errors.Is(err, license.ErrSignature):
		utils.Error(w, http.StatusBadRequest, "LICENSE_SIGNATURE_INVALID")
	case errors.Is(err, license.ErrHWIDMismatch):
		utils.Error(w, http.StatusBadRequest, "LICENSE_HWID_MISMATCH")
	case errors.Is(err, license.ErrHWIDUnavailable):
		utils.Error(w, http.StatusServiceUnavailable, "HWID_UNAVAILABLE")
	default:
		utils.Error(w, http.StatusInternalServerError, "SAVE_FAILED")
	}
}
