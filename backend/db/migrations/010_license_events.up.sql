-- The journal of license events (activation, failed checks, grace period,
-- read-only), kept apart from the audit of user actions.
CREATE TABLE IF NOT EXISTS license_events (
    id BIGSERIAL PRIMARY KEY,
    event VARCHAR(50) NOT NULL,
    details TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_license_events_created_at ON license_events(created_at);
