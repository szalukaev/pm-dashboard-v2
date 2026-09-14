package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

type SettingsHandler struct {
	DB *sql.DB
}

func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var settings struct {
		SelectedProjects          []int    `json:"selected_projects"`
		SelectedTeam              []int    `json:"selected_team"`
		Theme                     string   `json:"theme"`
		Language                  string   `json:"language"`
		TabOrder                  []string `json:"tab_order"`
		KanbanColumnOrderStatuses []string `json:"kanban_column_order_statuses"`
		KanbanColumnOrderUsers    []string `json:"kanban_column_order_users"`
		LastFilters               map[string]interface{} `json:"last_filters"`
		NotificationsEnabled      bool     `json:"notifications_enabled"`
	}

	var selectedProjects, selectedTeam, tabOrder, kanbanStatuses, kanbanUsers, lastFilters []byte
	var theme, language string
	var notificationsEnabled bool

	err := h.DB.QueryRow(`
		SELECT selected_projects, selected_team, theme, language, tab_order,
		       kanban_column_order_statuses, kanban_column_order_users,
		       last_filters, notifications_enabled
		FROM user_settings WHERE user_id = $1
	`, userID).Scan(
		&selectedProjects, &selectedTeam, &theme, &language,
		&tabOrder, &kanbanStatuses, &kanbanUsers, &lastFilters, &notificationsEnabled,
	)

	if err == sql.ErrNoRows {
		// Create default settings
		h.DB.Exec(`INSERT INTO user_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID)
		settings.Theme = "dark"
		settings.Language = "ru"
		settings.NotificationsEnabled = true
		utils.JSON(w, http.StatusOK, settings)
		return
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED")
		return
	}

	json.Unmarshal(selectedProjects, &settings.SelectedProjects)
	json.Unmarshal(selectedTeam, &settings.SelectedTeam)
	json.Unmarshal(tabOrder, &settings.TabOrder)
	json.Unmarshal(kanbanStatuses, &settings.KanbanColumnOrderStatuses)
	json.Unmarshal(kanbanUsers, &settings.KanbanColumnOrderUsers)
	json.Unmarshal(lastFilters, &settings.LastFilters)
	settings.Theme = theme
	settings.Language = language
	settings.NotificationsEnabled = notificationsEnabled

	utils.JSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Upsert settings
	_, err := h.DB.Exec(`
		INSERT INTO user_settings (user_id, updated_at) VALUES ($1, NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "SETTINGS_SAVE_FAILED")
		return
	}

	if v, ok := body["selected_projects"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET selected_projects = $1 WHERE user_id = $2", data, userID)
	}
	if v, ok := body["selected_team"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET selected_team = $1 WHERE user_id = $2", data, userID)
	}
	if v, ok := body["theme"]; ok {
		h.DB.Exec("UPDATE user_settings SET theme = $1 WHERE user_id = $2", v, userID)
	}
	if v, ok := body["language"]; ok {
		h.DB.Exec("UPDATE user_settings SET language = $1 WHERE user_id = $2", v, userID)
	}
	if v, ok := body["tab_order"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET tab_order = $1 WHERE user_id = $2", data, userID)
	}
	if v, ok := body["last_filters"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET last_filters = $1 WHERE user_id = $2", data, userID)
	}
	if v, ok := body["notifications_enabled"]; ok {
		h.DB.Exec("UPDATE user_settings SET notifications_enabled = $1 WHERE user_id = $2", v, userID)
	}
	if v, ok := body["kanban_column_order_statuses"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET kanban_column_order_statuses = $1 WHERE user_id = $2", data, userID)
	}
	if v, ok := body["kanban_column_order_users"]; ok {
		data, _ := json.Marshal(v)
		h.DB.Exec("UPDATE user_settings SET kanban_column_order_users = $1 WHERE user_id = $2", data, userID)
	}

	h.DB.Exec("UPDATE user_settings SET updated_at = NOW() WHERE user_id = $1", userID)
	utils.Success(w)
}
