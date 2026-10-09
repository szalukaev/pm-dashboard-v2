DELETE FROM role_templates WHERE name = 'Взять из СИ' AND is_system;
ALTER TABLE users DROP COLUMN IF EXISTS source_projects_at;
ALTER TABLE users DROP COLUMN IF EXISTS source_projects;
ALTER TABLE group_permissions DROP COLUMN IF EXISTS from_source;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS from_source;
ALTER TABLE role_templates DROP COLUMN IF EXISTS from_source;
