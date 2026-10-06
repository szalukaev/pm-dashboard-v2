# PM Dashboard V3 — Deployment Guide

## Системные требования

| Компонент | Минимум | Рекомендуется |
|-----------|---------|---------------|
| CPU | 2 vCPU | 4 vCPU |
| RAM | 4 GB | 8 GB |
| Диск | 20 GB SSD | 50 GB SSD |
| ОС | Linux x86_64 (Ubuntu 22.04+) | |
| Docker | 24+ | |
| Docker Compose | v2 | |
| PostgreSQL | 16+ (на хосте) | |

## Архитектура

PostgreSQL развёрнут **на хосте** (не в Docker Compose). Docker Compose запускает backend (Go), frontend (Vue 3 + nginx) и Redis (с паролем, только внутри compose-сети). SQLite хранится локально на хосте и используется для первоначальной настройки через Setup Wizard.

```
┌─────────────────────────────────────────────────────┐
│  Docker Compose (сеть backend)                       │
│  ┌─────────────┐     ┌──────────────┐  ┌─────────┐  │
│  │   Nginx     │────▶│   Backend    │─▶│  Redis  │  │
│  │   (port 82) │     │   (Go)       │  │ requirepass │
│  │  Static     │     │  REST API    │  │ без порта │ │
│  │  files      │     │  WebSocket   │  └─────────┘  │
│  └─────────────┘     └──────┬───────┘               │
└─────────────────────────────┼───────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
     ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
     │  PostgreSQL   │ │   Redmine    │ │              │
     │  (port 5432)  │ │  (external)  │ │              │
     │   на хосте    │ │              │ │              │
     └──────────────┘ └──────────────┘ └──────────────┘
```

## Установка

### 1. Подготовка PostgreSQL на хосте

Установите PostgreSQL 16+ на сервере:

```bash
# Ubuntu/Debian
sudo apt-get install postgresql-16

# Создайте базу данных и пользователя
sudo -u postgres psql
CREATE DATABASE pm_dashboard;
CREATE USER pm_user WITH PASSWORD 'ваш_пароль';
GRANT ALL PRIVILEGES ON DATABASE pm_dashboard TO pm_user;
\q
```

Убедитесь что PostgreSQL принимает подключения из Docker-сети:
- В `postgresql.conf`: `listen_addresses = '*'`
- В `pg_hba.conf` добавьте: `host all all 172.16.0.0/12 md5`

Перезапустите PostgreSQL: `sudo systemctl restart postgresql`

### 2. Клонирование репозитория

```bash
git clone <repository-url> pm-dashboard
cd pm-dashboard
```

### 3. Настройка окружения

Скопируйте `.env.example` в `.env`:

```bash
cp .env.example .env
```

Переменные окружения (`.env`):
- `APP_PORT` — порт backend (по умолчанию 8080)
- `DATA_DIR` — директория для SQLite конфигурации
- `REDIS_PASSWORD` — **обязателен**: пароль Redis (compose не стартует без него)
- `REDIS_URL` — `redis://:<пароль>@redis:6379/0` (пароль должен совпадать с `REDIS_PASSWORD`)
- `SECURE_COOKIES` — `true` при работе за HTTPS (иначе `false`)
- `FRONTEND_ORIGIN` — (опционально) дополнительный Origin для WebSocket/CORS

> **BREAKING (обновление со старых версий):** Redis теперь запускается внутри
> compose с `--requirepass` и **не публикует** порт 6379 на хост. В `.env`
> обязательно добавить `REDIS_PASSWORD`. Если на хосте стоял свой Redis —
> он больше не используется.

Пароль PostgreSQL, URL Redmine и другие credentials вводятся через Setup Wizard при первом запуске и сохраняются в SQLite (`data/config.db`).

### 4. Запуск через Docker Compose

```bash
docker-compose up -d
```

Сервисы в Docker Compose:
- `redis` — Redis 7 с паролем (кеш и сессии, только внутренняя сеть)
- `backend` — Go API сервер (порт 8080)
- `frontend` — Vue 3 + nginx (порт 82)

PostgreSQL **не** в Docker Compose — работает на хосте.

### 5. Первый запуск

Откройте `http://<server-ip>:82` в браузере. Запустится Setup Wizard:

1. **Шаг 1: Подключение к БД** — укажите DSN PostgreSQL (например `postgres://pm_user:password@host.docker.internal:5432/pm_dashboard`), нажмите «Проверить доступ»
2. **Шаг 2: Подключение к Redmine** — укажите URL и API-ключ
3. **Шаг 3: Создание администратора** — имя пользователя и пароль

Все введённые credentials сохраняются в SQLite (`data/config.db`), а не в .env.

### 6. Проверка статуса

```bash
curl http://localhost:82/api/status
```

Ответ:
```json
{
  "version": "3.0.0",
  "status": "ok",
  "is_setup": true,
  "server_time": "2026-01-15T10:00:00Z"
}
```

## Резервное копирование

### PostgreSQL (на хосте)

```bash
pg_dump -U pm_user pm_dashboard > backup_$(date +%Y%m%d).sql
```

### SQLite конфигурация

```bash
cp ./data/config.db backup_config_$(date +%Y%m%d).db
```

## Обновление

1. Остановите контейнеры: `docker-compose down`
2. Обновите код: `git pull`
3. Пересоберите: `docker-compose build`
4. Запустите: `docker-compose up -d`

Миграции применяются автоматически при старте backend.

## Мониторинг

### Health checks

- Backend: `GET /api/setup/status`
- Frontend: `GET /` (HTTP 200)
- PostgreSQL (на хосте): `pg_isready -U pm_user -d pm_dashboard`
- Redis (в compose): `docker compose exec redis redis-cli -a "$REDIS_PASSWORD" ping`

### Логи

```bash
docker-compose logs -f backend
docker-compose logs -f frontend
```

## Безопасность

- Все пароли хранятся в bcrypt
- Credentials (DB, Redmine) — в SQLite, не в .env; секреты в API отдаются маской
- Сессии — в Redis с TTL 7 дней (Redis с паролем, без публикации порта)
- Rate-limiting: 5 попыток входа / 15 мин на логин, 20 / 15 мин на IP
- CORS: белый список origins + `Vary: Origin`
- WebSocket `/ws` — только с валидной сессией
- Секреты в ответах API маскируются (`••••••••`, пароль в DSN → `****`)
- Все внешние зависимости локально (без CDN)
