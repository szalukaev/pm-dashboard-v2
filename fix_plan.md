# План исправлений: PM Dashboard V3 (Go + Vue)

Репозиторий: https://github.com/szalukaev/pm-dashboard-v2
Стек: Go 1.22 / gorilla-mux / PostgreSQL / Redis / Vue 3 + Vite.

## Правила работы для исполнителя (прочитать первым)

1. НЕ менять публичные контракты API (имена роутов, структуры JSON-ответов) без явного указания ниже.
2. Каждый пункт — отдельный коммит с сообщением `fix: <краткое описание>`.
3. После каждого пункта выполнять: `go build ./...`, `go vet ./...`, `go test -race ./...`. Репозиторий должен собираться без ошибок.
4. Не делать «refactor ради refactor» — только указанные изменения и минимально необходимые правки рядом.
5. Добавляемый код — на русском/английском как в существующем стиле файла, комментарии к нетривиальным местам обязательны.
6. Порядок исправлений — строго по списку: сначала критичные (раздел A), потом высокие (B), потом средние (C).

---

# РАЗДЕЛ A. Критичные баги

## A1. Data race в MemorySessionStore → крах процесса

**Файл:** `backend/db/session_store.go`

**Проблема:** поле `sessions map[string]sessionEntry` читается и пишется из HTTP-хендлеров разных горутин без синхронизации. При отказе Redis (штатный fallback) два параллельных запроса → `fatal error: concurrent map read and map write`, процесс падает. Дополнительно протухшие сессии никогда не удаляются (утечка памяти).

**Исправление:**
1. Добавить в структуру `sync.RWMutex` и брать блокировку во всех методах (`Set` — `Lock`, `Get`/`Delete` — `Lock`/`RLock` соответственно).
2. Добавить фоновую чистку: конструктор `NewMemorySessionStore()` запускает горутину с `time.Ticker` (интервал 1 минута), которая удаляет просроченные записи. Горутину корректно завершать — канал `stop` + закрытие в методе `Close()` (добавить метод, вызывать его в graceful shutdown в `main.go`).

**Проверка:** `go test -race` на тесте с 100 параллельными `Set`/`Get`. Тест добавить в `backend/db/session_store_test.go`.

## A2. Дедлок WebSocket-hub

**Файл:** `backend/handlers/ws.go`

**Проблема 1 (дедлок):** в `run()`, ветка `case message := <-h.broadcast`, при ошибке записи клиенту выполняется `h.unregister <- client`. Канал небуферизованный, единственный читатель — сам `run()`, который сейчас занят этой веткой → hub блокируется навсегда после первого «битого» клиента. Заодно: из hub напрямую вызывается `client.WriteMessage`, при этом писать в `*websocket.Conn` может только одна горутина (write-гонка с `writePump`).

**Проблема 2 (отсутствие авторизации):** эндпоинт `/ws` смонтирован вне `RequireAuth`, `CheckOrigin` возвращает `true` для любого Origin.

**Исправление:**
1. В ветке broadcast hub пишет ТОЛЬКО в буферизованный канал клиента через `select { case client.send <- message: default: }`; клиенты, чей `send` переполнен, помечаются «мёртвыми» и удаляются из `h.clients` под `h.mu.Lock()` ПОСЛЕ цикла. Никаких отправок в `h.unregister` изнутри `run()`.
2. Добавить авторизацию WS: middleware/обёртка перед `h.ServeWS`, которая: читает куку `session_id`, проверяет сессию через `SessionStore.Get`, при невалидной сессии — `http.Error(w, "unauthorized", 401)`. Проверка Origin в `CheckOrigin`: разрешать только Origin, совпадающий с `http://localhost:82`, `http://localhost:5173` и значением из env `FRONTEND_ORIGIN` (если задан), иначе 403.
3. Добавить в `writePump` ping/pong (`SetPongHandler`, ping-интервал 30 с, pong-ожидание 10 с), чтение с лимитом (`SetReadLimit`).

**Проверка:** unit-тест: создать hub, подключить клиент с переполненным `send`, вызвать `BroadcastEvent` — hub продолжает работать (канал `broadcast` не блокируется, следующий броадкаст проходит).

## A3. IDOR в платежах — чужие счета доступны любому пользователю

**Файл:** `backend/handlers/payments.go`

**Проблема:** `ListInvoices`, `DeleteInvoice`, `PayInvoice`, `DownloadInvoice` работают с `contract_id`/`invoice id` БЕЗ проверки `user_id` владельца контракта. `UpdateContract`/`DeleteContract` проверяют (`AND user_id=$N`) — сделать единообразно.

**Исправление (везде `userID` брать из сессии, как в существующем коде):**
1. `ListInvoices` — добавить join и условие:
   ```sql
   SELECT i.id, i.number, i.issue_date, i.total_amount, i.vat_rate, i.paid_amount, i.status
   FROM invoices i JOIN contracts c ON i.contract_id = c.id
   WHERE i.contract_id = $1 AND c.user_id = $2
   ORDER BY i.issue_date DESC
   ```
2. `DeleteInvoice`, `DownloadInvoice`:
   ```sql
   DELETE FROM invoices WHERE id = $1 AND contract_id IN (SELECT id FROM contracts WHERE user_id = $2)
   ```
   и для скачивания — `SELECT` с тем же подзапросом; если 0 строк → 404 `INVOICE_NOT_FOUND`.
3. `PayInvoice` — сначала `SELECT i.total_amount, i.paid_amount FROM invoices i JOIN contracts c ... WHERE i.id=$1 AND c.user_id=$2`, при 0 строк → 404, потом вычисления и обновление.
4. `CreateContract` — в запросе организации добавить `AND user_id = $2` (сейчас наследует данные чужой организации).

**Проверка:** интеграционный тест: пользователь А создаёт организацию/контракт/счёт; пользователь Б по прямому URL/ID получает 404 на list/delete/pay/download.

## A4. Отрицательные платежи в PayInvoice + отсутствие транзакции

**Файл:** `backend/handlers/payments.go`, метод `PayInvoice`.

**Проблема:** нет проверки `payAmount > 0` — `{"amount": -100000}` уменьшает `paid_amount` ниже нуля и портит все агрегаты. UPDATE + INSERT выполняются без транзакции — при параллельных платежах возможна потеря/двойное списание (оба прочитали старый `paid_amount`).

**Исправление:**
1. После вычисления `payAmount`: `if payAmount <= 0 { utils.Error(w, http.StatusBadRequest, "INVALID_AMOUNT"); return }`; `if payAmount > remainder { 400 }` (remainder = totalObligation - paidAmount, уже есть в коде — используйте его, а не повторные вычисления).
2. Обернуть UPDATE `invoices` и INSERT в `contract_payments` в `sql.Tx` (`(*h.DB).Begin()` … `tx.Commit()`), при ошибке — `tx.Rollback()`.

**Проверка:** тесты: отрицательная сумма → 400; сумма больше остатка → 400; два параллельных платежа под `-race` итоговая сумма корректна.

## A5. Расхождение схемы БД: свежая установка ломается

**Файлы:** `backend/db/migrations/001_initial.up.sql`, `backend/db/postgres.go` (fallbackMigrationSQL), добавить `backend/db/migrations/002_fix_schema_drift.up.sql` / `.down.sql`.

**Проблема:** fallback-вариант содержит колонки, которых нет в файловой миграции: `users.display_name`, `users.avatar`, `users.last_login`, `user_settings.overdue_alerts`, `statuses.synced_at`, `priorities.synced_at`. Чистый деплой через файловые миграции → `auth.go` падает с «column does not exist» при логине.

**Исправление:**
1. Создать `002_fix_schema_drift.up.sql`:
   ```sql
   ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) DEFAULT '';
   ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT DEFAULT '';
   ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login TIMESTAMPTZ;
   ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS overdue_alerts BOOLEAN DEFAULT true;
   ALTER TABLE statuses ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
   ALTER TABLE priorities ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
   ```
   и соответствующий `.down.sql` (DROP COLUMN IF EXISTS).
2. Привести `001_initial.up.sql` к тому же виду (добавить колонки в CREATE TABLE) и синхронизировать `fallbackMigrationSQL` в `postgres.go` — после правки все три источника схемы должны быть идентичны по структуре. В `RunMigrations` механизм применения файлов уже есть (по имени `NNN_*.up.sql`) — новый файл подхватится автоматически.

**Проверка:** поднять чистый PostgreSQL (`docker compose up postgres`), запустить бэкенд, убедиться что логин работает и `SELECT column_name FROM information_schema.columns WHERE table_name='users'` содержит все три колонки.

---

# РАЗДЕЛ B. Высокий приоритет

## B1. Redis наружу без пароля (docker-compose.yml)

Убрать проброс порта `6379` наружу, добавить пароль и внутреннюю сеть:
```yaml
redis:
  image: redis:7-alpine
  command: redis-server --requirepass ${REDIS_PASSWORD:?set REDIS_PASSWORD}
  networks: [backend]
  # секция ports: УДАЛИТЬ
```
Бэкенд: URL Redis брать из env `REDIS_URL=redis://:<пароль>@redis:6379` (пароль не хардкодить). Добавить `networks: [backend]` для postgres/backend. Принять, что штатный fallback на in-memory теперь потокобезопасен (A1).

## B2. Маскировать секреты в ответах API

**Файл:** `backend/handlers/admin.go`.
`GetDataSourceConfig`: `redmine_api_key` и `basic_password` возвращать как `"••••••••"` (8 точек) если заданы, пустую строку если нет. `GetDBConfig`: пароль в DSN маскировать (парсить DSN, заменить password на `****`). Фронт (`frontend/src/views/SettingsAdmin.vue`) и так показывает эти поля — убедиться, что после маскирования не ломается отправка формы (бэкенд в `UpdateDataSourceConfig` должен игнорировать значение-маску: если пришло `••••••••`, сохранять старое значение).

## B3. Grace-период лицензии не стартует сам; дедупликация проверок

**Файлы:** `backend/handlers/license.go`, `backend/middleware/license.go`.

1. В `main.go` после создания хендлеров запустить фоновый тикер (каждые 6 часов) вызывающий `licenseHandler.RunPeriodicCheck()` — новый метод, который делает то же, что публичный `checkLicense`, но внутренний (не по HTTP). Grace-period теперь стартует без посещения страницы лицензии.
2. В `IsReadOnly()`/`check()` middleware: перед `go lc.check()` проверять флаг `checkRunning atomic.Bool` — если уже идёт, не спавнить новую горутину.
3. HWID: если `/etc/machine-id` не прочитан, брать значение из env `PM_HWID`; если и его нет — fallback на hostname, но ЛОГИРОВАТЬ предупреждение. Зафиксировать поведение в комментарии.

## B4. Таймауты HTTP/DB в тестовых эндпоинтах

**Файлы:** `backend/handlers/setup.go`, `backend/handlers/admin.go`.
В `TestDataSourceConfig`/`TestDataSource`/`TestDBConfig`: `http.Client{Timeout: 8*time.Second}`; для DSN добавить `connect_timeout=5` в строку подключения перед `Ping`. В `TestDataSourceConfig` валидировать схему URL: разрешить `http`/`https`, для `https` ок, для `http` — предупреждение в ответе (не блокировать).

## B5. Аудит-лог: IP, статус, лимит тела

**Файл:** `backend/middleware/audit.go`.
1. IP: `X-Real-IP`, затем первый адрес из `X-Forwarded-For`, затем `net.SplitHostPort(r.RemoteAddr)`.
2. Писать в БД захваченный `statusCode` (добавить колонку в INSERT — в `audit_logs` колонка есть).
3. Чтение тела: `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` перед `io.ReadAll`.
4. `before_state`: оставить `nil`, но убрать из запроса колонку `before_state` из INSERT или задокументировать в коде, что она резервная (чтобы не вводить в заблуждение).

## B6. Неверный НДС в GetStats

**Файл:** `backend/handlers/payments.go`.
Убрать `*1.025`. Считать: `SELECT total_amount, vat_rate FROM invoices WHERE contract_id IN (SELECT id FROM contracts WHERE user_id=$1)` → `totalDebt = Σ(total_amount * (1 + vat_rate/100)) - totalPaid`. `closedCount` привести к той же логике фильтров, что `ListContracts`.

## B7. Panic при недоступной БД → 503

**Файлы:** `backend/handlers/auth.go` (Login, Logout), `backend/middleware/auth.go` (RequireAuth), пройтись по остальным хендлерам.
Паттерн (как уже сделано в `tasks.go`):
```go
if h.DB == nil || *h.DB == nil {
    utils.Error(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE")
    return
}
```
В `RequireAuth` — то же перед запросом роли. Не должно остаться прямых `(*pgDB)` без проверки.

## B8. Энумерация пользователей по таймингу в Login

**Файл:** `backend/handlers/auth.go`.
При `sql.ErrNoRows` выполнять `bcrypt.CompareHashAndPassword` против заранее вычисленного фиктивного хэша (константа уровня пакета), затем тот же `INVALID_CREDENTIALS`/401. Унифицировать время ответа.

## B9. Cookie сессии: флаг Secure

**Файл:** `backend/handlers/auth.go`, метод Login.
`Secure: os.Getenv("SECURE_COOKIES") == "true"`. В `docker-compose.yml` и `.env.example` добавить `SECURE_COOKIES=false` (для продакшена за HTTPS — `true`).

---

# РАЗДЕЛ C. Средние

## C1. Rate limiter: добавить per-IP
В `auth.go` второй счётчик по IP (`r.RemoteAddr`/парсинг как в B5): 20 неудач/15 мин с IP. Хранить рядом с существующим, тот же mutex.

## C2. Обработка ошибок Exec/Atoi
По всем хендлерам: каждый `(*h.DB).Exec(...)` — с проверкой `err` и `res.RowsAffected()` (0 строк → 404 там, где ресурс должен существовать). Каждый `strconv.Atoi(mux.Vars(r)["id"])` — с проверкой ошибки → 400. Не рефакторить сверх этого.

## C3. Лимит тела JSON
Добавить middleware в `main.go` (перед роутами API): `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` для `Content-Type: application/json`.

## C4. Security-заголовки
Middleware в `main.go` (после CORS): `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`.

## C5. CORS: Vary: Origin
В `middleware/cors.go` при совпадении Origin добавить заголовок `Vary: Origin`.

## C6. Удалить или подключить LicenseView
`frontend/src/views/LicenseView.vue` существует без роута. Либо добавить роут `/license` (в `router/index.ts`, meta `{ requiresAuth: true }`), либо удалить файл. Решение: добавить роут (страница полезна для проверки лицензии).

## C7. Кэш setup-status на фронте
`frontend/src/router/index.ts`: запрос `/api/setup/status` выполнять один раз за сессию (модуль-переменная/флаг), не на каждую навигацию.

## C8. Нейминг
Лог «Starting PM Dashboard V3» уже корректен — дополнительно привести `docker-compose.yml` (поле `container_name`, если есть «v2») и заголовки README к V3.

## C9. Публичный GET /api/license/status
В `handlers/license.go` кэшировать результат проверки на 5 минут (sync.Mutex + поле в хендлере), чтобы публичный эндпоинт не делал чтение файлов/проверку подписи на каждый запрос.

---

# Финальная проверка (обязательно в конце)

1. `go build ./... && go vet ./... && go test -race ./...` — без ошибок.
2. Ручной сценарий: чистый `docker compose up` → setup-wizard → создание админа → логин → создать организацию/контракт/счёт → платёж → статистика.
3. Два браузера (два пользователя): проверить 404 на чужих счетах (A3).
4. `docker compose stop redis` → залогиниться параллельно из двух вкладок → процесс жив (A1).
5. `gosec ./...` — прогнать, новых HIGH/критических замечаний не должно появиться (существующие до правок не считать блокером, зафиксировать в отчёте).

## Отчёт исполнителя
В конце вывести список: пункт → коммит(ы) → как проверялось (команда/сценарий) → статус.
