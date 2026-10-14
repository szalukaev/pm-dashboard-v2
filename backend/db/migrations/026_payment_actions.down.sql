ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_user_id_fkey;
ALTER TABLE contracts ADD CONSTRAINT contracts_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_user_id_fkey;
ALTER TABLE organizations ADD CONSTRAINT organizations_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE role_templates DROP COLUMN IF EXISTS payment_actions;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS payment_actions;
ALTER TABLE group_permissions DROP COLUMN IF EXISTS payment_actions;
ALTER TABLE role_templates DROP COLUMN IF EXISTS payments_own_only;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS payments_own_only;
ALTER TABLE group_permissions DROP COLUMN IF EXISTS payments_own_only;
