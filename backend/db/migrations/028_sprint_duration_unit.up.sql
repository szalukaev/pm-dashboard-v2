-- В чём считается продолжительность спринта: рабочие дни (по производственному
-- календарю) или календарные. Сама продолжительность выводится из дат.
ALTER TABLE sprints ADD COLUMN IF NOT EXISTS duration_unit VARCHAR(10) NOT NULL DEFAULT 'workdays';
