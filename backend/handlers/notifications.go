package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type NotificationsHandler struct {
	DB *sql.DB
}

// GetNotificationSettings returns user notification preferences
func (h *NotificationsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var settings struct {
		ToastsEnabled      bool `json:"toasts_enabled"`
		OverdueAlerts      bool `json:"overdue_alerts"`
		SoundEnabled       bool `json:"sound_enabled"`
		EmailNotifications bool `json:"email_notifications"`
	}

	var notifJSON []byte
	err := h.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='notification_config'`).Scan(&notifJSON)
	if err == nil && len(notifJSON) > 0 {
		json.Unmarshal(notifJSON, &settings)
	} else {
		settings.ToastsEnabled = true
		settings.OverdueAlerts = true
	}

	// Override with user settings
	var userNotifs []byte
	h.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='user_notif_' || $1`, userID).Scan(&userNotifs)
	if len(userNotifs) > 0 {
		var userSettings struct {
			ToastsEnabled *bool `json:"toasts_enabled"`
			OverdueAlerts *bool `json:"overdue_alerts"`
		}
		json.Unmarshal(userNotifs, &userSettings)
		if userSettings.ToastsEnabled != nil {
			settings.ToastsEnabled = *userSettings.ToastsEnabled
		}
		if userSettings.OverdueAlerts != nil {
			settings.OverdueAlerts = *userSettings.OverdueAlerts
		}
	}

	utils.JSON(w, http.StatusOK, settings)
}

// UpdateNotificationSettings updates user notification preferences
func (h *NotificationsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)

	data, _ := json.Marshal(body)
	h.DB.Exec(`INSERT INTO admin_settings (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value=$2, updated_at=NOW()`, "user_notif_"+utils.Itoa(userID), data)

	utils.Success(w)
}

// GetAlertThresholds returns configured alert thresholds
func (h *NotificationsHandler) GetAlertThresholds(w http.ResponseWriter, r *http.Request) {
	var thresholdsJSON []byte
	err := h.DB.QueryRow("SELECT value FROM admin_settings WHERE key='alert_thresholds'").Scan(&thresholdsJSON)
	if err == sql.ErrNoRows {
		// Default thresholds
		defaults := map[string]interface{}{
			"overdue_critical_days": 7,
			"overdue_warning_days":  3,
			"no_estimate_alert":     true,
			"high_priority_alert":   true,
		}
		utils.JSON(w, http.StatusOK, defaults)
		return
	}

	var thresholds map[string]interface{}
	json.Unmarshal(thresholdsJSON, &thresholds)
	utils.JSON(w, http.StatusOK, thresholds)
}

// UpdateAlertThresholds updates alert thresholds (admin only)
func (h *NotificationsHandler) UpdateAlertThresholds(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)

	data, _ := json.Marshal(body)
	h.DB.Exec(`INSERT INTO admin_settings (key, value, updated_at) VALUES ('alert_thresholds', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value=$1, updated_at=NOW()`, data)

	utils.Success(w)
}
