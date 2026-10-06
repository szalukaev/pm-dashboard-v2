package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"pm-dashboard/config"
	"pm-dashboard/db"
	"pm-dashboard/utils"

	"golang.org/x/crypto/bcrypt"
)

type SetupHandler struct {
	SQLite *config.SQLiteStore
	PGDB   **sql.DB
}

type dbTestRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type dsTestRequest struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	APIKey      string `json:"api_key"`
	BasicLogin  string `json:"basic_login"`
	BasicPasswd string `json:"basic_password"`
}

type adminCreateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Language    string `json:"language"`
}

func buildDSN(r dbTestRequest) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		r.User, r.Password, r.Host, portOrDefault(r.Port), r.DBName)
}

func buildMaintDSN(r dbTestRequest) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
		r.User, r.Password, r.Host, portOrDefault(r.Port))
}

func portOrDefault(port string) string {
	if port == "" {
		return "5432"
	}
	return port
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

	// 1. Try connecting to the target DB directly
	testDB, err := sql.Open("postgres", buildDSN(req))
	if err == nil {
		pingErr := testDB.Ping()
		testDB.Close()
		if pingErr == nil {
			utils.JSON(w, http.StatusOK, map[string]interface{}{
				"success": true, "message": "Connection established", "db_exists": true,
			})
			return
		}
	}

	// 2. Target DB may not exist — verify credentials via maintenance DB
	maintDB, err := sql.Open("postgres", buildMaintDSN(req))
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "Connection failed"})
		return
	}
	defer maintDB.Close()

	if err := maintDB.Ping(); err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Credentials OK, target DB doesn't exist yet
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "Connection established", "db_exists": false,
	})
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

	dsn := buildDSN(req)

	// Check if target DB is reachable
	dbReachable := false
	if testDB, err := sql.Open("postgres", dsn); err == nil {
		if testDB.Ping() == nil {
			dbReachable = true
		}
		testDB.Close()
	}

	// If not reachable, create the database via maintenance connection
	if !dbReachable {
		maintDB, err := sql.Open("postgres", buildMaintDSN(req))
		if err != nil {
			utils.Error(w, http.StatusBadRequest, "DB_CONNECTION_FAILED")
			return
		}
		if err := maintDB.Ping(); err != nil {
			maintDB.Close()
			utils.Error(w, http.StatusBadRequest, "DB_CONNECTION_FAILED")
			return
		}
		// CREATE DATABASE cannot run inside a transaction
		createSQL := fmt.Sprintf(`CREATE DATABASE "%s"`, req.DBName)
		if _, err := maintDB.Exec(createSQL); err != nil {
			maintDB.Close()
			utils.Error(w, http.StatusInternalServerError, "DB_CREATE_FAILED")
			return
		}
		maintDB.Close()
	}

	// Connect to the target DB
	conn, err := db.Connect(dsn)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DB_CONNECT_FAILED")
		return
	}

	// Run migrations
	if err := db.RunMigrations(conn); err != nil {
		utils.Error(w, http.StatusInternalServerError, "MIGRATION_FAILED")
		return
	}

	// Save to SQLite
	if err := h.SQLite.Set("db_dsn", dsn); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}

	*h.PGDB = conn

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

	client := &http.Client{}
	httpReq, err := http.NewRequest("GET", req.URL+"/projects.json?limit=1", nil)
	if err != nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Always set API key
	if req.APIKey != "" {
		httpReq.Header.Set("X-Redmine-API-Key", req.APIKey)
	}

	// Optionally add Basic Auth
	if req.BasicLogin != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(req.BasicLogin + ":" + req.BasicPasswd))
		httpReq.Header.Set("Authorization", "Basic "+cred)
	}

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

	if req.BasicLogin != "" {
		h.SQLite.Set("redmine_basic_login", req.BasicLogin)
		h.SQLite.Set("redmine_basic_password", req.BasicPasswd)
	} else {
		h.SQLite.Set("redmine_basic_login", "")
		h.SQLite.Set("redmine_basic_password", "")
	}

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

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}
	_, err = (*h.PGDB).Exec(
		"INSERT INTO users (username, password_hash, role, display_name) VALUES ($1, $2, 'admin', $3)",
		req.Username, string(hash), displayName,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			utils.Error(w, http.StatusConflict, "USER_EXISTS")
			return
		}
		utils.Error(w, http.StatusInternalServerError, "CREATE_USER_FAILED")
		return
	}

	h.SQLite.Set("session_secret", "auto-generated-secret")
	h.SQLite.Set("admin_created", "true")
	if req.Language != "" {
		h.SQLite.Set("default_language", req.Language)
	}

	utils.Success(w)
}
