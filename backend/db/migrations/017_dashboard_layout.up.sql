-- Как пользователь расположил виджеты вкладки «Аналитика»:
-- {"widgets": [{"id": "stats", "cols": 12, "collapsed": false}, ...],
--  "hidden_widgets": ["deadlines"]}.
-- Виджеты идут в порядке списка; cols — ширина в сетке из 12 колонок.
-- Пустой объект означает стандартную раскладку.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS dashboard_layout JSONB NOT NULL DEFAULT '{}';
