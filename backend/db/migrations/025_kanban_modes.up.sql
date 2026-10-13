-- Какие виды доски Kanban доступны: users — по сотрудникам, statuses — по
-- проектам (доска статусов одного проекта). Список ключей JSON; NULL — оба
-- вида, как было до этой настройки.
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS kanban_modes JSONB;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS kanban_modes JSONB;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS kanban_modes JSONB;
