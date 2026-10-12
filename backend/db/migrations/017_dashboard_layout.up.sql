-- How the user arranged the widgets of the Analytics tab:
-- {"widgets": [{"id": "stats", "cols": 12, "collapsed": false}, ...],
--  "hidden_widgets": ["deadlines"]}.
-- The widgets go in the order of the list; cols is the width in the
-- 12-column grid. An empty object means the standard arrangement.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS dashboard_layout JSONB NOT NULL DEFAULT '{}';
