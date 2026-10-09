ALTER TABLE users DROP COLUMN IF EXISTS role_template_id;
ALTER TABLE users DROP COLUMN IF EXISTS member_id;
ALTER TABLE users DROP COLUMN IF EXISTS is_blocked;
DROP TABLE IF EXISTS group_permissions;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_templates;
