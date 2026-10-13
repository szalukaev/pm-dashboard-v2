-- 002: выравнивание схемы между файлами миграций и fallbackMigrationSQL.
-- Эти столбцы были только во встроенном запасном SQL, поэтому чистая
-- установка через файлы миграций ломала вход (users.display_name/avatar/
-- last_login), настройки и синхронизацию. Заодно добавляется
-- audit_log.status_code для журнала аудита.

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login TIMESTAMPTZ;

ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS overdue_alerts BOOLEAN DEFAULT false;

ALTER TABLE statuses ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE priorities ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;

ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS status_code INTEGER;
