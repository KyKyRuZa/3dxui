package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func NewPostgres(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate creates the required tables if they do not exist yet.
func Migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
	id               BIGSERIAL PRIMARY KEY,
	username         TEXT UNIQUE NOT NULL,
	email            TEXT,
	password_hash    TEXT NOT NULL,
	is_active        BOOLEAN NOT NULL DEFAULT TRUE,
	telegram_id      BIGINT UNIQUE,
	panel_username   TEXT UNIQUE,
	panel_uuid       TEXT UNIQUE,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL AND email <> '';

-- Relax a pre-existing NOT NULL / UNIQUE email column for backward compatibility
-- (NULL is allowed for bot/Telegram-provisioned users; uniqueness is enforced
-- only for non-empty emails via the partial index above).
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

CREATE TABLE IF NOT EXISTS sessions (
	id                TEXT PRIMARY KEY,
	user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	refresh_token_hash TEXT NOT NULL,
	user_agent        TEXT,
	ip                TEXT,
	expires_at        TIMESTAMPTZ NOT NULL,
	created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS subscriptions (
	id              BIGSERIAL PRIMARY KEY,
	user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status          TEXT NOT NULL DEFAULT 'active',
	panel_email     TEXT NOT NULL UNIQUE,
	panel_sub_id    TEXT,
	group_name      TEXT NOT NULL DEFAULT 'Free',
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_panel_email ON subscriptions(panel_email);
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS last_expiry_notify_date DATE;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS last_expired_notify_date DATE;

ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code TEXT UNIQUE;

-- Backfill a referral code for users created before the column existed.
UPDATE users SET referral_code = 'ref' || id WHERE referral_code IS NULL;

CREATE TABLE IF NOT EXISTS referrals (
	id          BIGSERIAL PRIMARY KEY,
	referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	referred_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status      TEXT NOT NULL DEFAULT 'pending',
	reward_days INTEGER NOT NULL DEFAULT 0,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	completed_at TIMESTAMPTZ,
	UNIQUE (referrer_id, referred_id)
);

CREATE TABLE IF NOT EXISTS plans (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	duration_days INTEGER NOT NULL,
	price_minor   BIGINT NOT NULL DEFAULT 0,
	currency      TEXT NOT NULL DEFAULT 'RUB',
	group_name    TEXT NOT NULL DEFAULT 'Free'
);

CREATE TABLE IF NOT EXISTS discounts (
	id          TEXT PRIMARY KEY,
	code        TEXT NOT NULL UNIQUE,
	plan_id     TEXT REFERENCES plans(id),
	percent     INTEGER NOT NULL DEFAULT 0,
	fixed_minor BIGINT NOT NULL DEFAULT 0,
	starts_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	max_uses    INTEGER NOT NULL DEFAULT 0,
	used_count  INTEGER NOT NULL DEFAULT 0,
	is_active   BOOLEAN NOT NULL DEFAULT TRUE,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_discounts_code ON discounts(code);
CREATE INDEX IF NOT EXISTS idx_discounts_plan ON discounts(plan_id);

INSERT INTO plans (id, name, duration_days, price_minor, currency, group_name) VALUES
	('standard', 'Standard', 30, 29900, 'RUB', 'Free'),
	('pro', 'Pro', 90, 79900, 'RUB', 'Free')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS payments (
	id           TEXT PRIMARY KEY,
	user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	plan_id      TEXT NOT NULL REFERENCES plans(id),
	status       TEXT NOT NULL DEFAULT 'pending',
	amount_minor BIGINT NOT NULL DEFAULT 0,
	currency     TEXT NOT NULL DEFAULT 'RUB',
	created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);

CREATE TABLE IF NOT EXISTS renewal_notifications (
	id          BIGSERIAL PRIMARY KEY,
	user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	telegram_id BIGINT NOT NULL,
	expires_at  TIMESTAMPTZ NOT NULL,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	notified_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_renewal_notifications_pending ON renewal_notifications(user_id) WHERE notified_at IS NULL;

-- Generic bot notifications queue. Any backend event that should reach a user
-- via Telegram enqueues a row here; the bot polls pending rows and renders a
-- message by its kind. ref_key makes enqueue idempotent per (kind, event) so
-- webhook retries never produce duplicate pushes.
CREATE TABLE IF NOT EXISTS bot_notifications (
	id           BIGSERIAL PRIMARY KEY,
	telegram_id  BIGINT NOT NULL,
	kind         TEXT NOT NULL,
	data         JSONB NOT NULL DEFAULT '{}'::jsonb,
	ref_key      TEXT NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	notified_at  TIMESTAMPTZ,
	UNIQUE (kind, ref_key)
);
CREATE INDEX IF NOT EXISTS idx_bot_notifications_pending ON bot_notifications (created_at) WHERE notified_at IS NULL;

-- Telegram login tokens for the browser deep-link flow. A user on the website
-- requests a token, opens the bot with /start <token>, the bot reports the
-- user's Telegram id back, and the website polls until the token is claimed.
CREATE TABLE IF NOT EXISTS telegram_login_tokens (
	token       TEXT PRIMARY KEY,
	telegram_id BIGINT,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at  TIMESTAMPTZ NOT NULL,
	claimed_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_telegram_login_tokens_expire ON telegram_login_tokens (expires_at);

-- Consent records for 152-FZ compliance. Tracks user consent to personal data
-- processing including timestamp, IP, and consent text hash.
CREATE TABLE IF NOT EXISTS consent_records (
	id            BIGSERIAL PRIMARY KEY,
	user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	consent_type  TEXT NOT NULL DEFAULT 'privacy_policy',
	consent_hash  TEXT NOT NULL,
	ip            TEXT,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	revoked_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_consent_records_user ON consent_records(user_id);

-- Data retention tracking: marks users for automated cleanup.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cleanup_after TIMESTAMPTZ;
ALTER TABLE bot_notifications ADD COLUMN IF NOT EXISTS cleanup_after TIMESTAMPTZ;
ALTER TABLE renewal_notifications ADD COLUMN IF NOT EXISTS cleanup_after TIMESTAMPTZ;
`
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func NewRedis(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
