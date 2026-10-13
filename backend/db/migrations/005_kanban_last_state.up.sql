-- Доска открывается в том виде и на том проекте, где пользователь её оставил.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS kanban_last_mode VARCHAR(20) NOT NULL DEFAULT 'users';
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS kanban_last_project INTEGER;

-- Раньше порядок колонок сохранялся по отображаемым именам; теперь колонки
-- определяются идентификаторами сотрудников и статусов, и прежние списки
-- больше ничему не соответствуют.
UPDATE user_settings SET kanban_column_order_users = '[]', kanban_column_order_statuses = '[]';
