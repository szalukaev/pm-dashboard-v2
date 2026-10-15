package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/config"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/db"
	"pm-dashboard/handlers"
	"pm-dashboard/jobs"
	"pm-dashboard/license"
	"pm-dashboard/mail"
	"pm-dashboard/middleware"
	"pm-dashboard/notify"
	"pm-dashboard/push"
	"pm-dashboard/secrets"
	"pm-dashboard/snapshots"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	utils.InitLogger(logLevel)
	slog.Info("Starting PM Dashboard V3...")

	// Хранилище конфигурации в SQLite
	dataDir := "/data"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dataDir = d
	}
	sqliteStore, err := config.NewSQLiteStore(dataDir)
	if err != nil {
		slog.Error("Failed to init SQLite config", "error", err)
		os.Exit(1)
	}
	defer sqliteStore.Close()

	cfg := sqliteStore.LoadAppConfig()

	// Подключение к PostgreSQL, если оно настроено
	var pgDB *sql.DB
	if cfg.DSN != "" {
		pgDB, err = db.Connect(cfg.DSN)
		// Параметры базы меняли на экране «Подключения»: прежняя строка
		// хранится до первого удачного старта с новой. Если новая база не
		// отвечает — возвращаемся к прежней, иначе система осталась бы без базы.
		if prev, _ := sqliteStore.Get("db_dsn_prev"); prev != "" {
			if err != nil && prev != cfg.DSN {
				if prevDB, prevErr := db.Connect(prev); prevErr == nil {
					slog.Error("New database is not available, returned to the previous connection", "error", err)
					if err := sqliteStore.Set("db_dsn", prev); err != nil {
						slog.Warn("Operation failed", "at", "main", "error", err)
					}
					pgDB, err = prevDB, nil
				}
			}
			if err == nil {
				if err := sqliteStore.Set("db_dsn_prev", ""); err != nil {
					slog.Warn("Operation failed", "at", "main", "error", err)
				}
			}
		}
		if err != nil {
			slog.Warn("PostgreSQL not available", "error", err)
		} else {
			defer pgDB.Close()
			// На несовместимой схеме запросы не обслуживаются.
			if err := db.RunMigrations(pgDB); err != nil {
				slog.Error("Migrations failed, refusing to start", "error", err)
				os.Exit(1)
			}
		}
	}

	// Хранилище сессий (Redis или запасное в памяти)
	redisURL := os.Getenv("REDIS_URL")
	sessionStore := db.NewSessionStore(redisURL)
	defer sessionStore.Close()

	// Лицензия: её состоянием владеет один сервис — проверка при запуске и
	// каждый час. Аппаратный отпечаток берётся из файла, собранного на хосте.
	hwidPath := license.DefaultHWIDPath
	if p := os.Getenv("HWID_SOURCE"); p != "" {
		hwidPath = p
	}
	licenseSvc := license.New(&pgDB, hwidPath)
	go licenseSvc.Run(context.Background())

	// Журнал аудита
	var auditMW *middleware.AuditMiddleware
	if pgDB != nil {
		auditMW = &middleware.AuditMiddleware{DB: &pgDB}
	}

	// Обработчики
	setupH := &handlers.SetupHandler{SQLite: sqliteStore, PGDB: &pgDB}
	authH := &handlers.AuthHandler{DB: &pgDB, Sessions: sessionStore, Audit: auditMW}
	settingsH := &handlers.SettingsHandler{DB: &pgDB, SQLite: sqliteStore}
	taskH := &handlers.TaskHandler{DB: &pgDB}
	analyticsH := &handlers.AnalyticsHandler{DB: &pgDB}
	kanbanH := &handlers.KanbanHandler{DB: &pgDB}
	sprintH := &handlers.SprintHandler{DB: &pgDB}
	// Счета формируются по шаблону, загруженному администратором, или по
	// шаблону из поставки
	invoiceTemplates := &handlers.InvoiceTemplates{DB: &pgDB, Dir: dataDir, BuiltIn: "invoice/invoice_template.docx"}
	paymentsH := &handlers.PaymentsHandler{DB: &pgDB, Templates: invoiceTemplates}
	invoiceTemplateH := &handlers.InvoiceTemplateHandler{DB: &pgDB, Templates: invoiceTemplates}
	adminH := &handlers.AdminHandler{DB: &pgDB, SQLite: sqliteStore}
	licenseH := &handlers.LicenseHandler{License: licenseSvc}
	wsHub := handlers.NewWSHub()

	// Клиент источника данных и фоновая синхронизация. Менеджер на ходу
	// перечитывает параметры подключения и интервал, поэтому цикл работает,
	// даже пока источник или база ещё не настроены (мастер настройки).
	// Данные собираются только при действующей лицензии: не до активации
	// и не в режиме «только чтение».
	source := manager.New(sqliteStore, &pgDB, func() bool {
		return !licenseSvc.Usable() || licenseSvc.IsReadOnly()
	})
	taskH.Source = source
	kanbanH.Source = source
	adminH.Source = source
	// После смены параметров базы процесс завершается, и Docker поднимает его
	// заново (restart: unless-stopped). Пауза — чтобы ответ успел уйти клиенту.
	adminH.Restart = func() {
		go func() {
			time.Sleep(time.Second)
			slog.Warn("Restarting to apply the new database connection")
			os.Exit(0)
		}()
	}
	setupH.Source = source
	settingsH.Source = source
	authH.Source = source
	// Сбор данных начинается сразу после активации лицензии
	licenseH.OnActivated = func() {
		if err := source.TriggerSync(); err != nil {
			slog.Warn("Sync not started after license activation", "error", err)
		}
	}

	// Личные ключи API источника данных хранятся зашифрованными; с ними
	// изменения уходят в источник от имени самого пользователя.
	secretBox, err := secrets.Open(sqliteStore)
	if err != nil {
		slog.Error("Failed to open the secret store", "error", err)
		os.Exit(1)
	}
	authH.Secrets = secretBox
	userKeys := &handlers.UserKeys{DB: &pgDB, Source: source, Box: secretBox}
	taskH.Keys = userKeys
	kanbanH.Keys = userKeys
	authH.Keys = userKeys
	// «Права как в источнике данных»: в каких проектах там состоит каждый
	// пользователь, перечитывается перед каждой синхронизацией
	source.SetBeforeSync(userKeys.RefreshAllProjects)
	syncStatusH := &handlers.SyncStatusHandler{DB: &pgDB, Source: source}
	accessH := &handlers.AccessHandler{DB: &pgDB, Source: source, Sessions: sessionStore, Disconnect: wsHub.DisconnectUser}
	// Задачи синхронизируются по проектам, показанным пользователям в пределах их прав
	source.SetProjectSource((&access.Resolver{DB: &pgDB}).SyncProjects)

	// Обновление в реальном времени: синхронизация и правки пользователей сообщают об изменениях открытым страницам
	source.SetNotifier(wsHub.BroadcastEvent)

	// История: снимок дня после каждой синхронизации и ежедневная очистка
	// того, что старше срока хранения.
	jobSettings := &jobs.Settings{DB: &pgDB}
	cleaner := jobs.NewCleaner(&pgDB, jobSettings, jobs.NewStore(redisURL))
	cleaner.ReadOnly = func() bool { return !licenseSvc.Usable() || licenseSvc.IsReadOnly() }
	cleaner.Notify = wsHub.BroadcastEvent
	workersH := &handlers.WorkersHandler{DB: &pgDB, Source: source, Cleaner: cleaner, Settings: jobSettings, Events: wsHub.BroadcastEvent}
	// Уведомления: после синхронизации задачи сравниваются с последним
	// виденным состоянием, а правка задачи в самом дашборде сообщается сразу.
	notifier := &notify.Engine{DB: &pgDB, Send: wsHub.SendToUser, Scopes: (&access.Resolver{DB: &pgDB}).Scope}
	// Администраторам сообщается о лицензии в льготном периоде и в режиме
	// «только чтение»
	notifier.LicenseState = func() string {
		if state := licenseSvc.Status().State; state == license.Grace || state == license.ReadOnly {
			return string(state)
		}
		return ""
	}
	// Сбой синхронизации события до уведомлений не доходит сам: менеджер
	// сообщает о нём только открытым страницам
	source.SetNotifier(func(event string, data interface{}) {
		wsHub.BroadcastEvent(event, data)
		if status, ok := data.(manager.SyncStatus); ok && event == manager.EventSyncStatus && status.Status == "error" {
			go notifier.SyncFailed(time.Now())
		}
	})
	handlers.IssueEdited = notifier.IssueEdited
	notificationsH := &handlers.NotificationsHandler{DB: &pgDB}
	source.SetAfterSync(func() {
		if pgDB == nil {
			return
		}
		// Уведомления не зависят от снимка дня: сбой снимка их не отменяет
		defer notifier.AfterSync(time.Now())
		counts, err := snapshots.Record(pgDB, time.Now())
		if err != nil {
			slog.Error("Failed to record the snapshot of the day", "error", err)
			return
		}
		slog.Info("Snapshot of the day recorded", "members", counts.Members, "issues_changed", counts.Issues)
	})
	// Почта: настройки SMTP у администратора и рассылка — письма с
	// уведомлениями и регулярные сводки
	mailStore := &mail.Store{DB: &pgDB, Box: secretBox}
	// Push: те же уведомления, что в колокольчике, — на устройства пользователя.
	// Контакт отправителя — адрес дашборда из настроек почты, если он задан
	pushSvc := push.New(&pgDB, func() string {
		if cfg, err := mailStore.Load(); err == nil && strings.HasPrefix(cfg.BaseURL, "https://") {
			return cfg.BaseURL
		}
		return ""
	})
	notifier.Push = func(userID int, title, body, link, tag string) {
		pushSvc.Notify(userID, push.Message{Title: title, Body: body, URL: link, Tag: tag})
	}
	pushH := &handlers.PushHandler{Service: pushSvc}
	mailH := &handlers.MailHandler{Mail: mailStore}
	mailer := &jobs.Mailer{DB: &pgDB, Settings: jobSettings, Store: jobs.NewStore(redisURL), Mail: mailStore, Notify: wsHub.BroadcastEvent}
	reportH := &handlers.ReportHandler{DB: &pgDB, Mail: mailStore, Scopes: (&access.Resolver{DB: &pgDB}).Scope}
	mailer.Reports = reportH.SendDue
	workersH.Mailer = mailer
	go mailer.Run(context.Background())
	go cleaner.Run(context.Background())
	taskH.Events = wsHub.BroadcastEvent
	kanbanH.Events = wsHub.BroadcastEvent
	go source.Run(context.Background())

	// Маршрутизатор
	r := mux.NewRouter()

	// CORS
	r.Use(middleware.CORSMiddleware)

	// Защитные заголовки (после CORS, чтобы не перезаписать ACAO)
	r.Use(middleware.SecurityHeaders)

	// Предел размера тела JSON-запроса (1 МиБ)
	r.Use(middleware.LimitJSONBody)

	// Мастер настройки (без входа)
	r.HandleFunc("/api/setup/status", setupH.Status).Methods("GET")
	r.HandleFunc("/api/setup/database/test", setupH.TestDatabase).Methods("POST")
	r.HandleFunc("/api/setup/database", setupH.SaveDatabase).Methods("POST")
	r.HandleFunc("/api/setup/datasource/test", setupH.TestDataSource).Methods("POST")
	r.HandleFunc("/api/setup/datasource", setupH.SaveDataSource).Methods("POST")
	r.HandleFunc("/api/setup/admin", setupH.CreateAdmin).Methods("POST")

	// Состояние сервера (открыто всем, сведения о версии)
	r.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version":     "3.0.0",
			"status":      "ok",
			"is_setup":    sqliteStore.IsSetupComplete(),
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}).Methods("GET")

	// Состояние лицензии открыто всем (плашка у каждого пользователя); активация — только администратору, см. ниже
	r.HandleFunc("/api/license/status", licenseH.GetStatus).Methods("GET")

	// Вход и выход (для входа сессия не нужна)
	r.HandleFunc("/api/auth/login", authH.Login).Methods("POST")

	// Маршруты, требующие входа
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.RequireAuth(sessionStore, &pgDB))

	// Отладочное логирование: запросы пользователей, которым его включил
	// администратор, пишутся в журнал. После авторизации — пользователь уже известен.
	debugLog := middleware.NewDebugLog(&pgDB)
	api.Use(debugLog.Log)
	debugH := &handlers.DebugHandler{DB: &pgDB, Log: debugLog}

	// Аудит — записывает все изменяющие запросы
	if auditMW != nil {
		api.Use(auditMW.Log)
	}

	// Состояние лицензии: до активации — только экран активации, в режиме
	// «только чтение» — никаких изменений
	api.Use(licenseSvc.Guard)

	// Вкладка, скрытая администратором от пользователя, закрыта и здесь, а
	// не только в меню. Карточки задач и справочники остаются открытыми:
	// ими пользуются другие вкладки.
	api.Use(middleware.RequireTabs([]middleware.TabRule{
		{Path: "/api/tasks", Exact: true, Tab: "tasks"},
		{Path: "/api/analytics", Tab: "analytics"},
		{Path: "/api/kanban", Tab: "kanban"},
		{Path: "/api/sprints", Tab: "sprint"},
		{Path: "/api/organizations", Tab: "payments"},
		{Path: "/api/contracts", Tab: "payments"},
		{Path: "/api/payments", Tab: "payments"},
	}))

	api.Handle("/license/details", middleware.RequireAdmin(http.HandlerFunc(licenseH.GetDetails))).Methods("GET")
	api.Handle("/license/activate", middleware.RequireAdmin(http.HandlerFunc(licenseH.Activate))).Methods("POST")

	api.HandleFunc("/auth/logout", authH.Logout).Methods("POST")
	api.HandleFunc("/auth/me", authH.Me).Methods("GET")
	api.HandleFunc("/auth/me", authH.UpdateMe).Methods("PUT")
	api.HandleFunc("/auth/change-password", authH.ChangePassword).Methods("POST")
	api.HandleFunc("/auth/avatar", authH.UploadAvatar).Methods("POST")
	api.HandleFunc("/auth/avatar", authH.DeleteAvatar).Methods("DELETE")
	api.HandleFunc("/member-avatars", authH.MemberAvatars).Methods("GET")
	// Привязка к учётной записи пользователя в источнике данных: только своя, маршрута для администратора нет
	api.HandleFunc("/auth/source-token", authH.SetSourceToken).Methods("PUT")
	api.HandleFunc("/auth/source-token", authH.ClearSourceToken).Methods("DELETE")
	api.HandleFunc("/sync/status", syncStatusH.GetStatus).Methods("GET")
	api.HandleFunc("/settings", settingsH.GetSettings).Methods("GET")
	api.HandleFunc("/settings", settingsH.UpdateSettings).Methods("PUT")
	// Пробная сводка на почту — по настройке из запроса, до сохранения
	api.HandleFunc("/settings/email-report/test", reportH.SendTest).Methods("POST")

	// Задачи
	api.HandleFunc("/tasks", taskH.ListTasks).Methods("GET")
	api.HandleFunc("/tasks/categories", taskH.GetCategories).Methods("GET")
	api.HandleFunc("/tasks/project-categories", taskH.GetProjectCategories).Methods("GET")
	api.HandleFunc("/tasks/projects", taskH.GetProjects).Methods("GET")
	api.HandleFunc("/tasks/members", taskH.GetMembers).Methods("GET")
	api.HandleFunc("/tasks/statuses", taskH.GetStatuses).Methods("GET")
	api.HandleFunc("/tasks/priorities", taskH.GetPriorities).Methods("GET")
	api.HandleFunc("/tasks/{id}/details", taskH.GetTaskDetails).Methods("GET")
	api.HandleFunc("/tasks/{id}/attachments/{attachment_id}", taskH.GetTaskAttachment).Methods("GET")
	api.HandleFunc("/tasks/{id}/comments", taskH.GetComments).Methods("GET")
	api.Handle("/tasks/{id}/comments", middleware.RequireWrite(http.HandlerFunc(taskH.AddComment))).Methods("POST")
	api.HandleFunc("/tasks/{id}", taskH.GetTask).Methods("GET")
	api.Handle("/tasks/{id}", middleware.RequireWrite(http.HandlerFunc(taskH.UpdateTask))).Methods("PUT")

	// Аналитика
	api.HandleFunc("/analytics/stats", analyticsH.GetStats).Methods("GET")
	api.HandleFunc("/analytics/team-load", analyticsH.GetTeamLoad).Methods("GET")
	api.HandleFunc("/analytics/distribution", analyticsH.GetDistribution).Methods("GET")
	api.HandleFunc("/analytics/deadlines", analyticsH.GetDeadlines).Methods("GET")
	api.HandleFunc("/analytics/hours", analyticsH.GetHours).Methods("GET")
	api.HandleFunc("/analytics/velocity", analyticsH.GetVelocity).Methods("GET")
	api.HandleFunc("/analytics/health", analyticsH.GetHealth).Methods("GET")
	api.HandleFunc("/analytics/gantt", analyticsH.GetGantt).Methods("GET")
	api.HandleFunc("/analytics/tasks", analyticsH.GetTasks).Methods("GET")
	api.HandleFunc("/analytics/thresholds", analyticsH.GetThresholds).Methods("GET")

	// Kanban
	api.HandleFunc("/kanban/board", kanbanH.GetBoard).Methods("GET")
	api.Handle("/kanban/move", middleware.RequireWrite(http.HandlerFunc(kanbanH.MoveCard))).Methods("PUT")
	api.HandleFunc("/kanban/column-order", kanbanH.SaveColumnOrder).Methods("PUT")

	// Спринты
	api.HandleFunc("/sprints", sprintH.ListSprints).Methods("GET")
	// Каждое действие со спринтами — отдельное право. Изменение спринта
	// проверяется внутри обработчика: это «edit» или «close» в зависимости
	// от того, что меняется. «Только чтение» закрывает все изменения
	// спринтов: RequireSprint отклоняет его сам, изменение — RequireWrite.
	api.Handle("/sprints", middleware.RequireSprint(access.SprintCreate, sprintH.CreateSprint)).Methods("POST")
	api.HandleFunc("/sprints/backlog", sprintH.GetBacklog).Methods("GET")
	api.HandleFunc("/sprints/{id}", sprintH.GetSprint).Methods("GET")
	api.Handle("/sprints/{id}", middleware.RequireWrite(http.HandlerFunc(sprintH.UpdateSprint))).Methods("PUT")
	api.Handle("/sprints/{id}", middleware.RequireSprint(access.SprintDelete, sprintH.DeleteSprint)).Methods("DELETE")
	api.Handle("/sprints/{id}/assign", middleware.RequireSprint(access.SprintTasks, sprintH.AssignTask)).Methods("POST")
	api.Handle("/sprints/{id}/assign/{issueId}", middleware.RequireSprint(access.SprintTasks, sprintH.UnassignTask)).Methods("DELETE")
	api.Handle("/sprints/{id}/refresh", middleware.RequireSprint(access.SprintTasks, sprintH.RefreshSprint)).Methods("POST")

	// Уведомления пользователя (колокольчик)
	api.HandleFunc("/notifications", notificationsH.List).Methods("GET")
	api.HandleFunc("/notifications/read-all", notificationsH.MarkAllRead).Methods("POST")
	api.HandleFunc("/notifications/clear", notificationsH.Clear).Methods("POST")
	api.HandleFunc("/push/key", pushH.Key).Methods("GET")
	api.HandleFunc("/push/subscribe", pushH.Subscribe).Methods("POST")
	api.HandleFunc("/push/unsubscribe", pushH.Unsubscribe).Methods("POST")
	api.HandleFunc("/push/test", pushH.Test).Methods("POST")
	api.HandleFunc("/notifications/{id}/read", notificationsH.MarkRead).Methods("POST")

	// Оплаты. Изменения закрыты для «только чтения», отдельные действия
	// проверяют сами обработчики по правам на оплаты.
	write := func(f http.HandlerFunc) http.Handler { return middleware.RequireWrite(f) }
	api.HandleFunc("/organizations", paymentsH.ListOrganizations).Methods("GET")
	api.Handle("/organizations", write(paymentsH.CreateOrganization)).Methods("POST")
	api.Handle("/organizations/{id}", write(paymentsH.UpdateOrganization)).Methods("PUT")
	api.Handle("/organizations/{id}", write(paymentsH.DeleteOrganization)).Methods("DELETE")
	api.HandleFunc("/contracts", paymentsH.ListContracts).Methods("GET")
	api.Handle("/contracts", write(paymentsH.CreateContract)).Methods("POST")
	api.Handle("/contracts/{id}", write(paymentsH.UpdateContract)).Methods("PUT")
	api.Handle("/contracts/{id}", write(paymentsH.DeleteContract)).Methods("DELETE")
	api.HandleFunc("/contracts/{id}/invoices", paymentsH.ListInvoices).Methods("GET")
	api.Handle("/contracts/{id}/invoices", write(paymentsH.CreateInvoice)).Methods("POST")
	api.Handle("/contracts/{id}/invoices/{invoiceId}", write(paymentsH.DeleteInvoice)).Methods("DELETE")
	api.HandleFunc("/contracts/{id}/invoices/{invoiceId}/items", paymentsH.GetInvoiceItems).Methods("GET")
	api.Handle("/contracts/{id}/invoices/{invoiceId}/pay", write(paymentsH.PayInvoice)).Methods("POST")
	api.HandleFunc("/contracts/{id}/invoices/{invoiceId}/download", paymentsH.DownloadInvoice).Methods("GET")
	api.HandleFunc("/contracts/export/csv", paymentsH.ExportCSV).Methods("GET")
	api.HandleFunc("/payments/stats", paymentsH.GetStats).Methods("GET")

	// Администрирование (нужна роль администратора)
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.RequireAdmin)
	// Права доступа: пользователи, их права, группы, шаблоны ролей
	admin.HandleFunc("/users", accessH.ListUsers).Methods("GET")
	admin.HandleFunc("/users", accessH.CreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}", accessH.UpdateUser).Methods("PUT")
	admin.HandleFunc("/users/{id}", accessH.DeleteUser).Methods("DELETE")
	admin.HandleFunc("/users/{id}/reset-password", accessH.ResetPassword).Methods("POST")
	admin.HandleFunc("/users/{id}/permissions", accessH.GetUserPermissions).Methods("GET")
	admin.HandleFunc("/users/{id}/permissions", accessH.SetUserPermissions).Methods("PUT")
	admin.HandleFunc("/users/{id}/permissions", accessH.ClearUserPermissions).Methods("DELETE")
	admin.HandleFunc("/permissions/clone", accessH.ClonePermissions).Methods("POST")
	admin.HandleFunc("/groups", accessH.ListGroups).Methods("GET")
	admin.HandleFunc("/groups", accessH.CreateGroup).Methods("POST")
	admin.HandleFunc("/groups/{id}", accessH.UpdateGroup).Methods("PUT")
	admin.HandleFunc("/groups/{id}", accessH.DeleteGroup).Methods("DELETE")
	admin.HandleFunc("/role-templates", accessH.ListTemplates).Methods("GET")
	admin.HandleFunc("/role-templates", accessH.CreateTemplate).Methods("POST")
	admin.HandleFunc("/role-templates/{id}", accessH.UpdateTemplate).Methods("PUT")
	admin.HandleFunc("/role-templates/{id}", accessH.DeleteTemplate).Methods("DELETE")
	admin.HandleFunc("/statuses", adminH.ListStatuses).Methods("GET")
	admin.HandleFunc("/statuses/{id}", adminH.UpdateStatusGroup).Methods("PUT")
	admin.HandleFunc("/priorities", adminH.ListPriorities).Methods("GET")
	admin.HandleFunc("/priorities/{id}", adminH.UpdatePriority).Methods("PUT")
	admin.HandleFunc("/datasource-config", adminH.GetDataSourceConfig).Methods("GET")
	admin.HandleFunc("/datasource-config", adminH.SaveDataSourceConfig).Methods("PUT")
	admin.HandleFunc("/datasource-config/test", adminH.TestDataSourceConfig).Methods("POST")
	admin.HandleFunc("/db-config", adminH.GetDBConfig).Methods("GET")
	admin.HandleFunc("/db-config", adminH.SaveDBConfig).Methods("PUT")
	admin.HandleFunc("/db-config/test", adminH.TestDBConfig).Methods("POST")
	admin.HandleFunc("/sync-log", adminH.GetSyncLog).Methods("GET")
	admin.HandleFunc("/sync/run", adminH.RunSync).Methods("POST")
	admin.HandleFunc("/sync-settings", adminH.GetSyncSettings).Methods("GET")
	admin.HandleFunc("/sync-settings", adminH.UpdateSyncSettings).Methods("PUT")
	admin.HandleFunc("/audit-log", adminH.GetAuditLog).Methods("GET")
	admin.HandleFunc("/setup-checklist", adminH.GetSetupChecklist).Methods("GET")
	admin.HandleFunc("/users/{id}/debug", debugH.SetUserDebug).Methods("PUT")
	admin.HandleFunc("/debug-logs", debugH.List).Methods("GET")
	admin.HandleFunc("/debug-logs", debugH.Clear).Methods("DELETE")
	admin.HandleFunc("/debug-logs/{id}", debugH.Get).Methods("GET")
	// Шаблон счёта
	admin.HandleFunc("/invoice-template", invoiceTemplateH.Get).Methods("GET")
	admin.HandleFunc("/invoice-template", invoiceTemplateH.Upload).Methods("POST")
	admin.HandleFunc("/invoice-template", invoiceTemplateH.Reset).Methods("DELETE")
	admin.HandleFunc("/invoice-template/file", invoiceTemplateH.File).Methods("GET")
	admin.HandleFunc("/invoice-template/preview", invoiceTemplateH.Preview).Methods("GET")
	// Фоновые воркеры и история, которую они собирают
	admin.HandleFunc("/workers", workersH.ListWorkers).Methods("GET")
	admin.HandleFunc("/workers/{key}/run", workersH.RunWorker).Methods("POST")
	admin.HandleFunc("/workers/{key}/stop", workersH.StopWorker).Methods("POST")
	admin.HandleFunc("/workers/{key}/enable", workersH.EnableWorker).Methods("POST")
	admin.HandleFunc("/workers/{key}/disable", workersH.DisableWorker).Methods("POST")
	admin.HandleFunc("/workers/{key}/schedule", workersH.UpdateSchedule).Methods("PUT")
	// Отправка почты
	admin.HandleFunc("/smtp", mailH.GetSMTP).Methods("GET")
	admin.HandleFunc("/smtp", mailH.UpdateSMTP).Methods("PUT")
	admin.HandleFunc("/smtp/test", mailH.TestSMTP).Methods("POST")
	admin.HandleFunc("/data-retention", workersH.GetDataRetention).Methods("GET")
	admin.HandleFunc("/data-retention", workersH.UpdateDataRetention).Methods("PUT")
	admin.HandleFunc("/snapshots/cleanup", workersH.CleanupSnapshots).Methods("POST")
	admin.HandleFunc("/snapshots/purge", workersH.PurgeSnapshots).Methods("POST")

	// WebSocket (нужна кука сессии — не открыт всем)
	r.HandleFunc("/ws", middleware.RequireWSSession(sessionStore, wsHub.HandleWebSocket))

	// Статические файлы (сборка фронтенда)
	frontendDir := "../frontend/dist"
	if d := os.Getenv("FRONTEND_DIR"); d != "" {
		frontendDir = d
	}
	// #nosec G703 -- FRONTEND_DIR задаёт администратор в окружении
	if _, err := os.Stat(frontendDir); err == nil {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
	}

	// HTTP-сервер
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Плавная остановка
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	slog.Info("Server listening", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped.")
}
