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

	"pm-dashboard/config"
	"pm-dashboard/db"
	"pm-dashboard/datasource/redmine"
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
		slog.Error("Failed to init SQLite config", "error", err); os.Exit(1)
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
			if err := db.RunMigrations(pgDB); err != nil {
				slog.Warn("Migrations failed", "error", err)
			}
		}
	}

	// Session store (Redis or in-memory fallback)
	redisURL := os.Getenv("REDIS_URL")
	sessionStore := db.NewSessionStore(redisURL)
	defer sessionStore.Close()

	// Handlers
	setupH := &handlers.SetupHandler{SQLite: sqliteStore, PGDB: &pgDB}
	authH := &handlers.AuthHandler{DB: pgDB, Sessions: sessionStore}
	settingsH := &handlers.SettingsHandler{DB: pgDB}
	taskH := &handlers.TaskHandler{DB: pgDB}
	analyticsH := &handlers.AnalyticsHandler{DB: pgDB}
	kanbanH := &handlers.KanbanHandler{DB: pgDB}
	sprintH := &handlers.SprintHandler{DB: pgDB}
	paymentsH := &handlers.PaymentsHandler{DB: pgDB}
	adminH := &handlers.AdminHandler{DB: pgDB}
	licenseH := &handlers.LicenseHandler{DB: pgDB}
	notifH := &handlers.NotificationsHandler{DB: pgDB}
	wsHub := handlers.NewWSHub()

	// Start Redmine sync if configured
	if cfg.RedmineURL != "" && cfg.RedmineAPIKey != "" && pgDB != nil {
		client := redmine.NewClient(cfg.RedmineURL, cfg.RedmineAPIKey)
		syncer := redmine.NewSyncer(client)
		go func() {
			time.Sleep(2 * time.Second)
			if err := syncer.SyncAll(context.Background(), pgDB); err != nil {
				slog.Error("Initial sync failed", "error", err)
			}
			// Periodic sync every 5 minutes
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if err := syncer.SyncAll(context.Background(), pgDB); err != nil {
					slog.Error("Periodic sync failed", "error", err)
				}
			}
		}()
	}

	// Router
	r := mux.NewRouter()

	// CORS
	r.Use(middleware.CORSMiddleware)

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

	// License endpoints (public — no auth required)
	r.HandleFunc("/api/license/status", licenseH.GetStatus).Methods("GET")
	r.HandleFunc("/api/license/activate", licenseH.Activate).Methods("POST")

	// Auth endpoints (no auth required for login)
	r.HandleFunc("/api/auth/login", authH.Login).Methods("POST")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.RequireAuth(sessionStore, pgDB))

	api.HandleFunc("/auth/logout", authH.Logout).Methods("POST")
	api.HandleFunc("/auth/me", authH.Me).Methods("GET")
	api.HandleFunc("/settings", settingsH.GetSettings).Methods("GET")
	api.HandleFunc("/settings", settingsH.UpdateSettings).Methods("PUT")

	// Notifications & Alerts
	api.HandleFunc("/notifications/settings", notifH.GetSettings).Methods("GET")
	api.HandleFunc("/notifications/settings", notifH.UpdateSettings).Methods("PUT")
	api.HandleFunc("/notifications/thresholds", notifH.GetAlertThresholds).Methods("GET")
	api.HandleFunc("/notifications/thresholds", notifH.UpdateAlertThresholds).Methods("PUT")

	// Tasks
	api.HandleFunc("/tasks", taskH.ListTasks).Methods("GET")
	api.HandleFunc("/tasks/{id}", taskH.GetTask).Methods("GET")
	api.HandleFunc("/tasks/{id}", taskH.UpdateTask).Methods("PUT")
	api.HandleFunc("/tasks/categories", taskH.GetCategories).Methods("GET")
	api.HandleFunc("/tasks/projects", taskH.GetProjects).Methods("GET")
	api.HandleFunc("/tasks/statuses", taskH.GetStatuses).Methods("GET")

	// Analytics
	api.HandleFunc("/analytics/stats", analyticsH.GetStats).Methods("GET")
	api.HandleFunc("/analytics/team-load", analyticsH.GetTeamLoad).Methods("GET")
	api.HandleFunc("/analytics/distribution", analyticsH.GetDistribution).Methods("GET")
	api.HandleFunc("/analytics/deadlines", analyticsH.GetDeadlines).Methods("GET")

	// Kanban
	api.HandleFunc("/kanban/board", kanbanH.GetBoard).Methods("GET")
	api.HandleFunc("/kanban/move", kanbanH.MoveCard).Methods("PUT")
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
	admin.HandleFunc("/users", adminH.ListUsers).Methods("GET")
	admin.HandleFunc("/users", adminH.CreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}", adminH.UpdateUser).Methods("PUT")
	admin.HandleFunc("/users/{id}", adminH.DeleteUser).Methods("DELETE")
	admin.HandleFunc("/statuses", adminH.ListStatuses).Methods("GET")
	admin.HandleFunc("/statuses/{id}", adminH.UpdateStatusGroup).Methods("PUT")
	admin.HandleFunc("/priorities", adminH.ListPriorities).Methods("GET")
	admin.HandleFunc("/priorities/{id}", adminH.UpdatePriority).Methods("PUT")
	admin.HandleFunc("/datasource-config", adminH.GetDataSourceConfig).Methods("GET")
	admin.HandleFunc("/datasource-config", adminH.SaveDataSourceConfig).Methods("PUT")
	admin.HandleFunc("/sync-log", adminH.GetSyncLog).Methods("GET")
	admin.HandleFunc("/audit-log", adminH.GetAuditLog).Methods("GET")

	// WebSocket
	r.HandleFunc("/ws", wsHub.HandleWebSocket)

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
		slog.Error("Server error", "error", err); os.Exit(1)
	}
	slog.Info("Server stopped.")
}
