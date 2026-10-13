-- История данных: что дашборд показывал в каждый из дней.
--
-- В две таблицы снимков из первой схемы ничего не писалось; их заменяет
-- структура ниже.

-- Что источник знает о задаче помимо её текущего состояния.
ALTER TABLE issues ADD COLUMN IF NOT EXISTS created_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS updated_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS closed_on TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS fixed_version_name VARCHAR(255);

DROP TABLE IF EXISTS daily_snapshots;
DROP TABLE IF EXISTS issue_snapshots;

-- Строка на сотрудника и день: итоги по назначенным ему задачам.
-- redmine_id 0 означает задачи без исполнителя.
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
    -- Задачи по статусам и по приоритетам: {"New": 4, "In Progress": 2}.
    -- Статусы настраиваются в источнике, поэтому столбцами они не сделаны.
    status_counts JSONB NOT NULL DEFAULT '{}',
    priority_counts JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (date, redmine_id)
);

-- Строка на задачу и день: незакрытые задачи и те, что закрыты в этот день.
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
