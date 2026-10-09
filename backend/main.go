package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/config"
	"pm-dashboard/db"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/handlers"
	"pm-dashboard/middleware"
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

	// Init SQLite config store
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

	// Connect to PostgreSQL if configured
	var pgDB *sql.DB
	if cfg.DSN != "" {
		pgDB, err = db.Connect(cfg.DSN)
		if err != nil {
			slog.Warn("PostgreSQL not available", "error", err)
		} else {
			defer pgDB.Close()
			// Never serve traffic on top of an incompatible schema.
			if err := db.RunMigrations(pgDB); err != nil {
				slog.Error("Migrations failed, refusing to start", "error", err)
				os.Exit(1)
			}
		}
	}

	// Session store (Redis or in-memory fallback)
	redisURL := os.Getenv("REDIS_URL")
	sessionStore := db.NewSessionStore(redisURL)
	defer sessionStore.Close()

	// License checker (read-only middleware)
	var licenseChecker *middleware.LicenseChecker
	if pgDB != nil {
		licenseChecker = middleware.NewLicenseChecker(&pgDB)
	}

	// Audit middleware
	var auditMW *middleware.AuditMiddleware
	if pgDB != nil {
		auditMW = &middleware.AuditMiddleware{DB: &pgDB}
	}

	// Handlers
	setupH := &handlers.SetupHandler{SQLite: sqliteStore, PGDB: &pgDB}
	authH := &handlers.AuthHandler{DB: &pgDB, Sessions: sessionStore, Audit: auditMW}
	settingsH := &handlers.SettingsHandler{DB: &pgDB, SQLite: sqliteStore}
	taskH := &handlers.TaskHandler{DB: &pgDB}
	analyticsH := &handlers.AnalyticsHandler{DB: &pgDB}
	kanbanH := &handlers.KanbanHandler{DB: &pgDB}
	sprintH := &handlers.SprintHandler{DB: &pgDB}
	paymentsH := &handlers.PaymentsHandler{DB: &pgDB}
	adminH := &handlers.AdminHandler{DB: &pgDB, SQLite: sqliteStore}
	licenseH := &handlers.LicenseHandler{DB: &pgDB}
	notifH := &handlers.NotificationsHandler{DB: &pgDB}
	wsHub := handlers.NewWSHub()

	// License periodic check — starts grace period without visiting the license page
	if pgDB != nil {
		go func() {
			licenseH.RunPeriodicCheck()
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				licenseH.RunPeriodicCheck()
			}
		}()
	}


	// Data source client and background sync. The manager re-reads connection
	// settings and the interval on the fly, so the loop runs even while the
	// source or the database is not configured yet (setup wizard).
	source := manager.New(sqliteStore, &pgDB, func() bool {
		return licenseChecker != nil && licenseChecker.IsReadOnly()
	})
	taskH.Source = source
	kanbanH.Source = source
	adminH.Source = source
	setupH.Source = source
	settingsH.Source = source
	syncStatusH := &handlers.SyncStatusHandler{DB: &pgDB, Source: source}
	accessH := &handlers.AccessHandler{DB: &pgDB, Source: source}
	// Issues are synced for the projects the users are shown within their rights
	source.SetProjectSource((&access.Resolver{DB: &pgDB}).SyncProjects)

	// Real-time: the sync and user edits report changes to open pages
	source.SetNotifier(wsHub.BroadcastEvent)
	taskH.Events = wsHub.BroadcastEvent
	kanbanH.Events = wsHub.BroadcastEvent
	go source.Run(context.Background())

	// Router
	r := mux.NewRouter()

	// CORS
	r.Use(middleware.CORSMiddleware)

	// Security headers (after CORS so ACAO is not overwritten)
	r.Use(middleware.SecurityHeaders)

	// Cap JSON request bodies (1 MiB)
	r.Use(middleware.LimitJSONBody)

	// Setup endpoints (no auth required)
	r.HandleFunc("/api/setup/status", setupH.Status).Methods("GET")
	r.HandleFunc("/api/setup/database/test", setupH.TestDatabase).Methods("POST")
	r.HandleFunc("/api/setup/database", setupH.SaveDatabase).Methods("POST")
	r.HandleFunc("/api/setup/datasource/test", setupH.TestDataSource).Methods("POST")
	r.HandleFunc("/api/setup/datasource", setupH.SaveDataSource).Methods("POST")
	r.HandleFunc("/api/setup/admin", setupH.CreateAdmin).Methods("POST")

	// Status endpoint (public, version info)
	r.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":     "3.0.0",
			"status":      "ok",
			"is_setup":    sqliteStore.IsSetupComplete(),
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}).Methods("GET")

	// License status is public (banner for every user); activation is admin-only, see below
	r.HandleFunc("/api/license/status", licenseH.GetStatus).Methods("GET")

	// Auth endpoints (no auth required for login)
	r.HandleFunc("/api/auth/login", authH.Login).Methods("POST")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.RequireAuth(sessionStore, &pgDB))

	// Wire audit middleware — logs all mutating requests
	if auditMW != nil {
		api.Use(auditMW.Log)
	}

	// Wire license read-only middleware — blocks writes when license expired
	if licenseChecker != nil {
		api.Use(licenseChecker.ReadOnlyMiddleware)
	}

	// A tab hidden from a user by the administrator is closed here as well,
	// not only in the menu. Task cards and reference lists stay open: other
	// tabs use them.
	api.Use(middleware.RequireTabs([]middleware.TabRule{
		{Path: "/api/tasks", Exact: true, Tab: "tasks"},
		{Path: "/api/analytics", Tab: "analytics"},
		{Path: "/api/kanban", Tab: "kanban"},
		{Path: "/api/sprints", Tab: "sprint"},
		{Path: "/api/organizations", Tab: "payments"},
		{Path: "/api/contracts", Tab: "payments"},
		{Path: "/api/payments", Tab: "payments"},
	}))

	api.Handle("/license/activate", middleware.RequireAdmin(http.HandlerFunc(licenseH.Activate))).Methods("POST")

	api.HandleFunc("/auth/logout", authH.Logout).Methods("POST")
	api.HandleFunc("/auth/me", authH.Me).Methods("GET")
	api.HandleFunc("/auth/me", authH.UpdateMe).Methods("PUT")
	api.HandleFunc("/auth/change-password", authH.ChangePassword).Methods("POST")
	api.HandleFunc("/auth/avatar", authH.UploadAvatar).Methods("POST")
	api.HandleFunc("/auth/avatar", authH.DeleteAvatar).Methods("DELETE")
	api.HandleFunc("/sync/status", syncStatusH.GetStatus).Methods("GET")
	api.HandleFunc("/settings", settingsH.GetSettings).Methods("GET")
	api.HandleFunc("/settings", settingsH.UpdateSettings).Methods("PUT")

	// Notifications & Alerts
	api.HandleFunc("/notifications/settings", notifH.GetSettings).Methods("GET")
	api.HandleFunc("/notifications/settings", notifH.UpdateSettings).Methods("PUT")
	api.HandleFunc("/notifications/thresholds", notifH.GetAlertThresholds).Methods("GET")
	api.HandleFunc("/notifications/thresholds", notifH.UpdateAlertThresholds).Methods("PUT")

	// Tasks
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

	// Analytics
	api.HandleFunc("/analytics/stats", analyticsH.GetStats).Methods("GET")
	api.HandleFunc("/analytics/team-load", analyticsH.GetTeamLoad).Methods("GET")
	api.HandleFunc("/analytics/distribution", analyticsH.GetDistribution).Methods("GET")
	api.HandleFunc("/analytics/deadlines", analyticsH.GetDeadlines).Methods("GET")

	// Kanban
	api.HandleFunc("/kanban/board", kanbanH.GetBoard).Methods("GET")
	api.Handle("/kanban/move", middleware.RequireWrite(http.HandlerFunc(kanbanH.MoveCard))).Methods("PUT")
	api.HandleFunc("/kanban/column-order", kanbanH.SaveColumnOrder).Methods("PUT")

	// Sprints
	api.HandleFunc("/sprints", sprintH.ListSprints).Methods("GET")
	api.HandleFunc("/sprints", sprintH.CreateSprint).Methods("POST")
	api.HandleFunc("/sprints/backlog", sprintH.GetBacklog).Methods("GET")
	api.HandleFunc("/sprints/{id}", sprintH.GetSprint).Methods("GET")
	api.HandleFunc("/sprints/{id}", sprintH.UpdateSprint).Methods("PUT")
	api.HandleFunc("/sprints/{id}", sprintH.DeleteSprint).Methods("DELETE")
	api.HandleFunc("/sprints/{id}/assign", sprintH.AssignTask).Methods("POST")
	api.HandleFunc("/sprints/{id}/assign/{issueId}", sprintH.UnassignTask).Methods("DELETE")
	api.HandleFunc("/sprints/{id}/refresh", sprintH.RefreshSprint).Methods("POST")

	// Payments
	api.HandleFunc("/organizations", paymentsH.ListOrganizations).Methods("GET")
	api.HandleFunc("/organizations", paymentsH.CreateOrganization).Methods("POST")
	api.HandleFunc("/organizations/{id}", paymentsH.UpdateOrganization).Methods("PUT")
	api.HandleFunc("/organizations/{id}", paymentsH.DeleteOrganization).Methods("DELETE")
	api.HandleFunc("/contracts", paymentsH.ListContracts).Methods("GET")
	api.HandleFunc("/contracts", paymentsH.CreateContract).Methods("POST")
	api.HandleFunc("/contracts/{id}", paymentsH.UpdateContract).Methods("PUT")
	api.HandleFunc("/contracts/{id}", paymentsH.DeleteContract).Methods("DELETE")
	api.HandleFunc("/contracts/{id}/invoices", paymentsH.ListInvoices).Methods("GET")
	api.HandleFunc("/contracts/{id}/invoices", paymentsH.CreateInvoice).Methods("POST")
	api.HandleFunc("/contracts/{id}/invoices/{invoiceId}", paymentsH.DeleteInvoice).Methods("DELETE")
	api.HandleFunc("/contracts/{id}/invoices/{invoiceId}/pay", paymentsH.PayInvoice).Methods("POST")
	api.HandleFunc("/contracts/{id}/invoices/{invoiceId}/download", paymentsH.DownloadInvoice).Methods("GET")
	api.HandleFunc("/contracts/export/csv", paymentsH.ExportCSV).Methods("GET")
	api.HandleFunc("/payments/stats", paymentsH.GetStats).Methods("GET")

	// Admin (admin role required)
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.RequireAdmin)
	// Access control: users, their rights, groups, role templates
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
	admin.HandleFunc("/db-config/test", adminH.TestDBConfig).Methods("POST")
	admin.HandleFunc("/sync-log", adminH.GetSyncLog).Methods("GET")
	admin.HandleFunc("/sync/run", adminH.RunSync).Methods("POST")
	admin.HandleFunc("/sync-settings", adminH.GetSyncSettings).Methods("GET")
	admin.HandleFunc("/sync-settings", adminH.UpdateSyncSettings).Methods("PUT")
	admin.HandleFunc("/audit-log", adminH.GetAuditLog).Methods("GET")

	// WebSocket (session cookie required — not public)
	r.HandleFunc("/ws", middleware.RequireWSSession(sessionStore, wsHub.HandleWebSocket))

	// Serve static files (frontend build)
	frontendDir := "../frontend/dist"
	if d := os.Getenv("FRONTEND_DIR"); d != "" {
		frontendDir = d
	}
	if _, err := os.Stat(frontendDir); err == nil {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
	}

	// HTTP Server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	slog.Info("Server listening", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped.")
}
