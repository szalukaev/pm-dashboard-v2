# PM Dashboard __VERSION__ — установка

Нужны Linux x86_64, Docker 24+ с Docker Compose v2 и PostgreSQL 16+
(на этом же сервере или отдельном). Интернет для установки не нужен:
все образы в архиве.

## 1. Распаковать и загрузить образы

    tar xzf pm-dashboard-__VERSION__.tar.gz
    cd pm-dashboard-__VERSION__
    docker load -i images.tar.gz

## 2. Настроить окружение

    cp .env.example .env

В `.env` обязательно задайте свой `REDIS_PASSWORD` (и тот же пароль в
`REDIS_URL`). Остальные параметры описаны в самом файле. Параметры базы и
Redmine вводятся в мастере настройки при первом входе.

Порт по умолчанию — 82; другой задаётся строкой `FRONTEND_PORT=8080` в `.env`.

## 3. Запустить

    docker compose up -d
    docker compose ps

Откройте `http://<адрес-сервера>:82` и пройдите мастер настройки.

## 4. Лицензия

«Настройки → Администрирование → Лицензия»: скопируйте отпечаток сервера,
отправьте поставщику, вставьте полученную лицензию и нажмите «Активировать».

## Обновление

    cd pm-dashboard-__VERSION__ && docker compose down && cd ..
    tar xzf pm-dashboard-<новая версия>.tar.gz
    cp pm-dashboard-__VERSION__/.env pm-dashboard-<новая версия>/
    mv pm-dashboard-__VERSION__/data pm-dashboard-<новая версия>/
    cd pm-dashboard-<новая версия>
    docker load -i images.tar.gz
    docker compose up -d

Каталог `data` (конфигурация и ключ шифрования) и база данных переносятся
как есть; миграции применяются при старте. Перед обновлением сделайте
резервную копию базы и каталога `data`.
