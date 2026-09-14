-- 001: Initial schema
-- PM Dashboard V3 — All core tables

-- Users
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',
    force_password_change BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- User settings
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

-- Cached projects from data source
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    external_id INTEGER NOT NULL,
    name VARCHAR(500) NOT NULL,
    parent_id INTEGER,
    data_source VARCHAR(50) DEFAULT 'redmine',
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);

-- Cached users/members from data source
CREATE TABLE IF NOT EXISTS members (
    id SERIAL PRIMARY KEY,
    external_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    login VARCHAR(100),
    data_source VARCHAR(50) DEFAULT 'redmine',
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);

-- Cached issues from data source
CREATE TABLE IF NOT EXISTS issues (
    id SERIAL PRIMARY KEY,
    external_id INTEGER NOT NULL,
    project_id INTEGER,
    project_name VARCHAR(500),
    subject VARCHAR(1000),
    description TEXT,
    status_name VARCHAR(255),
    status_id INTEGER,
    priority_name VARCHAR(255),
    priority_id INTEGER,
    assigned_to_name VARCHAR(255),
    assigned_to_id INTEGER,
    category_name VARCHAR(255),
    start_date DATE,
    due_date DATE,
    estimated_hours NUMERIC(10,2),
    spent_hours NUMERIC(10,2),
    done_ratio INTEGER DEFAULT 0,
    tracker_name VARCHAR(255),
    author_name VARCHAR(255),
    bug_fix_hours NUMERIC(10,2) DEFAULT 0,
    data_source VARCHAR(50) DEFAULT 'redmine',
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(external_id, data_source)
);

-- Statuses mapping
CREATE TABLE IF NOT EXISTS statuses (
    id SERIAL PRIMARY KEY,
    external_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_closed BOOLEAN DEFAULT false,
    group_name VARCHAR(50) DEFAULT 'open',
    data_source VARCHAR(50) DEFAULT 'redmine',
    UNIQUE(external_id, data_source)
);

-- Priorities mapping
CREATE TABLE IF NOT EXISTS priorities (
    id SERIAL PRIMARY KEY,
    external_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    sort_order INTEGER DEFAULT 0,
    color VARCHAR(20) DEFAULT '#888888',
    data_source VARCHAR(50) DEFAULT 'redmine',
    UNIQUE(external_id, data_source)
);

-- Organizations
CREATE TABLE IF NOT EXISTS organizations (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    address TEXT,
    inn VARCHAR(20),
    contact_name VARCHAR(255),
    contact_phone VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Contracts
CREATE TABLE IF NOT EXISTS contracts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
    contract_type VARCHAR(20) NOT NULL DEFAULT 'service',
    name VARCHAR(500) NOT NULL,
    company_name VARCHAR(500),
    company_address TEXT,
    amount NUMERIC(15,2) DEFAULT 0,
    vat_rate VARCHAR(10) DEFAULT 'none',
    contact_name VARCHAR(255),
    contact_phone VARCHAR(50),
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Invoices
CREATE TABLE IF NOT EXISTS invoices (
    id SERIAL PRIMARY KEY,
    contract_id INTEGER REFERENCES contracts(id) ON DELETE CASCADE,
    amount NUMERIC(15,2) NOT NULL,
    vat_rate VARCHAR(10) DEFAULT 'none',
    issued_at DATE NOT NULL,
    paid_amount NUMERIC(15,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'unpaid',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Invoice items
CREATE TABLE IF NOT EXISTS invoice_items (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER REFERENCES invoices(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    quantity NUMERIC(10,2) DEFAULT 1,
    price NUMERIC(15,2) DEFAULT 0
);

-- Contract payments
CREATE TABLE IF NOT EXISTS contract_payments (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER REFERENCES invoices(id) ON DELETE CASCADE,
    amount NUMERIC(15,2) NOT NULL,
    paid_at DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sprints
CREATE TABLE IF NOT EXISTS sprints (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    project_name VARCHAR(500),
    status VARCHAR(20) DEFAULT 'open',
    start_date DATE,
    due_date DATE,
    description TEXT,
    category_name VARCHAR(255),
    auto_fill_category BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sprint-Issue mapping
CREATE TABLE IF NOT EXISTS sprint_issues (
    sprint_id INTEGER REFERENCES sprints(id) ON DELETE CASCADE,
    issue_external_id INTEGER NOT NULL,
    added_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (sprint_id, issue_external_id)
);

-- Admin settings
CREATE TABLE IF NOT EXISTS admin_settings (
    key VARCHAR(100) PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Collection log
CREATE TABLE IF NOT EXISTS collection_log (
    id SERIAL PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    duration_ms INTEGER,
    issues_collected INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'running',
    error_text TEXT,
    data_source VARCHAR(50) DEFAULT 'redmine'
);

-- Daily snapshots
CREATE TABLE IF NOT EXISTS daily_snapshots (
    id SERIAL PRIMARY KEY,
    snapshot_date DATE NOT NULL,
    project_id INTEGER,
    project_name VARCHAR(500),
    open_count INTEGER DEFAULT 0,
    testing_count INTEGER DEFAULT 0,
    closed_count INTEGER DEFAULT 0,
    overdue_count INTEGER DEFAULT 0,
    total_count INTEGER DEFAULT 0,
    data_source VARCHAR(50) DEFAULT 'redmine',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(snapshot_date, project_id, data_source)
);

-- Issue snapshots
CREATE TABLE IF NOT EXISTS issue_snapshots (
    id SERIAL PRIMARY KEY,
    issue_external_id INTEGER NOT NULL,
    snapshot_date DATE NOT NULL,
    status_name VARCHAR(255),
    priority_name VARCHAR(255),
    assigned_to_name VARCHAR(255),
    estimated_hours NUMERIC(10,2),
    spent_hours NUMERIC(10,2),
    done_ratio INTEGER,
    data_source VARCHAR(50) DEFAULT 'redmine',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Global runtime settings
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(100) PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audit log
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50),
    entity_id BIGINT,
    before_state JSONB,
    after_state JSONB,
    ip_address TEXT,
    user_agent TEXT
);

-- Licenses
CREATE TABLE IF NOT EXISTS licenses (
    id BIGSERIAL PRIMARY KEY,
    license_blob TEXT NOT NULL,
    hwid_hash TEXT NOT NULL,
    first_activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_check_ok_at TIMESTAMPTZ,
    grace_started_at TIMESTAMPTZ
);

-- User layouts
CREATE TABLE IF NOT EXISTS user_layouts (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    layout JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Issue weights
CREATE TABLE IF NOT EXISTS issue_weights (
    id SERIAL PRIMARY KEY,
    priority_external_id INTEGER NOT NULL,
    weight NUMERIC(5,2) DEFAULT 1.0,
    data_source VARCHAR(50) DEFAULT 'redmine',
    UNIQUE(priority_external_id, data_source)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_issues_project_id ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_issues_status_name ON issues(status_name);
CREATE INDEX IF NOT EXISTS idx_issues_due_date ON issues(due_date);
CREATE INDEX IF NOT EXISTS idx_issues_assigned_to_id ON issues(assigned_to_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_occurred_at ON audit_log(occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_collection_log_started_at ON collection_log(started_at);
