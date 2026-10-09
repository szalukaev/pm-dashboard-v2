-- Issues without an assignee are shown only to those given the right to see
-- them; the right is off by default.
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS show_unassigned BOOLEAN NOT NULL DEFAULT false;
