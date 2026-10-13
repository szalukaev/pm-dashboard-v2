ALTER TABLE group_permissions DROP COLUMN IF EXISTS kanban_modes;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS kanban_modes;
ALTER TABLE role_templates DROP COLUMN IF EXISTS kanban_modes;
