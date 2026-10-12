-- Регулярная сводка на почту:
-- {"enabled": true, "email": "", "period": "weekly", "time": "09:00",
--  "sections": ["stats", "team_load", "distribution", "deadlines"], "overdue_list": true}.
-- email пуст — письмо уходит на адрес из профиля. period: daily — каждый
-- день, weekly — по понедельникам, monthly — 1-го числа.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS email_report JSONB NOT NULL DEFAULT '{}';

-- Когда сводка ушла в последний раз. При включении сюда ставится момент
-- включения: первая сводка уходит в ближайший срок по расписанию, а не сразу.
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS email_report_sent_at TIMESTAMPTZ;
