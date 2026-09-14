package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	// Create migrations tracking table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Find migration files
	migrationsDir := "db/migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Try relative to executable
		execPath, _ := os.Executable()
		migrationsDir = filepath.Join(filepath.Dir(execPath), "db", "migrations")
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		slog.Warn("No migration files found, using embedded SQL fallback")
		return runFallbackMigrations(db)
	}

	sort.Strings(files)

	for _, f := range files {
		// Extract version number from filename (e.g., 001_initial.up.sql -> 1)
		base := filepath.Base(f)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) < 2 {
			continue
		}
		var version int
		fmt.Sscanf(parts[0], "%d", &version)
		if version == 0 {
			continue
		}

		// Check if already applied
		var count int
		db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
		if count > 0 {
			continue
		}

		// Read and execute migration
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		slog.Info("Applying migration", "version", version, "file", base)
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %d: %w", version, err)
		}

		db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version)
		slog.Info("Migration applied", "version", version)
	}

	return nil
}

// runFallbackMigrations runs the inline SQL if no migration files found
func runFallbackMigrations(db *sql.DB) error {
	_, err := db.Exec(fallbackMigrationSQL)
	return err
}

// fallbackMigrationSQL is kept as a safety net when migration files are not available
const fallbackMigrationSQL = `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',
    force_password_change BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS user_settings (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    selected_projects JSONB DEFAULT '[]',
    selected_team JSONB DEFAULT '[]',
    theme VARCHAR(50) DEFAULT 'dark',
    language VARCHAR(10) DEFAULT 'ru',
    tab_order JSONB DEFAULT '[]',
    kanban_column_order_statuses JSONB DEFAULT '[]',
    kanban_column_order_users JSONB DEFAULT '[]',
    last_filters JSONB DEFAULT '{}',
    notifications_enabled BOOLEAN DEFAULT true,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, name VARCHAR(500) NOT NULL,
    parent_id INTEGER, data_source VARCHAR(50) DEFAULT 'redmine', synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);
CREATE TABLE IF NOT EXISTS members (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, name VARCHAR(255) NOT NULL,
    login VARCHAR(100), data_source VARCHAR(50) DEFAULT 'redmine', synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);
CREATE TABLE IF NOT EXISTS issues (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, project_id INTEGER, project_name VARCHAR(500),
    subject VARCHAR(1000), description TEXT, status_name VARCHAR(255), status_id INTEGER,
    priority_name VARCHAR(255), priority_id INTEGER, assigned_to_name VARCHAR(255), assigned_to_id INTEGER,
    category_name VARCHAR(255), start_date DATE, due_date DATE, estimated_hours NUMERIC(10,2),
    spent_hours NUMERIC(10,2), done_ratio INTEGER DEFAULT 0, tracker_name VARCHAR(255),
    author_name VARCHAR(255), bug_fix_hours NUMERIC(10,2) DEFAULT 0,
    data_source VARCHAR(50) DEFAULT 'redmine', synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);
CREATE TABLE IF NOT EXISTS statuses (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, name VARCHAR(255) NOT NULL,
    is_closed BOOLEAN DEFAULT false, group_name VARCHAR(50) DEFAULT 'open',
    data_source VARCHAR(50) DEFAULT 'redmine', UNIQUE(external_id, data_source)
);
CREATE TABLE IF NOT EXISTS priorities (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, name VARCHAR(255) NOT NULL,
    sort_order INTEGER DEFAULT 0, color VARCHAR(20) DEFAULT '#888888',
    data_source VARCHAR(50) DEFAULT 'redmine', UNIQUE(external_id, data_source)
);
CREATE TABLE IF NOT EXISTS organizations (
    id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL, address TEXT, inn VARCHAR(20),
    contact_name VARCHAR(255), contact_phone VARCHAR(50), created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS contracts (
    id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
    contract_type VARCHAR(20) NOT NULL DEFAULT 'service', name VARCHAR(500) NOT NULL,
    company_name VARCHAR(500), company_address TEXT, amount NUMERIC(15,2) DEFAULT 0,
    vat_rate VARCHAR(10) DEFAULT 'none', contact_name VARCHAR(255), contact_phone VARCHAR(50),
    start_date DATE, end_date DATE, created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS invoices (
    id SERIAL PRIMARY KEY, contract_id INTEGER REFERENCES contracts(id) ON DELETE CASCADE,
    amount NUMERIC(15,2) NOT NULL, vat_rate VARCHAR(10) DEFAULT 'none',
    issued_at DATE NOT NULL, paid_amount NUMERIC(15,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'unpaid', created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS invoice_items (
    id SERIAL PRIMARY KEY, invoice_id INTEGER REFERENCES invoices(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL, quantity NUMERIC(10,2) DEFAULT 1, price NUMERIC(15,2) DEFAULT 0
);
CREATE TABLE IF NOT EXISTS contract_payments (
    id SERIAL PRIMARY KEY, invoice_id INTEGER REFERENCES invoices(id) ON DELETE CASCADE,
    amount NUMERIC(15,2) NOT NULL, paid_at DATE NOT NULL, created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS sprints (
    id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL, project_name VARCHAR(500), status VARCHAR(20) DEFAULT 'open',
    start_date DATE, due_date DATE, description TEXT, category_name VARCHAR(255),
    auto_fill_category BOOLEAN DEFAULT false, created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS sprint_issues (
    sprint_id INTEGER REFERENCES sprints(id) ON DELETE CASCADE,
    issue_external_id INTEGER NOT NULL, added_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (sprint_id, issue_external_id)
);
CREATE TABLE IF NOT EXISTS admin_settings (
    key VARCHAR(100) PRIMARY KEY, value JSONB NOT NULL, updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS collection_log (
    id SERIAL PRIMARY KEY, started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ, duration_ms INTEGER, issues_collected INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'running', error_text TEXT, data_source VARCHAR(50) DEFAULT 'redmine'
);
CREATE TABLE IF NOT EXISTS daily_snapshots (
    id SERIAL PRIMARY KEY, snapshot_date DATE NOT NULL, project_id INTEGER, project_name VARCHAR(500),
    open_count INTEGER DEFAULT 0, testing_count INTEGER DEFAULT 0, closed_count INTEGER DEFAULT 0,
    overdue_count INTEGER DEFAULT 0, total_count INTEGER DEFAULT 0,
    data_source VARCHAR(50) DEFAULT 'redmine', created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(snapshot_date, project_id, data_source)
);
CREATE TABLE IF NOT EXISTS issue_snapshots (
    id SERIAL PRIMARY KEY, issue_external_id INTEGER NOT NULL, snapshot_date DATE NOT NULL,
    status_name VARCHAR(255), priority_name VARCHAR(255), assigned_to_name VARCHAR(255),
    estimated_hours NUMERIC(10,2), spent_hours NUMERIC(10,2), done_ratio INTEGER,
    data_source VARCHAR(50) DEFAULT 'redmine', created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(100) PRIMARY KEY, value JSONB NOT NULL, updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY, occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL, action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50), entity_id BIGINT, before_state JSONB, after_state JSONB,
    ip_address TEXT, user_agent TEXT
);
CREATE TABLE IF NOT EXISTS licenses (
    id BIGSERIAL PRIMARY KEY, license_blob TEXT NOT NULL, hwid_hash TEXT NOT NULL,
    first_activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_check_ok_at TIMESTAMPTZ,
    grace_started_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS user_layouts (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    layout JSONB NOT NULL DEFAULT '{}', updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS issue_weights (
    id SERIAL PRIMARY KEY, priority_external_id INTEGER NOT NULL,
    weight NUMERIC(5,2) DEFAULT 1.0, data_source VARCHAR(50) DEFAULT 'redmine',
    UNIQUE(priority_external_id, data_source)
);
`
