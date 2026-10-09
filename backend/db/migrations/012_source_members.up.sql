-- "Rights as in the data source" also cover the team: the members the
-- source shows to the user (read with the user's personal API key) are the
-- most the user may see. NULL = not read yet.
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_members JSONB;
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_members_at TIMESTAMPTZ;

UPDATE role_templates
SET description = 'Проекты — те же, в которых пользователь участвует в системе-источнике; сотрудники — те, кого источник показывает пользователю в этих проектах'
WHERE name = 'Взять из СИ' AND is_system;
