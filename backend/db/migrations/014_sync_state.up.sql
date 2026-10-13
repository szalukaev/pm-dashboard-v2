-- Когда каждый проект в последний раз читался из источника данных: любая
-- успешная загрузка и последняя полная. Хранится в базе, чтобы после
-- перезапуска синхронизация не читала всё заново, и по каждому проекту,
-- чтобы сбой одного проекта не заставлял перечитывать остальные.
CREATE TABLE IF NOT EXISTS sync_state (
    project_id INTEGER NOT NULL,
    data_source VARCHAR(50) NOT NULL DEFAULT 'redmine',
    last_sync_at TIMESTAMPTZ NOT NULL,
    last_full_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (project_id, data_source)
);
