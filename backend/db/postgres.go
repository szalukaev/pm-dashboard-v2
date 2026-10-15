package db

import (
	"database/sql"
	_ "embed"
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
	// Таблица учёта применённых миграций
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Поиск файлов миграций
	migrationsDir := "db/migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Вторая попытка — рядом с исполняемым файлом
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
		// Номер версии — из имени файла (например, 001_initial.up.sql -> 1)
		base := filepath.Base(f)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) < 2 {
			continue
		}
		var version int
		_, _ = fmt.Sscanf(parts[0], "%d", &version)
		if version == 0 {
			continue
		}

		// Уже применена?
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if count > 0 {
			continue
		}

		// Чтение и выполнение миграции
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		// Миграция и запись о ней применяются одной транзакцией: наполовину
		// применённая миграция не должна ни остаться, ни выполниться повторно.
		slog.Info("Applying migration", "version", version, "file", base)
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", version, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
		slog.Info("Migration applied", "version", version)
	}

	return nil
}

// runFallbackMigrations выполняет встроенный SQL, если файлы миграций не найдены
func runFallbackMigrations(db *sql.DB) error {
	if _, err := db.Exec(fallbackMigrationSQL); err != nil {
		return err
	}
	if _, err := db.Exec(accessControlSQL); err != nil {
		return err
	}
	if _, err := db.Exec(rightsFromSourceSQL); err != nil {
		return err
	}
	if _, err := db.Exec(showUnassignedSQL); err != nil {
		return err
	}
	if _, err := db.Exec(sprintActionsSQL); err != nil {
		return err
	}
	if _, err := db.Exec(licenseEventsSQL); err != nil {
		return err
	}
	if _, err := db.Exec(paginationSQL); err != nil {
		return err
	}
	if _, err := db.Exec(sourceMembersSQL); err != nil {
		return err
	}
	if _, err := db.Exec(tableColumnsSQL); err != nil {
		return err
	}
	if _, err := db.Exec(syncStateSQL); err != nil {
		return err
	}
	// Таблицы снимков создаёт только эта миграция; дважды она выполняться
	// не должна, иначе перезапуск удалил бы накопленную историю.
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('issue_snapshots') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := db.Exec(snapshotsSQL); err != nil {
			return err
		}
	}
	if _, err := db.Exec(invoiceItemNameSQL); err != nil {
		return err
	}
	if _, err := db.Exec(dashboardLayoutSQL); err != nil {
		return err
	}
	if _, err := db.Exec(sprintSharingSQL); err != nil {
		return err
	}
	if _, err := db.Exec(notificationsSQL); err != nil {
		return err
	}
	if _, err := db.Exec(emailSQL); err != nil {
		return err
	}
	if _, err := db.Exec(emailReportSQL); err != nil {
		return err
	}
	_, err := db.Exec(indicatorThresholdsSQL)
	return err
}

// Схема прав доступа идемпотентна, поэтому запасной путь выполняет сами
// файлы миграций, а не держит их вторую копию.
//
//go:embed migrations/006_access_control.up.sql
var accessControlSQL string

//go:embed migrations/007_rights_from_source.up.sql
var rightsFromSourceSQL string

//go:embed migrations/008_show_unassigned.up.sql
var showUnassignedSQL string

//go:embed migrations/009_sprint_actions.up.sql
var sprintActionsSQL string

//go:embed migrations/010_license_events.up.sql
var licenseEventsSQL string

//go:embed migrations/011_pagination.up.sql
var paginationSQL string

//go:embed migrations/012_source_members.up.sql
var sourceMembersSQL string

//go:embed migrations/013_table_columns.up.sql
var tableColumnsSQL string

//go:embed migrations/014_sync_state.up.sql
var syncStateSQL string

//go:embed migrations/015_snapshots.up.sql
var snapshotsSQL string

//go:embed migrations/016_invoice_item_name.up.sql
var invoiceItemNameSQL string

//go:embed migrations/017_dashboard_layout.up.sql
var dashboardLayoutSQL string

//go:embed migrations/018_sprint_sharing.up.sql
var sprintSharingSQL string

//go:embed migrations/019_notifications.up.sql
var notificationsSQL string

//go:embed migrations/020_email.up.sql
var emailSQL string

//go:embed migrations/021_email_report.up.sql
var emailReportSQL string

//go:embed migrations/022_indicator_thresholds.up.sql
var indicatorThresholdsSQL string

// fallbackMigrationSQL — страховка на случай, когда файлы миграций недоступны
const fallbackMigrationSQL = `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) DEFAULT '',
    avatar TEXT DEFAULT '',
    role VARCHAR(20) DEFAULT 'user',
    force_password_change BOOLEAN DEFAULT false,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
DO $$ BEGIN
    ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) DEFAULT '';
    ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT DEFAULT '';
    ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login TIMESTAMPTZ;
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
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
    kanban_last_mode VARCHAR(20) NOT NULL DEFAULT 'users',
    kanban_last_project INTEGER,
    last_filters JSONB DEFAULT '{}',
    notifications_enabled BOOLEAN DEFAULT true,
    overdue_alerts BOOLEAN DEFAULT false,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS overdue_alerts BOOLEAN DEFAULT false;
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
    data_source VARCHAR(50) DEFAULT 'redmine', synced_at TIMESTAMPTZ, UNIQUE(external_id, data_source)
);
ALTER TABLE statuses ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE statuses ADD COLUMN IF NOT EXISTS is_bug BOOLEAN NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS priorities (
    id SERIAL PRIMARY KEY, external_id INTEGER NOT NULL, name VARCHAR(255) NOT NULL,
    sort_order INTEGER DEFAULT 0, color VARCHAR(20) DEFAULT '#888888',
    data_source VARCHAR(50) DEFAULT 'redmine', synced_at TIMESTAMPTZ, UNIQUE(external_id, data_source)
);
ALTER TABLE priorities ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
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
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(100) PRIMARY KEY, value JSONB NOT NULL, updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY, occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL, action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50), entity_id BIGINT, before_state JSONB, after_state JSONB,
    status_code INTEGER, ip_address TEXT, user_agent TEXT
);
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS status_code INTEGER;
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
CREATE TABLE IF NOT EXISTS time_entries (
    external_id INTEGER NOT NULL, issue_id INTEGER NOT NULL, project_id INTEGER NOT NULL,
    activity_name VARCHAR(255) NOT NULL DEFAULT '', hours NUMERIC(10,2) NOT NULL DEFAULT 0,
    data_source VARCHAR(50) NOT NULL DEFAULT 'redmine', synced_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (external_id, data_source)
);
CREATE INDEX IF NOT EXISTS idx_time_entries_issue_id ON time_entries(issue_id);
CREATE INDEX IF NOT EXISTS idx_time_entries_project_id ON time_entries(project_id);
`
