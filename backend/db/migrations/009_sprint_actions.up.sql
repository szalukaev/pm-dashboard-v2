-- What a user may do with sprints, each action allowed separately:
-- create, edit, tasks (change the tasks of a sprint), close, delete.
-- NULL = every action, as it was before these rights existed.
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
