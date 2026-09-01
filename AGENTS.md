#rjl AGENTS.md — Контекст проекта NoMoreBlocks VPN

> Назначение файла: дать следующему AI-агенту (новый чат) полный контекст проекта
> «на лету», без необходимости перечитывать всю переписку. Обновляй этот файл
> после каждого значимого изменения.

## 1. Что это за проект

**NoMoreBlocks VPN** — self-hosted платформа выдачи/продажи VPN-ключей на базе панели
**3x-ui** (Xray). Пользователь получает ключ через веб-дашборд или Telegram-бота.
Домен: `thenomoreblocks.com`.

- Модуль бэкенда: `github.com/ilyas/vpn-service/backend` (исторически такое имя пакета).
- Git-remote: `github.com/KyKyRuZa/3dxui` (ветка `master`).
- Статус (на 2026-09-01): регистрация/вход на сайте — по username/password или через Telegram
  (deep-link или код); универсальная очередь уведомлений `bot_notifications`
  покрывает все сценарии бот↔сайт↔БД; баг атрибуции рефералов исправлен. Проверено
  end-to-end: deep-link авторизация, уведомления о продлении/рефералах/оплате, вход по коду.
  Остаётся тестовый запуск оплаты в тестовом магазине ЮKassa и CRUD тарифов.

## 2. Архитектура (docker-compose)

Сервисы в `docker-compose.yml`:
- `3xui` — панель VPN (источник клиентов/ключей), `XUI_ENABLE_FAIL2BAN=true`.
- `postgres` — БД (`vpn_user`/`vpn_db`, см. `.env`).
- `redis` — подключён, используется для verification codes (bind/login через Telegram).
- `backend` — Go + Gin, порт `8080` (только `expose`, снаружи через nginx).
- `frontend` — React + Vite + TS, собирается в статику, раздаётся nginx.
- `nginx` — TLS-терминация, HSTS/CSP, rate-limit, прокси `/api` → backend.
- `bot` — Python + aiogram (`AutoColorsBot`), добавлен в compose позже (раньше его не было).

Ключевые каталоги бэкенда:
- `internal/handlers/` — `handler.go` (роуты), `auth.go`, `bot.go`, `subscription.go`.
- `internal/store/` — Postgres-доступ (`store.go`), Redis-доступ (`store_redis.go`), модели в `internal/models/`.
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
- Бот `send_expiry_notifications` шлёт «осталось ~N дн.» (в последний день — более срочная
  «Последний день подписки!»), а `send_expired_notifications` (ранее НЕ был подключён к циклу)
  теперь тоже вызывается из `notification_loop` и шлёт навязчивую «купите ключ» рассылку
   ежедневно в течение 7 дней после истечения (`backend_expired(hours=168)`). Дедуп по
  `last_expired_notify_date` (раз в сутки, переживает рестарт).
- Во все pushy-сообщения (истечение, последний день, `/status`, `callback status`) добавлен
  **реферальный якорь**: «пригласите друга — получите +7 дней бесплатно» + готовая
  `t.me/AutoColorsBot?start=<code>` (ссылка подтягивается через `backend_referral`).
- Фоновый цикл раз в `NOTIFY_INTERVAL` (по умолч. 3600с). Проверено на `tg_699469085`:
  `expires_at` = now+2д, уведомление приходит.

### 3.3 Реферальная программа — ФУНДАМЕНТ ГОТОВ (без биллинга)
- `users.referral_code` (уникальный; существующим юзерам backfill как `ref<id>`).
- Таблица `referrals` (`referrer_id`, `referred_id`, `status`, `reward_days`).
- Бот: `/start REFCODE` захватывает код → при первой покупке ключа передаёт в бэкенд →
  создаётся `referrals` (pending) + приведённому сразу **+2 дня** (`REFERRAL_SIGNUP_BONUS_DAYS`).
- Команда/кнопка `/referral` показывает ссылку `https://t.me/AutoColorsBot?start=<code>`,
  число приглашённых и начисленных дней.
- `CreditReferralReward` готов начислить рефереру **+7 дней** (`REFERRAL_REWARD_DAYS`)
  и теперь вызывается из биллинг-webhook (`handlers/billing.go`) при успешном платеже.
- **Веб-рефералы**: добавлен `GET /api/referral` (JWT, `handlers/bot.go:webReferral`),
  зеркалит бот-эндпоинт и отдаёт `referral_code`, `invited`, `earned_days`, `bot_username`.
  Фронт `frontend/src/components/Referral.tsx` строит `t.me/<bot_username>?start=<code>`,
  копирует ссылку и показывает счётчики. `bot_username` берётся из `BOT_USERNAME` (дефолт `AutoColorsBot`).
- **БАГ (исправлен 2026-08-31):** атрибуция реферала работала только для **совсем новых**
  юзеров — запись `referrals` и `+2 дня` приведённому были внутри ветки «нет подписки»
  в `botEnsureUser` (`handlers/bot.go`). Существующий аккаунт, кликнувший реф-ссылку
  впервые, НЕ приписывался рефереру. Исправлено: атрибуция вынесена из ветки «новый
  юзер», выполняется для любого юзера с валидным `referral_code`, один раз на пару
  (через `store.GetReferral`, `ON CONFLICT DO NOTHING`), и размещена ПОСЛЕ `renewSubscription`,
  чтобы бонус не перетирался продлением. Добавлен `store.GetReferral`. Проверка на сервере —
  скрипт `scripts/verify_referral.sh` (создаёт реферера+приведённого, проверяет строку в БД).
  Все `go test ./...` проходят.

### 3.4 Прочие правки (важно для контекста)
- `BOT_API_SECRET` теперь проброшен в `backend` и сервис `bot` добавлен в compose
  (раньше `/api/bot/*` падал с 404/401). Без `BOT_API_SECRET` в `.env` middleware
  `BotRequired` возвращает 404, а в логах контейнера видно `The "BOT_API_SECRET" variable is not set`.
- Коллизия `email` UNIQUE устранена: колонка стала nullable + частичный индекс
  `idx_users_email` (уникальность только для непустых); `CreateUser` пишет `NULL`.
- **БАГ (исправлен 2026-08-29):** `models.User.Email` был `string`, а в БД `email` —
  NULL у бот-пользователей. `lib/pq` падал на `Scan` (`converting NULL to string is
  unsupported`), из-за чего `GetUserByTelegramID`/`scanUser` возвращали ошибку, а
  `botGetUser`/`botReferral` отдавали 404 и бот «не обновлялся». Исправлено:
  `User.Email` → `sql.NullString`, `Public().Email` берёт `.String`. Аналогично уже
  были nullable-safe `panel_username`/`panel_uuid`/`referral_code`.
- **Очистка логов**: последний `fmt.Println` в `internal/auth/jwt.go` заменён на
  `zap.SugaredLogger.Warn`; `NewTokenService` теперь принимает логгер из `cmd/main.go`.
  В бэкенде больше нет `fmt.Print*`; `log.Fatalf` в `main.go` остаются только на
  раннем старте до инициализации zap и не содержат ПДн.
- **Важно:** `/api/bot/referral` — это POST, не GET. При вызове из бота
  `backend_referral()` отправляет POST с `{"telegram_id": ...}` (`bot/main.py:58-68`).
  Ручной тест: `curl -X POST -H "X-Bot-Secret: ..." -d '{"telegram_id":699469085}' ...`.

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
- **Фронт знает про тестовый магазин**: бэкенд отдаёт `GET /api/config`
  (`{yookassa_test_mode}`; `true`, если `YOOKASSA_SECRET_KEY` начинается с `test_` или
  `YOOKASSA_TEST_MODE=true`), и `PricingCards` показывает жёлтый бейдж «🧪 Тестовый режим
  оплаты». Кнопки «Купить» уже привязаны к ЮKassa через `/api/billing/create`.

### 3.6 Уведомления бота — ВСЕ сценарии (актуально 2026-08-31)

Бот должен оповещать пользователя обо всех значимых событиях в связке бот↔сайт↔БД.
Реализовано два механизма:

1. **Опрос подписок** (поллинг из `notification_loop`, `NOTIFY_INTERVAL` по умолч. 3600с):
   - `GET /api/bot/notifications/expiring` → «🔔 Подписка скоро истечёт» (`send_expiry_notifications`).
   - `GET /api/bot/notifications/expired` → «🚨 Доступ перекрыт» (`send_expired_notifications`, окно 168ч).
   - `GET /api/bot/notifications/renewed` → «✅ Подписка продлена!» (`send_renewal_notifications`,
     пишется из `provisionPlan` через `renewal_notifications`).

2. **Универсальная очередь `bot_notifications`** (новое, 2026-08-31): таблица
   `(telegram_id, kind, data JSONB, ref_key UNIQUE(kind,ref_key), notified_at)`. Любое
   бэкенд-событие кладёт строку; бот опрашивает `GET /api/bot/notifications/pending`
   (`botNotifications` → `send_bot_notifications`), рендерит текст по `kind` и помечает
   доставленным. Идемпотентность по `ref_key` → повторы вебхука не шлют дубль.
   Покрытые `kind`:
   - `referral_signup` — рефереру: «друг зарегистрировался по вашей ссылке» (пишется в
     `botEnsureUser` при создании `referrals` pending; `ref_key=signup:<referrer>:<referred>`).
   - `referral_reward` — рефереру: «вам начислено +N дней» (пишется в `CreditReferralReward`
     после `CompleteReferral`; `ref_key=reward:<referrer>:<referred>`).
   - `payment_failed` — юзеру: «оплата не прошла» (пишется в `billingWebhook` на
     `payment.canceled`; `ref_key=payfail:<payment_id>`).

**Интерактивные ответы** (не рассылка, бот отвечает прямо в чате): выдача ключа (`/buy`),
статус (`/status`), реф-ссылка (`/referral`), инструкция/починка доступа (`/fix`, `/instructions`).
Их в очередь НЕ кладём — пользователь уже в диалоге.

Добавить новый сценарий = положить строку в `bot_notifications` + добавить ветку в
`render_bot_notification` (`bot/main.py`); эндпоинт и цикл уже всё разберут.

### 3.7 Аутентификация — username/password + Telegram (2026-09-01)

- **Username/password**: добавлены `POST /api/auth/register` и `POST /api/auth/login`.
  `users.password_hash` хранит реальный bcrypt-хеш (не placeholder).
- **Telegram deep-link**: `POST /api/auth/telegram/link` → `GET /api/auth/telegram/link/<token>`.
  Токены живут 5 минут, таблица `telegram_login_tokens`.
- **Telegram code login**: пользователь отправляет `/link` боту → получает 8-значный код →
  вводит на сайте. Коды хранятся в Redis с TTL 300с.
- **Telegram bind**: авторизованный пользователь нажимает «Привязать Telegram» в Settings →
  получает код → переходит в бота по ссылке `t.me/Bot?start=bind-XXXXXXXX`.
- Фронт `AuthForm`: поля username/password + кнопка «Войти через Telegram» + ввод кода из бота.
- **Telegram Login Widget** (`auth.VerifyTelegramWidget`, `POST /api/auth/telegram/widget`)
  был реализован, но ЗАМЕНЁН на deep-link (виджет требовал `/setdomain` у @BotFather и
  имел проблемы с CSP). Код виджета остался в кодовой базе как резервный — можно удалить,
  если не понадобится.

### 3.8 Соответствие 152-ФЗ (2026-09-01)

- Политика конфиденциальности: `frontend/src/pages/Privacy.tsx` (доступна по `/privacy`)
- Сбор согласия при регистрации: checkbox в `AuthForm.tsx` + запись в `consent_records`
- Экспорт данных: `GET /api/user/data-export` (все данные пользователя)
- Удаление аккаунта: `DELETE /api/user` (каскадное удаление всех данных)
- Cookie consent banner: `frontend/src/components/CookieConsent.tsx`
- Таблица `consent_records` для хранения записей о согласии
- Права субъекта: доступ к данным, удаление, отзыв согласия
- **TODO**: необходимо назначить ответственного за обработку ПД и опубликовать контакты

## 4. Безопасность и исправления (актуально 2026-09-01)

### 4.1 Исправленные уязвимости
- **Race condition в вебхуке ЮKassa**: добавлен атомарный `ClaimPayment` (`UPDATE ... WHERE status NOT IN`),
  предотвращает двойное продление подписки при параллельных вебхуках.
- **Race condition в рефералах**: `CreateReferral` теперь возвращает `(bool, error)`,
  бонус начисляется только если именно этот запрос создал запись.
- **initData replay protection**: добавлена проверка `auth_date` (max 24 часа) в `ValidateTelegramInitData`.
- **TLS верификация**: `InsecureSkipVerify` теперь включается только через `PANEL_INSECURE_SKIP_VERIFY=true`.
- **SameSite cookie**: refresh token cookie теперь с `SameSite=Strict` (защита от CSRF).
- **Панель аутентификация**: добавлен `cookiejar` для сохранения сессионных cookies.
- **Дублирование уведомлений**: добавлен атомарный `ClaimBotNotifications` с `FOR UPDATE SKIP LOCKED`.
- **parseInt64**: теперь возвращает ошибку вместо silencе 0.
- **JWT_SECRET**: убран из required config (не используется, JWT работает на EC ключах).
- **renewSubscription**: теперь продляет от текущего expiry, а не сбрасывает оставшиеся дни.
- **Expired notifications**: параметр `hours` теперь парсится из query (было 24ч, бот вызывает 168ч).

### 4.2 Тесты (2026-09-01)
- **Backend**: 6 пакетов, ~80+ тестов (auth, jwt, middleware, handlers, billing, store, utils).
- **Frontend**: 33 теста (AuthForm, PricingCards, authStore, Subscription, Referral).
- **Bot**: 53 теста (notifications, referrals, login token, bind code, login code).
- Все тесты проходят: `go test ./...`, `npm test -- --run`, `python3 -m pytest test_bot.py`.

## 5. Как запускать и проверять

```bash
cp .env.example .env        # заполнить BOT_TOKEN, BOT_API_SECRET, DATABASE_URL, REDIS_URL,
                            # PANEL_URL/USERNAME/PASSWORD/API_TOKEN, DEFAULT_INBOUND_IDS
                            # JWT_PRIVATE_KEY (для постоянных сессий)
docker compose up -d --build backend bot
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

Проверка verification codes:
```bash
# Генерация кода для входа
curl -X POST https://thenomoreblocks.com/api/codes/login \
  -H "Content-Type: application/json" \
  -d '{"telegram_id": 699469085}'

# Вход по коду
curl -X POST https://thenomoreblocks.com/api/auth/verify-login-code \
  -H "Content-Type: application/json" \
  -d '{"code": "16294059"}'
```

## 6. Подводные камни (gotchas)

- Бэкенд в compose не проброшен на хост; curl-тесты делать изнутри сети (`docker compose exec backend`)
  или через публичный домен (nginx).
- Логи бэкенда НЕ содержат `DEBUG`-строк при «тихих» 500 (generic `internal error` без принта) —
  если видишь 500 без DEBUG, смотри ветки `CreateUser`/`SetTelegramID`/`CreateSubscription`/`HashPassword`.
- `panel.UpdateClient` заменяет строку клиента целиком — при продлении ОБЯЗАТЕЛЬНО передавать
  `email`, `subId`, `expiryTime`, `enable`, `inboundIds`, иначе клиент отвяжется от inbounds / сменится subId.
- `openapi.json` в корне — это спецификация самой панели 3x-ui, НЕ нашего API.
- JWT-ключи эфемерны, если не задан `JWT_PRIVATE_KEY` (токены сбросятся при рестарте).
  **Последствие при деплое:** после каждой пересборки `backend` (`docker compose up -d
  --build backend`) ключ меняется → все существующие сессии/refresh-токены становятся
  недействительны (в логах/браузере — шквал 401 на `/api/auth/refresh`, `/api/billing/plans`),
  пользователям нужно просто перелогиниться. Чтобы сессии переживали рестарты — однократно
  сгенерировать EC P256 PEM (`openssl ecparam -name prime256v1 -genkey -noout`) и задать
  `JWT_PRIVATE_KEY` в `.env` + пробросить `JWT_PRIVATE_KEY: ${JWT_PRIVATE_KEY}` в
  `backend.environment` compose-файла, затем пересобрать backend.
- **BOT_API_SECRET обязателен для работы бот-эндпоинтов:** если переменная не задана,
  middleware `BotRequired` возвращает 404, а в логах видно `The "BOT_API_SECRET" variable is not set`.
  Добавь `BOT_API_SECRET` в `.env` и пересобери `backend` и `bot`.
- **DNS-инъекция РКН:** домен `thenomoreblocks.com` может блокироваться на уровне провайдера
  (клиенты получают битый DNS на WiFi, при этом мобильный интернет/через VPN работает).
  Панель 3x-ui имеет фикс: явно задать «Server IP» = `2.26.138.90` (тогда клиентские конфиги
  используют IP и туннель не зависит от DNS клиента). Для доступа к сайту без VPN у покупателей
  можно включить DoH (`dns.adguard.com`, `8.8.8.8`) или использовать скрипты `/fix` в боте
  (прописывают `hosts`). На сервере `dig` с `8.8.8.8`/`77.88.8.8` даёт корректный IP;
  `1.1.1.1` для этого домена может отдавать устаревшие адреса — не рекомендовать покупателям.
- При пересборке только `frontend` **nginx рестартовать не нужно**: nginx раздаёт статику из
  volume `frontend_dist`, который перезаписывается `frontend`-контейнером; конфиг
  nginx (`nginx/conf.d`) при этом не меняется.
- Миграции в `db.go` написаны как `CREATE TABLE IF NOT EXISTS` + `ALTER ... IF NOT EXISTS`
  (идемпотентны, безопасны для уже созданной БД, включая ослабление старых NOT NULL).
- `/api/bot/referral` — это POST, не GET. При вызове из бота отправляется POST с `{"telegram_id": ...}`.
- **БАГ frontend entrypoint (исправлен 2026-08-31):** `frontend/docker-entrypoint.sh`
  изначально делал `rm -rf /app/dist`, но `/app/dist` — это mount point (volume `frontend_dist`),
  и `rm` падал с `Resource busy`. Исправлено на `find /app/dist -mindepth 1 -delete`,
  что удаляет содержимое, но оставляет сам mount point. Если увидите в логах
  `rm: can't remove '/app/dist': Resource busy` — пересоберите frontend.
- **Redis для verification codes**: коды хранятся в Redis с TTL 300с. Если Redis недоступен —
  коды не генерируются (ошибка 500). Проверка: `docker compose exec redis redis-cli ping`.

## 7. Быстрый индекс файлов для правок

- Роуты бота/API: `backend/internal/handlers/handler.go`
- Биллинг (plans/payments/webhook/provision): `backend/internal/handlers/billing.go`
- Клиент ЮKassa: `backend/internal/billing/yookassa.go`
- Логика бота (ensure/renew/referral/expiring): `backend/internal/handlers/bot.go`
- Провижн/активация подписки (веб): `backend/internal/handlers/subscription.go`
- Клиент 3x-ui (AddClient/GetClient/UpdateClient/Groups): `backend/internal/panel/client.go`
- Миграции БД: `backend/internal/db/db.go`
- Модели: `backend/internal/models/models.go`
- Store-методы (Postgres): `backend/internal/store/store.go`
- Store-методы (Redis codes): `backend/internal/store/store_redis.go`
- Конфиг (дни подписки/уведомлений/рефералов/юкасса): `backend/internal/config/config.go`
- Бот (aiogram): `bot/main.py`, `bot/add_hosts_windows.bat`, `bot/add_hosts_linux_macos.sh`
- Аутентификация (deep-link + WebApp): `backend/internal/auth/telegram.go`, `backend/internal/handlers/auth.go`
- Деплой: `docker-compose.yml`, `bot/Dockerfile`, `frontend/docker-entrypoint.sh`, `nginx/conf.d/default.conf`
- Фронт (биллинг-кнопка): `frontend/src/api/billing.ts`, `frontend/src/components/PricingCards.tsx`
- Фронт (подписка/рефералы/тест-бейдж): `frontend/src/pages/Subscription.tsx`,
  `frontend/src/components/Referral.tsx`, `frontend/src/api/referral.ts`,
  `frontend/src/api/config.ts`, `frontend/src/styles/Subscription.module.css`,
  `frontend/src/styles/Referral.module.css`
- Фронт (авторизация): `frontend/src/pages/AuthForm.tsx`, `frontend/src/api/auth.ts`,
  `frontend/src/store/auth.tsx`, `frontend/src/styles/Auth.module.css`
- Фронт (политика/cookies/settings): `frontend/src/pages/Privacy.tsx`,
  `frontend/src/components/CookieConsent.tsx`, `frontend/src/pages/Settings.tsx`,
  `frontend/src/styles/Privacy.module.css`, `frontend/src/styles/CookieConsent.module.css`,
  `frontend/src/styles/Settings.module.css`

## 8. Что осталось сделать (TODO)

1. **Перевод биллинга на боевой магазин ЮKassa**: сейчас используется тестовый
   (`YOOKASSA_SHOP_ID=1268375`, ключ `test_...`). После одобрения магазина заменить
   `YOOKASSA_SECRET_KEY` на боевой и проверить реальное списание.
2. **Модель тарифов (планов)**: базовая таблица `plans` уже есть (seeded standard/pro),
   но управление планами (CRUD, цены в админке) не реализовано. Пока правим руками в БД.
3. **Админ-панель / управление планами** — опционально.
4. **Назначить ответственного за обработку ПД** — требуется по 152-ФЗ, опубликовать контакты.
5. **Договоры с третьими лицами (DPA)** — документировать передачу данных ЮKassa и Telegram.
6. **Обход DNS-блокировки**: добавлена команда `/fix` и кнопка «🔧 Починить доступ к сайту» в боте.
