-- How many rows a page of each table holds, chosen by the user:
-- {"tasks_table": 50, "backlog": 25, ...}. A table that is not listed uses
-- the default page size.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pagination JSONB NOT NULL DEFAULT '{}';
