-- Сколько строк на странице каждой таблицы выбрал пользователь:
-- {"tasks_table": 50, "backlog": 25, ...}. У таблицы, которой нет в списке,
-- размер страницы по умолчанию.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pagination JSONB NOT NULL DEFAULT '{}';
