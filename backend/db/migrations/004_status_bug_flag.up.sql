-- Bugs — отметка поверх группы статуса: статус-баг остаётся открытым (или
-- тестируемым), он лишь считается отдельно в аналитике.
ALTER TABLE statuses ADD COLUMN IF NOT EXISTS is_bug BOOLEAN NOT NULL DEFAULT false;

-- Исходное состояние: статусы, которые считались багами до появления
-- отметки (по названию). Отметки администратор правит в настройках.
UPDATE statuses SET is_bug = true WHERE LOWER(name) LIKE '%bug%';
