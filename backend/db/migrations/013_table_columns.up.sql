-- Какие колонки каждой таблицы видит пользователь и в каком порядке:
-- {"tasks_table": {"visible": ["subject", ...], "order": ["external_id", ...]}}.
-- Таблица, которой нет в списке, показывает все колонки в стандартном порядке.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS table_columns JSONB NOT NULL DEFAULT '{}';
