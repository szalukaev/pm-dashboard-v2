-- Что пользователь может делать со спринтами; каждое действие разрешается
-- отдельно: create, edit, tasks (менять задачи спринта), close, delete.
-- NULL — все действия, как было до появления этих прав.
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS sprint_actions JSONB;
