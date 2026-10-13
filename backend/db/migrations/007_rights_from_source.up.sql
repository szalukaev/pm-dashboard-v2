-- «Права как в источнике данных»: набор прав может указывать, что
-- пользователь получает проекты, в которых он состоит в источнике (Redmine).
-- Список читается личным ключом API пользователя и обновляется синхронизацией.

ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS from_source BOOLEAN NOT NULL DEFAULT false;

-- Проекты пользователя в источнике и время их последнего чтения.
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_projects JSONB NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_projects_at TIMESTAMPTZ;

INSERT INTO role_templates (name, description, is_system, from_source, all_team)
VALUES ('Взять из СИ', 'Проекты — те же, в которых пользователь участвует в системе-источнике; все сотрудники этих проектов', true, true, true)
ON CONFLICT (name) DO NOTHING;
