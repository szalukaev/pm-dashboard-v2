-- Настройка «оповещать о просрочках» заменена правилами уведомлений и больше
-- нигде не читается.
ALTER TABLE user_settings DROP COLUMN IF EXISTS overdue_alerts;
