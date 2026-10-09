ALTER TABLE group_permissions DROP COLUMN IF EXISTS sprint_actions;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS sprint_actions;
ALTER TABLE role_templates DROP COLUMN IF EXISTS sprint_actions;
