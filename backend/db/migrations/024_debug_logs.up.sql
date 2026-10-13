-- Отладочное логирование по пользователю. Администратор включает его в
-- таблице пользователей; пока срок не истёк, запросы этого пользователя
-- записываются в debug_logs.

-- До какого момента включено; NULL или прошедшее время — выключено.
-- Срок ставится на 24 часа, чтобы режим не остался включённым по забывчивости.
ALTER TABLE users ADD COLUMN IF NOT EXISTS debug_until TIMESTAMPTZ;

-- Один запрос пользователя. Секреты в телах заменены на ***; тело больше
-- предела не сохраняется — вместо него пометка с размером.
CREATE TABLE IF NOT EXISTS debug_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    method VARCHAR(10) NOT NULL,
    path TEXT NOT NULL,
    query TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    request_body TEXT NOT NULL DEFAULT '',
    response_body TEXT NOT NULL DEFAULT '',
    ip_address VARCHAR(64) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_debug_logs_user_time ON debug_logs(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_debug_logs_time ON debug_logs(occurred_at);
