package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/ilyas/vpn-service/backend/internal/models"
)

// ErrConflict indicates a unique-constraint violation (e.g. duplicate username).
var ErrConflict = errors.New("resource already exists")

// ErrNotFound indicates the requested row does not exist.
var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	const q = `
INSERT INTO users (username, email, password_hash)
	VALUES ($1, $2, $3)
	RETURNING id, username, email, is_active, panel_username, panel_uuid, created_at`

	u := &models.User{}
	err := s.db.QueryRowContext(ctx, q, username, email, passwordHash).
		Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &u.PanelUsername, &u.PanelUUID, &u.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("CreateUser db: %w", err)
	}
	return u, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.scanUser(ctx, "WHERE username = $1", username)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.scanUser(ctx, "WHERE email = $1", email)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return s.scanUser(ctx, "WHERE id = $1", id)
}

func (s *Store) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	return s.scanUser(ctx, "WHERE telegram_id = $1", telegramID)
}

func (s *Store) scanUser(ctx context.Context, where string, arg any) (*models.User, error) {
	const base = `
SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, created_at
	FROM users `

	u := &models.User{}
	err := s.db.QueryRowContext(ctx, base+where, arg).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsActive, &u.TelegramID, &u.PanelUsername, &u.PanelUUID, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) SetPanelUsername(ctx context.Context, userID int64, panelUsername string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET panel_username = $1 WHERE id = $2", panelUsername, userID)
	if err != nil {
		return fmt.Errorf("SetPanelUsername db: %w", err)
	}
	return nil
}

func (s *Store) SetTelegramID(ctx context.Context, userID int64, telegramID int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET telegram_id = $1 WHERE id = $2", telegramID, userID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("SetTelegramID db: %w", err)
	}
	return nil
}
func (s *Store) SetPanelUUID(ctx context.Context, userID int64, panelUUID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET panel_uuid = $1 WHERE id = $2", panelUUID, userID)
	if err != nil {
		return fmt.Errorf("SetPanelUUID db: %w", err)
	}
	return nil
}

func (s *Store) UpdateEmail(ctx context.Context, userID int64, email string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET email = $1 WHERE id = $2", email, userID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrConflict
		}
	}
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", passwordHash, userID)
	return err
}

func (s *Store) CreateSession(ctx context.Context, sess models.Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)`,
		sess.ID, sess.UserID, sess.RefreshHash, sess.UserAgent, sess.IP, sess.ExpiresAt)
	return err
}

func (s *Store) GetSession(ctx context.Context, id string) (*models.Session, error) {
	var sess models.Session
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at
FROM sessions WHERE id = $1`, id).
		Scan(&sess.ID, &sess.UserID, &sess.RefreshHash, &sess.UserAgent, &sess.IP, &sess.ExpiresAt, &sess.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", id)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}
