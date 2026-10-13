-- Часовой пояс, в котором пользователю показывается время (имя IANA, например
-- Asia/Yekaterinburg). Пустая строка — не задан.
-- user_settings.timezone — выбор самого пользователя в настройках интерфейса;
-- users.default_timezone — пояс, заданный администратором в карточке пользователя.
-- Действует первый заданный: свой, затем от администратора, затем пояс браузера.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS default_timezone VARCHAR(64) NOT NULL DEFAULT '';
