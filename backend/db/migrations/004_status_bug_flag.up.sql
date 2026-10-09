-- Bugs is a flag on top of the status group: a bug status is still an open
-- (or testing) one, it is only counted separately in the analytics.
ALTER TABLE statuses ADD COLUMN IF NOT EXISTS is_bug BOOLEAN NOT NULL DEFAULT false;

-- Starting point: the statuses that were counted as bugs before the flag
-- existed (by name). The administrator adjusts the marks in the settings.
UPDATE statuses SET is_bug = true WHERE LOWER(name) LIKE '%bug%';
