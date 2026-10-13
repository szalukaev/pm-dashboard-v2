-- Задачи без исполнителя показываются только тем, кому дано такое право; по
-- умолчанию оно выключено.
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
