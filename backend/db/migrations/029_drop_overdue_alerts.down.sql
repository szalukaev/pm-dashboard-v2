ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS overdue_alerts BOOLEAN DEFAULT false;
