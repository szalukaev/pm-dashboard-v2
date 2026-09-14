package config

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "config.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

func (s *SQLiteStore) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) Set(key, value string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
	return err
}

func (s *SQLiteStore) IsSetupComplete() bool {
	required := []string{"db_dsn", "session_secret", "admin_created"}
	for _, key := range required {
		val, err := s.Get(key)
		if err != nil || val == "" {
			return false
		}
	}
	return true
}

func (s *SQLiteStore) LoadAppConfig() *AppConfig {
	cfg := &AppConfig{
		Port: "8080",
	}
	if v, _ := s.Get("db_dsn"); v != "" {
		cfg.DSN = v
	}
	if v, _ := s.Get("session_secret"); v != "" {
		cfg.SessionSecret = v
	}
	if v, _ := s.Get("redmine_url"); v != "" {
		cfg.RedmineURL = v
	}
	if v, _ := s.Get("redmine_api_key"); v != "" {
		cfg.RedmineAPIKey = v
	}
	if v, _ := s.Get("data_source_type"); v != "" {
		cfg.DataSourceType = v
	} else {
		cfg.DataSourceType = "redmine"
	}
	if v := os.Getenv("APP_PORT"); v != "" {
		cfg.Port = v
	}
	return cfg
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
