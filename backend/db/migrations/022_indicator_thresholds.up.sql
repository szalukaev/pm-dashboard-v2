-- Пороги индикаторов пользователя:
-- {"overdue": 5, "bugs": 10, "no_estimate": 3, "per_project": false}.
-- Индикатор, превысивший порог, подсвечивается на «Аналитике». per_project —
-- порог сравнивается с числом задач каждого проекта, а не всех сразу.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS indicator_thresholds JSONB NOT NULL DEFAULT '{}';
