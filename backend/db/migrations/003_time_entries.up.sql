-- Time entries from the data source. Fact and bug fix hours of an issue are
-- sums over this table, so a sync only needs to fetch recent entries.
CREATE TABLE IF NOT EXISTS time_entries (
    external_id INTEGER NOT NULL,
    issue_id INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    activity_name VARCHAR(255) NOT NULL DEFAULT '',
    hours NUMERIC(10,2) NOT NULL DEFAULT 0,
    data_source VARCHAR(50) NOT NULL DEFAULT 'redmine',
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (external_id, data_source)
);
CREATE INDEX IF NOT EXISTS idx_time_entries_issue_id ON time_entries(issue_id);
CREATE INDEX IF NOT EXISTS idx_time_entries_project_id ON time_entries(project_id);
