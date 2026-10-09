-- When each project was last read from the data source: any successful
-- load and the last complete one. Kept in the database so that a restart
-- does not make the next sync read everything again, and per project so
-- that a failure of one project does not make the others be re-read.
CREATE TABLE IF NOT EXISTS sync_state (
    project_id INTEGER NOT NULL,
    data_source VARCHAR(50) NOT NULL DEFAULT 'redmine',
    last_sync_at TIMESTAMPTZ NOT NULL,
    last_full_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (project_id, data_source)
);
