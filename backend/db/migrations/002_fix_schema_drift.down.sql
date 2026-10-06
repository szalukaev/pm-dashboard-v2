-- 002 down: drop columns added to fix schema drift.
ALTER TABLE audit_log DROP COLUMN IF EXISTS status_code;

ALTER TABLE priorities DROP COLUMN IF EXISTS synced_at;
ALTER TABLE statuses DROP COLUMN IF EXISTS synced_at;

ALTER TABLE user_settings DROP COLUMN IF EXISTS overdue_alerts;

ALTER TABLE users DROP COLUMN IF EXISTS last_login;
ALTER TABLE users DROP COLUMN IF EXISTS avatar;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
