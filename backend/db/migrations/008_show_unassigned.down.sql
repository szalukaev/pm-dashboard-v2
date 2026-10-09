ALTER TABLE group_permissions DROP COLUMN IF EXISTS show_unassigned;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS show_unassigned;
ALTER TABLE role_templates DROP COLUMN IF EXISTS show_unassigned;
