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
