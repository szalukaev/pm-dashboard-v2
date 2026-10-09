-- "Rights as in the data source": a permission set may say that the user
-- gets the projects they are a member of in the source (Redmine). The list
-- is read with the personal API key of the user and refreshed by the sync.

ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;

-- The projects of the user in the source and when they were last read.
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_projects JSONB NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_projects_at TIMESTAMPTZ;

INSERT INTO role_templates (name, description, is_system, from_source, all_team)
VALUES ('Взять из СИ', 'Проекты — те же, в которых пользователь участвует в системе-источнике; все сотрудники этих проектов', true, true, true)
ON CONFLICT (name) DO NOTHING;
