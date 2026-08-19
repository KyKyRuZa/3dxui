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
	email            TEXT UNIQUE NOT NULL,
	password_hash    TEXT NOT NULL,
	is_active        BOOLEAN NOT NULL DEFAULT TRUE,
	telegram_id      BIGINT UNIQUE,
	panel_username   TEXT UNIQUE,
	panel_uuid       TEXT UNIQUE,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
