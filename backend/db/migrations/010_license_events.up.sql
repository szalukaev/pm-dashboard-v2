-- Журнал событий лицензии (активация, неудачные проверки, льготный период,
-- только чтение); ведётся отдельно от аудита действий пользователей.
CREATE TABLE IF NOT EXISTS license_events (
    id BIGSERIAL PRIMARY KEY,
    event VARCHAR(50) NOT NULL,
    details TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_license_events_created_at ON license_events(created_at);
