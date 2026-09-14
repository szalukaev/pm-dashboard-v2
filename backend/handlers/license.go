package handlers

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"pm-dashboard/utils"
)

type LicenseHandler struct {
	DB *sql.DB
}

// Vendor public key — hardcoded, not from config (ТЗ requirement)
var vendorPublicKey = ed25519.PublicKey{} // Set at build time via -ldflags

type LicensePayload struct {
	Client         string   `json:"client"`
	HWIDComponents struct {
		MachineIDHash  string `json:"machine_id_hash"`
		BoardSerialHash string `json:"board_serial_hash"`
	} `json:"hwid_components"`
	Edition         string   `json:"edition"`
	Features        []string `json:"features"`
	GracePeriodDays int      `json:"grace_period_days"`
	IssuedAt        string   `json:"issued_at"`
}

type LicenseStatus struct {
	IsActive       bool       `json:"is_active"`
	IsGracePeriod  bool       `json:"is_grace_period"`
	IsReadOnly     bool       `json:"is_read_only"`
	Client         string     `json:"client,omitempty"`
	Edition        string     `json:"edition,omitempty"`
	GraceEndsAt    *time.Time `json:"grace_ends_at,omitempty"`
	ActivatedAt    *time.Time `json:"activated_at,omitempty"`
	LastCheckOK    *time.Time `json:"last_check_ok,omitempty"`
}

func (h *LicenseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.checkLicense()
	utils.JSON(w, http.StatusOK, status)
}

func (h *LicenseHandler) Activate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LicenseBlob string `json:"license_blob"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LicenseBlob == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Parse token: base64url(payload).base64url(signature)
	parts := strings.SplitN(body.LicenseBlob, ".", 2)
	if len(parts) != 2 {
		utils.Error(w, http.StatusBadRequest, "INVALID_LICENSE_FORMAT")
		return
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_LICENSE_PAYLOAD")
		return
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != ed25519.SignatureSize {
		utils.Error(w, http.StatusBadRequest, "INVALID_LICENSE_SIGNATURE")
		return
	}

	// Verify signature
	if len(vendorPublicKey) == 0 {
		utils.Error(w, http.StatusInternalServerError, "LICENSE_VERIFICATION_NOT_CONFIGURED")
		return
	}

	if !ed25519.Verify(vendorPublicKey, payloadBytes, signature) {
		utils.Error(w, http.StatusForbidden, "LICENSE_SIGNATURE_INVALID")
		return
	}

	// Parse payload
	var payload LicensePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		utils.Error(w, http.StatusBadRequest, "LICENSE_PAYLOAD_INVALID")
		return
	}

	// Read HWID
	hwidHash := readHWID()

	// Store license
	h.DB.Exec(`DELETE FROM licenses`)
	h.DB.Exec(`INSERT INTO licenses (license_blob, hwid_hash, first_activated_at, last_check_ok_at)
		VALUES ($1, $2, NOW(), NOW())`, body.LicenseBlob, hwidHash)

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"client":  payload.Client,
		"edition": payload.Edition,
	})
}

func (h *LicenseHandler) checkLicense() LicenseStatus {
	var blob, hwidHash string
	var firstActivated, lastCheckOK, graceStarted *time.Time

	err := h.DB.QueryRow(`SELECT license_blob, hwid_hash, first_activated_at, last_check_ok_at, grace_started_at
		FROM licenses ORDER BY id DESC LIMIT 1`).Scan(&blob, &hwidHash, &firstActivated, &lastCheckOK, &graceStarted)

	if err == sql.ErrNoRows {
		return LicenseStatus{IsActive: false}
	}
	if err != nil {
		return LicenseStatus{IsActive: false}
	}

	// Parse and verify
	parts := strings.SplitN(blob, ".", 2)
	if len(parts) != 2 {
		return LicenseStatus{IsActive: false, IsGracePeriod: true}
	}

	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	signature, _ := base64.RawURLEncoding.DecodeString(parts[1])

	if len(vendorPublicKey) > 0 && !ed25519.Verify(vendorPublicKey, payloadBytes, signature) {
		return LicenseStatus{IsActive: false, IsGracePeriod: true}
	}

	var payload LicensePayload
	json.Unmarshal(payloadBytes, &payload)

	// Check HWID (1 of 2 match)
	currentHWID := readHWID()
	hwidMatch := currentHWID == hwidHash
	if !hwidMatch {
		// Check individual components if available
		if payload.HWIDComponents.MachineIDHash != "" {
			hwidComponent := readMachineID()
			hwidMatch = hwidComponent == payload.HWIDComponents.MachineIDHash
		}
	}

	if !hwidMatch {
		// Start grace period if not already started
		if graceStarted == nil {
			now := time.Now()
			h.DB.Exec("UPDATE licenses SET grace_started_at = $1 WHERE license_blob = $2", now, blob)
			graceStarted = &now
		}
		graceEnd := graceStarted.Add(time.Duration(payload.GracePeriodDays) * 24 * time.Hour)
		if time.Now().After(graceEnd) {
			return LicenseStatus{IsActive: false, IsReadOnly: true, Client: payload.Client, Edition: payload.Edition}
		}
		return LicenseStatus{IsActive: false, IsGracePeriod: true, GraceEndsAt: &graceEnd, Client: payload.Client, Edition: payload.Edition}
	}

	// All good — update last check
	h.DB.Exec("UPDATE licenses SET last_check_ok_at = NOW(), grace_started_at = NULL WHERE license_blob = $1", blob)

	return LicenseStatus{
		IsActive:    true,
		Client:      payload.Client,
		Edition:     payload.Edition,
		ActivatedAt: firstActivated,
		LastCheckOK: lastCheckOK,
	}
}

func readHWID() string {
	data, err := os.ReadFile("/etc/pmdashboard/hwid-source")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readMachineID() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
