-- «Права как в источнике данных» распространяются и на команду: сотрудники,
-- которых источник показывает пользователю (читаются его личным ключом API),
-- — предел того, что он может видеть. NULL — ещё не прочитаны.
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_members JSONB;
ALTER TABLE users ADD COLUMN IF NOT EXISTS source_members_at TIMESTAMPTZ;

UPDATE role_templates
SET description = 'Проекты — те же, в которых пользователь участвует в системе-источнике; сотрудники — те, кого источник показывает пользователю в этих проектах'
WHERE name = 'Взять из СИ' AND is_system;
