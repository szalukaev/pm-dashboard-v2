-- Журнал уведомлений пользователя. Одно событие приходит один раз: пара
-- (пользователь, ключ повтора) уникальна. Ключ строит правило: у ежедневной
-- просрочки в нём дата дня, у «до срока осталось N дней» — дата срока.
--   rule         правило, по которому создано уведомление
--   params       данные для текста: номер и название задачи, что изменилось
--   entity_type  на что ведёт уведомление: issue, contract, invoice, sprint, system
--   to_bell      показывать в колокольчике
--   to_email     включить в письмо; emailed_at — когда отправлено
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule VARCHAR(50) NOT NULL,
    params JSONB NOT NULL DEFAULT '{}',
    entity_type VARCHAR(20) NOT NULL DEFAULT '',
    entity_id INTEGER NOT NULL DEFAULT 0,
    dedupe_key VARCHAR(200) NOT NULL,
    to_bell BOOLEAN NOT NULL DEFAULT true,
    to_email BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ,
    emailed_at TIMESTAMPTZ,
    UNIQUE (user_id, dedupe_key)
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications (user_id, created_at DESC);

-- Последнее виденное состояние задачи. Сравнение с ним после синхронизации
-- показывает, что изменилось: ответственный, статус, приоритет, срок, оценка.
CREATE TABLE IF NOT EXISTS issue_watch (
    issue_id INTEGER PRIMARY KEY,
    assigned_to_id INTEGER,
    status_id INTEGER NOT NULL DEFAULT 0,
    status_name VARCHAR(255) NOT NULL DEFAULT '',
    status_group VARCHAR(50) NOT NULL DEFAULT 'open',
    priority_id INTEGER NOT NULL DEFAULT 0,
    priority_name VARCHAR(255) NOT NULL DEFAULT '',
    due_date VARCHAR(10) NOT NULL DEFAULT '',
    estimated_hours NUMERIC(10,2),
    seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Правила уведомлений пользователя:
-- {"task_due_soon": {"bell": true, "email": false, "days": 3}, ...}.
-- Правило, которого здесь нет, работает по умолчанию.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS alert_rules JSONB NOT NULL DEFAULT '{}';
