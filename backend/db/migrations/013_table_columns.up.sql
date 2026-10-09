-- Which columns of each table the user sees and in what order:
-- {"tasks_table": {"visible": ["subject", ...], "order": ["external_id", ...]}}.
-- A table that is not listed shows every column in the standard order.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS table_columns JSONB NOT NULL DEFAULT '{}';
