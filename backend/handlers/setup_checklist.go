package handlers

import (
	"net/http"

	"pm-dashboard/utils"
)

// Чек-лист первого запуска: какие шаги настройки системы уже пройдены.
// Показывается администратору над меню настроек.

// checklistItem — один шаг. Required: без него система не работает;
// остальные шаги — рекомендации.
type checklistItem struct {
	Key      string `json:"key"`
	Done     bool   `json:"done"`
	Required bool   `json:"required"`
}

// checklistFacts — то, что известно о состоянии системы.
type checklistFacts struct {
	SourceOK bool // последняя завершённая синхронизация прошла
	Statuses bool // статусы загружены из источника
	Projects bool // проекты загружены из источника
	Users    bool // есть пользователи кроме первого администратора
	Rights   bool // хотя бы одному пользователю назначены права
	Schedule bool // администратор задавал расписание синхронизации
}

// buildChecklist раскладывает факты по шагам в том порядке, в каком их
// проходят при настройке.
func buildChecklist(f checklistFacts) []checklistItem {
	return []checklistItem{
		{Key: "source", Done: f.SourceOK, Required: true},
		{Key: "statuses", Done: f.Statuses, Required: true},
		{Key: "projects", Done: f.Projects, Required: true},
		{Key: "users", Done: f.Users, Required: true},
		{Key: "rights", Done: f.Rights},
		{Key: "schedule", Done: f.Schedule},
	}
}

func (h *AdminHandler) GetSetupChecklist(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	db := *h.DB
	var facts checklistFacts

	// Идущая синхронизация о связи ещё ничего не говорит: смотрим на
	// последнюю завершённую. Прошедшая с ошибками по части проектов — тоже связь.
	var status string
	db.QueryRow(`SELECT status FROM collection_log WHERE status <> 'running' ORDER BY started_at DESC LIMIT 1`).Scan(&status)
	facts.SourceOK = status == "success" || status == "partial"

	db.QueryRow(`SELECT EXISTS(SELECT 1 FROM statuses)`).Scan(&facts.Statuses)
	db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects)`).Scan(&facts.Projects)
	db.QueryRow(`SELECT COUNT(*) > 1 FROM users`).Scan(&facts.Users)
	// Права назначены: у пользователя есть шаблон роли, своя настройка или группа
	db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM users u WHERE u.role <> 'admin' AND (
			u.role_template_id IS NOT NULL
			OR EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id)
			OR EXISTS(SELECT 1 FROM group_members gm WHERE gm.user_id = u.id)))`).Scan(&facts.Rights)

	// Расписание считается настроенным, когда интервал или время полной
	// синхронизации сохраняли: до этого действуют значения по умолчанию
	for _, key := range []string{"sync_interval_minutes", "full_sync_time"} {
		if v, _ := h.SQLite.Get(key); v != "" {
			facts.Schedule = true
		}
	}

	items := buildChecklist(facts)
	done := 0
	for _, item := range items {
		if item.Done {
			done++
		}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "done": done, "total": len(items),
	})
}
