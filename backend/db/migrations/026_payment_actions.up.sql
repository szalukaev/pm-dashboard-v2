-- Права во вкладке «Оплаты»: какие действия разрешены (список ключей JSON;
-- NULL — все) и видны ли только записи, созданные самим пользователем
-- (NULL — не задано: в личных правах и шаблонах это «только свои», как было
-- до этой настройки, а у группы — ограничения нет, видны все записи).
ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS payment_actions JSONB;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS payment_actions JSONB;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS payment_actions JSONB;

ALTER TABLE role_templates ADD COLUMN IF NOT EXISTS payments_own_only BOOLEAN;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS payments_own_only BOOLEAN;
ALTER TABLE group_permissions ADD COLUMN IF NOT EXISTS payments_own_only BOOLEAN;

-- Оплаты теперь общие: удаление пользователя не должно стирать организации
-- и договоры, с которыми работают другие. Автор просто перестаёт быть указан.
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_user_id_fkey;
ALTER TABLE organizations ADD CONSTRAINT organizations_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_user_id_fkey;
ALTER TABLE contracts ADD CONSTRAINT contracts_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
