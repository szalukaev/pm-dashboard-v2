package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"pm-dashboard/config"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/middleware"
	"pm-dashboard/notify"
	"pm-dashboard/utils"
)

type SettingsHandler struct {
	DB     **sql.DB
	SQLite *config.SQLiteStore
	Source *manager.Manager
}

func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var settings struct {
		SelectedProjects          []int    `json:"selected_projects"`
		SelectedTeam              []int    `json:"selected_team"`
		Theme                     string   `json:"theme"`
		Language                  string   `json:"language"`
		TabOrder                  []string `json:"tab_order"`
		KanbanColumnOrderStatuses []string `json:"kanban_column_order_statuses"`
		KanbanColumnOrderUsers    []string `json:"kanban_column_order_users"`
		KanbanLastMode            string   `json:"kanban_last_mode"`
		KanbanLastProject         *int     `json:"kanban_last_project"`
		LastFilters              map[string]interface{} `json:"last_filters"`
		NotificationsEnabled      bool     `json:"notifications_enabled"`
		OverdueAlerts             bool     `json:"overdue_alerts"`
		DataSourceURL             string   `json:"data_source_url"`
		// Rows per page chosen for each table; a table not listed uses the default
		Pagination map[string]int `json:"pagination"`
		// Visible columns and their order chosen for each table
		TableColumns json.RawMessage `json:"table_columns"`
		// Arrangement of the widgets of the Analytics tab; {} is the standard one
		DashboardLayout json.RawMessage `json:"dashboard_layout"`
		// Действующие правила уведомлений: сохранённые поверх стандартных
		AlertRules notify.Rules `json:"alert_rules"`
		// Регулярная сводка на почту
		EmailReport ReportSettings `json:"email_report"`
		// Пороги индикаторов «Аналитики»
		IndicatorThresholds Thresholds `json:"indicator_thresholds"`
	}
	settings.AlertRules = notify.Defaults()
	settings.EmailReport = defaultReportSettings()
	settings.IndicatorThresholds = defaultThresholds()
	settings.Pagination = map[string]int{}
	settings.TableColumns = json.RawMessage("{}")
	settings.DashboardLayout = json.RawMessage("{}")

	var selectedProjects, selectedTeam, tabOrder, kanbanStatuses, kanbanUsers, lastFilters, pagination, tableColumns, dashboardLayout, alertRules, emailReport, thresholds []byte
	var theme, language string
	var notificationsEnabled, overdueAlerts bool

	err := (*h.DB).QueryRow(`
		SELECT selected_projects, selected_team, theme, language, tab_order,
		       kanban_column_order_statuses, kanban_column_order_users,
		       last_filters, notifications_enabled, overdue_alerts,
		       kanban_last_mode, kanban_last_project, pagination, table_columns, dashboard_layout, alert_rules, email_report, indicator_thresholds
		FROM user_settings WHERE user_id = $1
	`, userID).Scan(
		&selectedProjects, &selectedTeam, &theme, &language,
		&tabOrder, &kanbanStatuses, &kanbanUsers, &lastFilters, &notificationsEnabled, &overdueAlerts,
		&settings.KanbanLastMode, &settings.KanbanLastProject, &pagination, &tableColumns, &dashboardLayout, &alertRules, &emailReport, &thresholds,
	)

	if err == sql.ErrNoRows {
		// Create default settings
		if _, err := (*h.DB).Exec(`INSERT INTO user_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
			return
		}
		settings.Theme = "dark"
		settings.Language = "ru"
		settings.KanbanLastMode = "users"
		settings.NotificationsEnabled = true
		settings.OverdueAlerts = false
		if h.SQLite != nil {
			if v, _ := h.SQLite.Get("redmine_url"); v != "" {
				settings.DataSourceURL = v
			}
		}
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
	json.Unmarshal(pagination, &settings.Pagination)
	if settings.Pagination == nil {
		settings.Pagination = map[string]int{}
	}
	if json.Valid(tableColumns) && len(tableColumns) > 0 {
		settings.TableColumns = tableColumns
	}
	if json.Valid(dashboardLayout) && len(dashboardLayout) > 0 {
		settings.DashboardLayout = dashboardLayout
	}
	settings.AlertRules = notify.Effective(alertRules)
	settings.EmailReport = parseReportSettings(emailReport)
	settings.IndicatorThresholds = parseThresholds(thresholds)
	settings.Theme = theme
	settings.Language = language
	settings.NotificationsEnabled = notificationsEnabled
	settings.OverdueAlerts = overdueAlerts

	// Get Redmine URL from SQLite config
	if h.SQLite != nil {
		if v, _ := h.SQLite.Get("redmine_url"); v != "" {
			settings.DataSourceURL = v
		}
	}

	utils.JSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Upsert settings
	_, err := (*h.DB).Exec(`
		INSERT INTO user_settings (user_id, updated_at) VALUES ($1, NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "SETTINGS_SAVE_FAILED")
		return
	}

	// The user narrows what the administrator allows and can never widen it:
	// anything outside of the rights is dropped from the selection.
	scope := middleware.GetScope(r)
	if v, ok := body["selected_projects"]; ok {
		requested := intList(v)
		allowed := scope.FilterProjects(requested)
		if len(allowed) != len(uniqueIntCount(requested)) {
			slog.Warn("Selection of projects outside of user rights dropped", "user", userID)
		}
		data, _ := json.Marshal(allowed)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET selected_projects = $1 WHERE user_id = $2", data, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		// Issues are synced only for selected projects: fetch the new ones
		// now instead of waiting for the next periodic sync.
		if h.Source != nil {
			h.Source.TriggerSync()
		}
	}
	if v, ok := body["selected_team"]; ok {
		requested := intList(v)
		allowed := scope.FilterTeam(requested)
		if len(allowed) != len(uniqueIntCount(requested)) {
			slog.Warn("Selection of team members outside of user rights dropped", "user", userID)
		}
		data, _ := json.Marshal(allowed)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET selected_team = $1 WHERE user_id = $2", data, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["theme"]; ok {
		if _, err := (*h.DB).Exec("UPDATE user_settings SET theme = $1 WHERE user_id = $2", v, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["language"]; ok {
		if _, err := (*h.DB).Exec("UPDATE user_settings SET language = $1 WHERE user_id = $2", v, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["tab_order"]; ok {
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET tab_order = $1 WHERE user_id = $2", data, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["last_filters"]; ok {
		// {"<page or widget>": its filters}; the sent ones replace their saved
		// values, the filters of the other pages stay
		if _, isObject := v.(map[string]interface{}); !isObject {
			utils.Error(w, http.StatusBadRequest, "INVALID_FILTERS")
			return
		}
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET last_filters = COALESCE(last_filters, '{}'::jsonb) || $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["notifications_enabled"]; ok {
		if _, err := (*h.DB).Exec("UPDATE user_settings SET notifications_enabled = $1 WHERE user_id = $2", v, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["overdue_alerts"]; ok {
		if _, err := (*h.DB).Exec("UPDATE user_settings SET overdue_alerts = $1 WHERE user_id = $2", v, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["kanban_column_order_statuses"]; ok {
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_statuses = $1 WHERE user_id = $2", data, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["kanban_column_order_users"]; ok {
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_users = $1 WHERE user_id = $2", data, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["pagination"]; ok {
		// {"table": rows per page}; the sent tables are merged into the saved ones
		sizes, isObject := v.(map[string]interface{})
		if !isObject {
			utils.Error(w, http.StatusBadRequest, "INVALID_PAGINATION")
			return
		}
		for table, size := range sizes {
			n, isNumber := size.(float64)
			if !isNumber || len(table) > 50 || !validPageSize(int(n)) || n != float64(int(n)) {
				utils.Error(w, http.StatusBadRequest, "INVALID_PAGINATION")
				return
			}
		}
		data, _ := json.Marshal(sizes)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET pagination = pagination || $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["table_columns"]; ok {
		// {"table": {"visible": [...], "order": [...]}}; the sent tables
		// replace their saved setups, the others stay
		if !validTableColumns(v) {
			utils.Error(w, http.StatusBadRequest, "INVALID_TABLE_COLUMNS")
			return
		}
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET table_columns = table_columns || $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["alert_rules"]; ok {
		// Присланные правила заменяют свои сохранённые, остальные остаются
		raw, _ := json.Marshal(v)
		rules := notify.Rules{}
		if json.Unmarshal(raw, &rules) != nil || !notify.Valid(rules) {
			utils.Error(w, http.StatusBadRequest, "INVALID_ALERT_RULES")
			return
		}
		data, _ := json.Marshal(rules)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET alert_rules = alert_rules || $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["email_report"]; ok {
		raw, _ := json.Marshal(v)
		report := defaultReportSettings()
		if json.Unmarshal(raw, &report) != nil || !report.valid() {
			utils.Error(w, http.StatusBadRequest, "INVALID_EMAIL_REPORT")
			return
		}
		data, _ := json.Marshal(report)
		// Отсчёт срока идёт от момента включения: первая сводка уходит в
		// ближайший срок по расписанию, а не сразу и не за прошедший
		if _, err := (*h.DB).Exec(`UPDATE user_settings SET email_report = $1::jsonb,
			email_report_sent_at = CASE WHEN $2 THEN COALESCE(email_report_sent_at, NOW()) ELSE NULL END
			WHERE user_id = $3`, string(data), report.Enabled, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["indicator_thresholds"]; ok {
		raw, _ := json.Marshal(v)
		limits := defaultThresholds()
		if json.Unmarshal(raw, &limits) != nil || !limits.valid() {
			utils.Error(w, http.StatusBadRequest, "INVALID_THRESHOLDS")
			return
		}
		data, _ := json.Marshal(limits)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET indicator_thresholds = $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["dashboard_layout"]; ok {
		// The whole arrangement is sent and replaces the saved one
		if !validDashboardLayout(v) {
			utils.Error(w, http.StatusBadRequest, "INVALID_DASHBOARD_LAYOUT")
			return
		}
		data, _ := json.Marshal(v)
		if _, err := (*h.DB).Exec("UPDATE user_settings SET dashboard_layout = $1::jsonb WHERE user_id = $2", string(data), userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if v, ok := body["kanban_last_mode"]; ok {
		mode, _ := v.(string)
		if mode != "users" && mode != "statuses" {
			utils.Error(w, http.StatusBadRequest, "INVALID_MODE")
			return
		}
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_last_mode = $1 WHERE user_id = $2", mode, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if v, ok := body["kanban_last_project"]; ok {
		// A number selects the project, null clears the choice
		var project interface{}
		if id, isNumber := v.(float64); isNumber {
			project = int(id)
		} else if v != nil {
			utils.Error(w, http.StatusBadRequest, "INVALID_PROJECT")
			return
		}
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_last_project = $1 WHERE user_id = $2", project, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if _, err := (*h.DB).Exec("UPDATE user_settings SET updated_at = NOW() WHERE user_id = $1", userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.Success(w)
}
