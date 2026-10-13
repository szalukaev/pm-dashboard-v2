-- Права доступа: какие данные видит пользователь, задаёт администратор —
-- лично, через шаблон роли или через группы.
--
-- Набор прав хранится одинаково в трёх таблицах:
--   project_ids / team_ids   списки разрешённых проектов и сотрудников (идентификаторы источника данных)
--   all_projects / all_team  все проекты / сотрудники, списки не учитываются
--   own_tasks_only           плюс задачи, назначенные самому пользователю (users.member_id)
--   read_only                данные можно смотреть, но не менять
--   visible_tabs / widgets   NULL — всё, иначе разрешённые ключи

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

-- Строка здесь значит, что права пользователя заданы лично и перекрывают
-- шаблон роли.
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
-- Сотрудник источника данных, которым является этот пользователь («свои
-- задачи» назначены ему). Определяется по личному ключу API, который вводит
-- пользователь. Ключ хранится зашифрованным (см. backend/secrets), чтобы
-- менять данные от имени пользователя; его последние символы хранятся
-- отдельно — по ним видно, какой ключ используется.
ALTER TABLE users ADD COLUMN IF NOT EXISTS member_id INTEGER;
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_token_enc TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_token_hint VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_template_id INTEGER REFERENCES role_templates(id) ON DELETE SET NULL;

-- Готовые шаблоны ролей.
INSERT INTO role_templates (name, description, is_system, grants_admin, all_projects, all_team, own_tasks_only, read_only) VALUES
    ('Только свои задачи', 'Доступ только к задачам, где пользователь указан исполнителем', true, false, false, false, true, false),
    ('Наблюдатель', 'Все проекты и сотрудники, только чтение', true, false, true, true, false, true),
    ('Руководитель', 'Все проекты и сотрудники', true, false, true, true, false, false),
    ('Администратор', 'Полный доступ ко всем данным и настройкам', true, true, true, true, false, false)
ON CONFLICT (name) DO NOTHING;

-- Пользователи, существовавшие до появления прав доступа, видели всё: так
-- и остаётся, пока администратор не сузит их права.
INSERT INTO user_permissions (user_id, all_projects, all_team)
SELECT id, true, true FROM users WHERE role <> 'admin'
ON CONFLICT (user_id) DO NOTHING;
