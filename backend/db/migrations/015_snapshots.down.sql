DROP TABLE IF EXISTS issue_snapshots;
DROP TABLE IF EXISTS daily_snapshots;
ALTER TABLE issues DROP COLUMN IF EXISTS fixed_version_name;
ALTER TABLE issues DROP COLUMN IF EXISTS closed_on;
ALTER TABLE issues DROP COLUMN IF EXISTS updated_on;
ALTER TABLE issues DROP COLUMN IF EXISTS created_on;
