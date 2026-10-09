-- The board opens in the mode and on the project the user left it.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS kanban_last_mode VARCHAR(20) NOT NULL DEFAULT 'users';
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS kanban_last_project INTEGER;

-- Column order used to be saved by display names; columns are now identified
-- by member and status ids, so the old lists no longer match anything.
UPDATE user_settings SET kanban_column_order_users = '[]', kanban_column_order_statuses = '[]';
