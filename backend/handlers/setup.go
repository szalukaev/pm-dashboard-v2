package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"pm-dashboard/config"
	"pm-dashboard/db"
	"pm-dashboard/utils"

	"golang.org/x/crypto/bcrypt"
)

type SetupHandler struct {
	SQLite     *config.SQLiteStore
	PGDB       **sql.DB // pointer to pointer, set after setup
}

type dbTestRequest struct {
	DSN string `json:"dsn"`
}

type dsTestRequest struct {
	Type    string `json:"type"` // "redmine"
	URL     string `json:"url"`
	APIKey  string `json:"api_key"`
}

type adminCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"isComplete": h.SQLite.IsSetupComplete(),
	})
}

func (h *SetupHandler) TestDatabase(w http.ResponseWriter, r *http.Request) {
	if h.SQLite.IsSetupComplete() {
		utils.Error(w, http.StatusForbidden, "SETUP_ALREADY_COMPLETE")
		return
	}

	var req dbTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	testDB, err := sql.Open("postgres", req.DSN)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	defer testDB.Close()

	if err := testDB.Ping(); err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Connection established"})
}

func (h *SetupHandler) SaveDatabase(w http.ResponseWriter, r *http.Request) {
	if h.SQLite.IsSetupComplete() {
		utils.Error(w, http.StatusForbidden, "SETUP_ALREADY_COMPLETE")
		return
	}

	var req dbTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Test connection
	testDB, err := sql.Open("postgres", req.DSN)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "DB_CONNECTION_FAILED")
		return
	}
	defer testDB.Close()
	if err := testDB.Ping(); err != nil {
		utils.Error(w, http.StatusBadRequest, "DB_CONNECTION_FAILED")
		return
	}

	// Run migrations
	if err := db.RunMigrations(testDB); err != nil {
		utils.Error(w, http.StatusInternalServerError, "MIGRATION_FAILED")
		return
	}

	// Save to SQLite
	if err := h.SQLite.Set("db_dsn", req.DSN); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}

	// Connect main DB
	mainDB, err := db.Connect(req.DSN)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DB_CONNECT_FAILED")
		return
	}
	*h.PGDB = mainDB

	utils.Success(w)
}

func (h *SetupHandler) TestDataSource(w http.ResponseWriter, r *http.Request) {
	if h.SQLite.IsSetupComplete() {
		utils.Error(w, http.StatusForbidden, "SETUP_ALREADY_COMPLETE")
		return
	}

	var req dsTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Simple HTTP test to Redmine
	client := &http.Client{}
	httpReq, err := http.NewRequest("GET", req.URL+"/projects.json?limit=1", nil)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	httpReq.Header.Set("X-Redmine-API-Key", req.APIKey)
	resp, err := client.Do(httpReq)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "HTTP " + resp.Status})
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Connection established"})
}

func (h *SetupHandler) SaveDataSource(w http.ResponseWriter, r *http.Request) {
	if h.SQLite.IsSetupComplete() {
		utils.Error(w, http.StatusForbidden, "SETUP_ALREADY_COMPLETE")
		return
	}

	var req dsTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	h.SQLite.Set("data_source_type", req.Type)
	h.SQLite.Set("redmine_url", req.URL)
	h.SQLite.Set("redmine_api_key", req.APIKey)

	utils.Success(w)
}

func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	if h.SQLite.IsSetupComplete() {
		utils.Error(w, http.StatusForbidden, "SETUP_ALREADY_COMPLETE")
		return
	}

	if *h.PGDB == nil {
		utils.Error(w, http.StatusBadRequest, "DATABASE_NOT_CONFIGURED")
		return
	}

	var req adminCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if len(req.Username) < 3 || len(req.Password) < 6 {
		utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}

	_, err = (*h.PGDB).Exec(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'admin')",
		req.Username, string(hash),
	)
	if err != nil {
		utils.Error(w, http.StatusConflict, "USER_EXISTS")
		return
	}

	// Generate session secret
	h.SQLite.Set("session_secret", "auto-generated-secret")

	// Mark admin as created
	h.SQLite.Set("admin_created", "true")

	utils.Success(w)
}
