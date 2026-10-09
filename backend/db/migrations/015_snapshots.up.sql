-- History of the data: what the dashboard showed on each day.
--
-- The two snapshot tables of the first schema were never written to; they
-- are replaced by the layout below.

-- What the source knows about an issue beyond its current state.
ALTER TABLE issues ADD COLUMN IF NOT EXISTS created_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS updated_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS closed_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS fixed_version_name VARCHAR(255);

DROP TABLE IF EXISTS daily_snapshots;
DROP TABLE IF EXISTS issue_snapshots;

-- One row per member and day: the totals of the issues assigned to them.
-- redmine_id 0 stands for the issues without an assignee.
CREATE TABLE daily_snapshots (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL,
    redmine_id INTEGER NOT NULL,
    user_login TEXT NOT NULL DEFAULT '',
    user_name TEXT NOT NULL DEFAULT '',
    total_open INTEGER NOT NULL DEFAULT 0,
    total_closed INTEGER NOT NULL DEFAULT 0,
    total_overdue INTEGER NOT NULL DEFAULT 0,
    testing_count INTEGER NOT NULL DEFAULT 0,
    bugs_count INTEGER NOT NULL DEFAULT 0,
    high_priority_count INTEGER NOT NULL DEFAULT 0,
    no_estimate_count INTEGER NOT NULL DEFAULT 0,
    total_estimated_hours REAL NOT NULL DEFAULT 0,
    total_actual_hours REAL NOT NULL DEFAULT 0,
    total_bug_hours REAL NOT NULL DEFAULT 0,
    bug_percent REAL NOT NULL DEFAULT 0,
    -- Issues per status and per priority: {"New": 4, "In Progress": 2}.
    -- Statuses are configured in the source, so they are not columns.
    status_counts JSONB NOT NULL DEFAULT '{}',
    priority_counts JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (date, redmine_id)
);

-- One row per issue and day: the issues that were not closed, and the ones
-- closed on that day.
CREATE TABLE issue_snapshots (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL,
    issue_id INTEGER NOT NULL,
    redmine_id INTEGER NOT NULL DEFAULT 0,
    user_login TEXT NOT NULL DEFAULT '',
    subject TEXT,
    project_id INTEGER,
    project_name TEXT,
    status TEXT,
    priority TEXT,
    due_date DATE,
    estimated_hours REAL,
    is_overdue BOOLEAN NOT NULL DEFAULT FALSE,
    is_closed BOOLEAN NOT NULL DEFAULT FALSE,
    bug_percent REAL NOT NULL DEFAULT 0,
    last_updated_on TIMESTAMPTZ,
    created_on TIMESTAMPTZ,
    sprint_name TEXT,
    UNIQUE (date, issue_id)
);
CREATE INDEX idx_issue_snapshots_issue ON issue_snapshots (issue_id, date);
