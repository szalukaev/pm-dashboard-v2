-- Access control: what data a user may see is set by the administrator —
-- individually, through a role template or through groups.
--
-- A permission set is stored the same way in three tables:
--   project_ids / team_ids   lists of allowed projects and members (ids of the data source)
--   all_projects / all_team  every project / member, the lists are ignored
--   own_tasks_only           plus the issues assigned to the user (users.member_id)
--   read_only                data may be viewed but not changed
--   visible_tabs / widgets   NULL = everything, otherwise the allowed keys

CREATE TABLE IF NOT EXISTS role_templates (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_system BOOLEAN NOT NULL DEFAULT false,
    grants_admin BOOLEAN NOT NULL DEFAULT false,
    project_ids JSONB NOT NULL DEFAULT '[]',
    team_ids JSONB NOT NULL DEFAULT '[]',
    visible_tabs JSONB,
    widgets JSONB,
    all_projects BOOLEAN NOT NULL DEFAULT false,
    all_team BOOLEAN NOT NULL DEFAULT false,
    own_tasks_only BOOLEAN NOT NULL DEFAULT false,
    read_only BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A row here means the rights of the user are set individually and override
-- the role template.
CREATE TABLE IF NOT EXISTS user_permissions (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    project_ids JSONB NOT NULL DEFAULT '[]',
    team_ids JSONB NOT NULL DEFAULT '[]',
    visible_tabs JSONB,
    widgets JSONB,
    all_projects BOOLEAN NOT NULL DEFAULT false,
    all_team BOOLEAN NOT NULL DEFAULT false,
    own_tasks_only BOOLEAN NOT NULL DEFAULT false,
    read_only BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id);

CREATE TABLE IF NOT EXISTS group_permissions (
    group_id INTEGER PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    project_ids JSONB NOT NULL DEFAULT '[]',
    team_ids JSONB NOT NULL DEFAULT '[]',
    visible_tabs JSONB,
    widgets JSONB,
    all_projects BOOLEAN NOT NULL DEFAULT false,
    all_team BOOLEAN NOT NULL DEFAULT false,
    own_tasks_only BOOLEAN NOT NULL DEFAULT false,
    read_only BOOLEAN NOT NULL DEFAULT false
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_blocked BOOLEAN NOT NULL DEFAULT false;
-- The member of the data source this user is ("own tasks" are assigned to it).
-- It is found by the personal API key the user enters. The key is stored
-- encrypted (see backend/secrets) to change data on behalf of the user; its
-- last characters are kept apart to show which key is in use.
ALTER TABLE users ADD COLUMN IF NOT EXISTS member_id INTEGER;
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_token_enc TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_token_hint VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_template_id INTEGER REFERENCES role_templates(id) ON DELETE SET NULL;

-- Preset role templates.
INSERT INTO role_templates (name, description, is_system, grants_admin, all_projects, all_team, own_tasks_only, read_only) VALUES
    ('Только свои задачи', 'Доступ только к задачам, где пользователь указан исполнителем', true, false, false, false, true, false),
    ('Наблюдатель', 'Все проекты и сотрудники, только чтение', true, false, true, true, false, true),
    ('Руководитель', 'Все проекты и сотрудники', true, false, true, true, false, false),
    ('Администратор', 'Полный доступ ко всем данным и настройкам', true, true, true, true, false, false)
ON CONFLICT (name) DO NOTHING;

-- Users that existed before access control saw everything: keep it that way
-- until the administrator narrows their rights.
INSERT INTO user_permissions (user_id, all_projects, all_team)
SELECT id, true, true FROM users WHERE role <> 'admin'
ON CONFLICT (user_id) DO NOTHING;
