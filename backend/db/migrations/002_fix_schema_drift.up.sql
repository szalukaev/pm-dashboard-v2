-- 002: Fix schema drift between file migrations and fallbackMigrationSQL.
-- These columns existed only in the embedded fallback, so a clean deploy
-- via file migrations broke login (users.display_name/avatar/last_login)
-- and settings/sync. Also adds audit_log.status_code for the audit middleware.

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login TIMESTAMPTZ;

ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS overdue_alerts BOOLEAN DEFAULT false;

ALTER TABLE statuses ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE priorities ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;

ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS status_code INTEGER;
