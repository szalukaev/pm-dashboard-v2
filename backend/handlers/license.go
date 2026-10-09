package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"pm-dashboard/license"
	"pm-dashboard/utils"
)

// LicenseHandler is the HTTP side of the license: its state for every user
// (the banner), and the activation screen for administrators. All the logic
// lives in the license package.
type LicenseHandler struct {
	License *license.Service
	// OnActivated runs after a successful activation (start collecting data).
	OnActivated func()
}

// GetStatus returns the state of the license. It is public — the banner and
// the "not activated" screen need it before anything else works — and so
// tells only the state and the end of the grace period.
func (h *LicenseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.License.Status()
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"state":         status.State,
		"grace_ends_at": status.GraceEndsAt,
	})
}

// GetDetails returns everything the activation screen shows: the full
// status, the hardware fingerprint to send to the vendor and the journal.
func (h *LicenseHandler) GetDetails(w http.ResponseWriter, r *http.Request) {
	// The screen is opened to see the current state, not an hour-old one
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

// Activate stores a license token.
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
