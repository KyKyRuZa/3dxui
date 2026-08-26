# AGENTS.md — Контекст проекта NoMoreBlocks VPN

> Назначение файла: дать следующему AI-агенту (новый чат) полный контекст проекта
> «на лету», без необходимости перечитывать всю переписку. Обновляй этот файл
> после каждого значимого изменения.

## 1. Что это за проект

**NoMoreBlocks VPN** — self-hosted платформа выдачи/продажи VPN-ключей на базе панели
**3x-ui** (Xray). Пользователь получает ключ через веб-дашборд или Telegram-бота.
Домен: `thenomoreblocks.com`.

- Модуль бэкенда: `github.com/ilyas/vpn-service/backend` (исторически такое имя пакета).
- Git-remote: `github.com/KyKyRuZa/3dxui` (ветка `master`).
- Статус (на 2026-08-26): **MVP ~85%**, сквозной happy-path рабочий, биллинг ЮKassa подключён (MVP, тестовый магазин).

## 2. Архитектура (docker-compose)

Сервисы в `docker-compose.yml`:
- `3xui` — панель VPN (источник клиентов/ключей), `XUI_ENABLE_FAIL2BAN=true`.
- `postgres` — БД (`vpn_user`/`vpn_db`, см. `.env`).
- `redis` — подключён, но пока не используется активно.
- `backend` — Go + Gin, порт `8080` (только `expose`, снаружи через nginx).
- `frontend` — React + Vite + TS, собирается в статику, раздаётся nginx.
- `nginx` — TLS-терминация, HSTS/CSP, rate-limit, прокси `/api` → backend.
- `bot` — Python + aiogram (`AutoColorsBot`), добавлен в compose позже (раньше его не было).

Ключевые каталоги бэкенда:
- `internal/handlers/` — `handler.go` (роуты), `auth.go`, `bot.go`, `subscription.go`.
- `internal/store/` — Postgres-доступ (`store.go`), модели в `internal/models/`.
- `internal/panel/` — клиент к REST API 3x-ui (`client.go`).
- `internal/db/` — подключение + миграции (`db.go`).
- `internal/config/` — viper + env (`config.go`).
- `internal/auth/`, `internal/middleware/`.

## 3. Что сделано (актуальное состояние)

### 3.1 Работа выдачи ключа в боте — РАБОТАЕТ
- `POST /api/bot/user` (`botEnsureUser`): провижн клиента в 3x-ui, добавление в группу,
  выдача `subscription_url` + `vless` + `singbox`.
- Идемпотентность: конфликт `username`/`telegram_id`/`panel_email` больше не даёт 500 —
  при коллизии подхватывается существующая запись.
- Возвращается поле `vless` (раньше только `links`).
- `singbox` извлекается «мягко»: если `subId` пустой и `http.Get` падает — не 500, а отдача
  того, что получили (`links`+`vless`).

### 3.2 Уведомления об истечении подписки — РАБОТАЕТ
- Подписки теперь **ограничены по времени**: `DEFAULT_SUBSCRIPTION_DAYS=2`,
  `EXPIRY_NOTIFY_DAYS=2`.
- Миграция: в `subscriptions` добавлены `expires_at TIMESTAMPTZ` и
  `last_expiry_notify_date DATE`.
- `GET /api/bot/notifications/expiring` реально отдаёт юзеров с `expires_at` в окне
  `[now, now+2 дня]` и `last_expiry_notify_date != CURRENT_DATE`, и сразу проставляет
  «уведомлено сегодня» (дедуп по суткам, переживает рестарт бота).
- Бот `send_expiry_notifications` шлёт «осталось ~N дн.»; фоновый цикл раз в `NOTIFY_INTERVAL`
  (по умолч. 3600с). Проверено на `tg_699469085`: `expires_at` = now+2д, уведомление приходит.

### 3.3 Реферальная программа — ФУНДАМЕНТ ГОТОВ (без биллинга)
- `users.referral_code` (уникальный; существующим юзерам backfill как `ref<id>`).
- Таблица `referrals` (`referrer_id`, `referred_id`, `status`, `reward_days`).
- Бот: `/start REFCODE` захватывает код → при первой покупке ключа передаёт в бэкенд →
  создаётся `referrals` (pending) + приведённому сразу **+2 дня** (`REFERRAL_SIGNUP_BONUS_DAYS`).
- Команда/кнопка `/referral` показывает ссылку `https://t.me/AutoColorsBot?start=<code>`,
  число приглашённых и начисленных дней.
- `CreditReferralReward` готов начислить рефереру **+7 дней** (`REFERRAL_REWARD_DAYS`)
  и теперь вызывается из биллинг-webhook (`handlers/billing.go`) при успешном платеже.

### 3.4 Прочие правки (важно для контекста)
- `BOT_API_SECRET` теперь проброшен в `backend` и сервис `bot` добавлен в compose
  (раньше `/api/bot/*` падал с 404/401).
- Коллизия `email` UNIQUE устранена: колонка стала nullable + частичный индекс
  `idx_users_email` (уникальность только для непустых); `CreateUser` пишет `NULL`.

### 3.5 Биллинг ЮKassa — РАБОТАЕТ (MVP, тестовый магазин)
- Клиент ЮKassa: `internal/billing/yookassa.go` (`CreatePayment`, `GetPayment`, Basic-auth,
  `Idempotence-Key`). API v3, `capture: true` (списание сразу при успехе).
- Конфиг: `YOOKASSA_SHOP_ID`, `YOOKASSA_SECRET_KEY`, `YOOKASSA_API_URL`
  (дефолт `https://api.yookassa.ru/v3`), `YOOKASSA_RETURN_URL`
  (дефолт `PUBLIC_ORIGIN`/`WEB_APP_URL`). Секретный ключ — только в `.env` (gitignored),
  в `.env.example` — заглушки.
- Таблицы `plans` (сеeded: `standard` 30 дн / 299.00 RUB, `pro` 90 дн / 799.00 RUB, группа `Free`)
  и `payments` (связь payment→user→plan, статус, идемпотентность по `payments.id`).
- Роуты (`handlers/billing.go`):
  - `GET /api/billing/plans` (auth) — список тарифов;
  - `POST /api/billing/create` (auth) — создаёт платёж, возвращает `confirmation_url`;
  - `POST /api/billing/webhook` (без авторизации) — обработка `payment.succeeded`.
- **Верификация webhook**: ЮKassa не подписывает уведомления, поэтому подлинность
  проверяем повторным запросом `GET /payments/{id}` через API (Basic-auth). Только при
  `status == "succeeded"` провижним/продлеваем подписку. Идемпотентность: повторная
  обработка тому же `payment.id` не начисляет дважды.
- На успехе: `provisionPlan` создаёт/продлевает клиента в 3x-ui на `plan.DurationDays`
  (аналог `renewSubscription`, но по длительности плана) → вызывается
  `CreditReferralReward(referredUserID)` (теперь НЕ dead code).
- ВАЖНО: webhook-URL (`https://thenomoreblocks.com/api/billing/webhook`) должен быть
  указан в профиле магазина ЮKassa и быть публично доступен по HTTPS. Для локальной
  отладки — ngrok/туннель.
- **Проверено end-to-end (2026-08-26, тестовый магазин `1268375`):** платёж
  `POST /api/billing/create` → webhook от IP ЮKassa (`payment.succeeded`) →
  `provisionPlan` создаёт клиента в 3x-ui → `expires_at` = now+30 дней. Смена на
  боевой магазин — только `YOOKASSA_SHOP_ID` + `YOOKASSA_SECRET_KEY` в `.env` и
  `docker compose up -d --build backend`; код менять не нужно.

## 4. Что в планах (не сделано)

1. **Перевод биллинга на боевой магазин ЮKassa**: сейчас используется тестовый
   (`YOOKASSA_SHOP_ID=1268375`, ключ `test_...`). После одобрения магазина заменить
   `YOOKASSA_SECRET_KEY` на боевой и проверить реальное списание.
2. **Модель тарифов (планов)**: базовая таблица `plans` уже есть (seeded standard/pro),
   но управление планами (CRUD, цены в админке) не реализовано. Пока правим руками в БД.
3. **One-shot уведомление в день истечения** («подписка закончилась — продлите»).
4. **Веб-дашборд**: показывать `expires_at` (API уже отдаёт) в `Subscription.tsx`
   (фронт пока не выводит срок). Кнопка покупки тарифа — **СДЕЛАНА**:
   `PricingCards` грузит планы с `/api/billing/plans` и по «Купить» создаёт платёж
   через `/api/billing/create`, открывает `confirmation_url` ЮKassa в новой вкладке;
   после оплаты webhook продлевает подписку.
5. **Чистка DEBUG-логов**: в `bot.go`/`auth.go`/`subscription.go`/`panel/client.go` много
   `fmt.Printf("DEBUG ...")` — заменить на zap.
6. **Админ-панель / управление планами** — опционально.

## 5. Как запускать и проверять

```bash
cp .env.example .env        # заполнить BOT_TOKEN, BOT_API_SECRET, DATABASE_URL, REDIS_URL,
                            # JWT_SECRET, PANEL_URL/USERNAME/PASSWORD/API_TOKEN, DEFAULT_INBOUND_IDS
docker compose up -d --build backend bot
docker compose logs -f bot
```

Проверка выдачи ключа ботом (изнутри сети контейнеров):
```bash
docker compose exec backend sh -c 'wget -qO- --post-data="{\"telegram_id\":12345,\"first_name\":\"Test\"}" \
  --header="X-Bot-Secret: $BOT_API_SECRET" --header="Content-Type: application/json" \
  http://localhost:8080/api/bot/user'
```

Проверка уведомлений (БД):
```bash
docker compose exec postgres psql -U vpn_user -d vpn_db \
  -c "SELECT panel_email, expires_at, last_expiry_notify_date FROM subscriptions WHERE panel_email='tg_699469085';"
```
Тест в Telegram: `/buy` → `/notify` (или дождаться фонового цикла) → приходит напоминание.

Рефералы (тест без ЮKassa): у реферера `/referral` → ссылка; с другого аккаунта открыть
`https://t.me/AutoColorsBot?start=<code>` → `/buy` → новому юзеру +2 дня, у реферера
счётчик «Приглашено» +1. Вознаграждение рефереру (+7 дней) — только после платежа.

## 6. Подводные камни (gotchas)

- Бэкенд в compose не проброшен на хост; curl-тесты делать изнутри сети (`docker compose exec backend`)
  или через публичный домен (nginx).
- Логи бэкенда НЕ содержат `DEBUG`-строк при «тихих» 500 (generic `internal error` без принта) —
  если видишь 500 без DEBUG, смотри ветки `CreateUser`/`SetTelegramID`/`CreateSubscription`/`HashPassword`.
- `panel.UpdateClient` заменяет строку клиента целиком — при продлении ОБЯЗАТЕЛЬНО передавать
  `email`, `subId`, `expiryTime`, `enable`, `inboundIds`, иначе клиент отвяжется от inbounds / сменится subId.
- `openapi.json` в корне — это спецификация самой панели 3x-ui, НЕ нашего API.
- JWT-ключи эфемерны, если не задан `JWT_PRIVATE_KEY` (токены сбросятся при рестарте).
- Миграции в `db.go` написаны как `CREATE TABLE IF NOT EXISTS` + `ALTER ... IF NOT EXISTS`
  (идемпотентны, безопасны для уже созданной БД, включая ослабление старых NOT NULL).

## 7. Быстрый индекс файлов для правок

- Роуты бота/API: `backend/internal/handlers/handler.go`
- Биллинг (plans/payments/webhook/provision): `backend/internal/handlers/billing.go`
- Клиент ЮKassa: `backend/internal/billing/yookassa.go`
- Логика бота (ensure/renew/referral/expiring): `backend/internal/handlers/bot.go`
- Провижн/активация подписки (веб): `backend/internal/handlers/subscription.go`
- Клиент 3x-ui (AddClient/GetClient/UpdateClient/Groups): `backend/internal/panel/client.go`
- Миграции БД: `backend/internal/db/db.go`
- Модели: `backend/internal/models/models.go`
- Store-методы (включая рефералы/начисления/планы/платежи): `backend/internal/store/store.go`
- Конфиг (дни подписки/уведомлений/рефералов/юкасса): `backend/internal/config/config.go`
- Бот (aiogram): `bot/main.py`
- Деплой: `docker-compose.yml`, `bot/Dockerfile`, `nginx/conf.d/default.conf`
- Фронт (биллинг-кнопка): `frontend/src/api/billing.ts`, `frontend/src/components/PricingCards.tsx`
