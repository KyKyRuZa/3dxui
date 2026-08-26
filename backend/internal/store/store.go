package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	VALUES ($1, NULLIF($2, ''), $3)
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
SELECT id, username, COALESCE(email, '') AS email, password_hash, is_active, telegram_id, panel_username, panel_uuid, created_at
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
	_, err := s.db.ExecContext(ctx, "UPDATE users SET email = NULLIF($1, '') WHERE id = $2", email, userID)
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

func (s *Store) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	const q = `
INSERT INTO subscriptions (user_id, status, panel_email, panel_sub_id, group_name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at`
	return s.db.QueryRowContext(ctx, q, sub.UserID, sub.Status, sub.PanelEmail, sub.PanelSubID, sub.GroupName, sub.ExpiresAt).
		Scan(&sub.ID, &sub.CreatedAt)
}

func (s *Store) GetUserSubscription(ctx context.Context, userID int64) (*models.Subscription, error) {
	sub := &models.Subscription{}
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at
FROM subscriptions WHERE user_id = $1`, userID).
		Scan(&sub.ID, &sub.UserID, &sub.Status, &sub.PanelEmail, &sub.PanelSubID, &sub.GroupName, &sub.CreatedAt, &sub.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sub, nil
}

func (s *Store) GetSubscriptionByPanelEmail(ctx context.Context, email string) (*models.Subscription, error) {
	sub := &models.Subscription{}
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at
FROM subscriptions WHERE panel_email = $1`, email).
		Scan(&sub.ID, &sub.UserID, &sub.Status, &sub.PanelEmail, &sub.PanelSubID, &sub.GroupName, &sub.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sub, nil
}

func (s *Store) UpdateSubscriptionGroup(ctx context.Context, subID int64, group string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET group_name = $1 WHERE id = $2`, group, subID)
	return err
}

func (s *Store) UpdateSubscriptionSubID(ctx context.Context, subID int64, subIDStr string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET panel_sub_id = $1 WHERE id = $2`, subIDStr, subID)
	return err
}

// ExpiringItem is a subscription that is about to expire, joined with the
// owning user's Telegram id and username (used for bot notifications).
type ExpiringItem struct {
	ID         int64     `db:"id"`
	TelegramID int64     `db:"telegram_id"`
	Username    string    `db:"username"`
	ExpiresAt   time.Time `db:"expires_at"`
}

// GetExpiringSubscriptions returns subscriptions whose expiry falls within
// (now, before], that belong to users with a Telegram id, and that have not
// already been notified today. Results are ordered by soonest expiry first.
func (s *Store) GetExpiringSubscriptions(ctx context.Context, before time.Time) ([]ExpiringItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, u.telegram_id, u.username, s.expires_at
FROM subscriptions s
JOIN users u ON u.id = s.user_id
WHERE s.expires_at IS NOT NULL
  AND s.expires_at <= $1
  AND s.expires_at > NOW()
  AND u.telegram_id IS NOT NULL
  AND (s.last_expiry_notify_date IS NULL OR s.last_expiry_notify_date < CURRENT_DATE)
ORDER BY s.expires_at ASC`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ExpiringItem, 0)
	for rows.Next() {
		var it ExpiringItem
		if err := rows.Scan(&it.ID, &it.TelegramID, &it.Username, &it.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// MarkExpiryNotified records that the given subscriptions were notified today,
// so the daily notification loop does not re-send on the same calendar day.
func (s *Store) MarkExpiryNotified(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE subscriptions SET last_expiry_notify_date = CURRENT_DATE WHERE id = ANY($1)`, ids)
	return err
}

// UpdateSubscriptionExpiry extends (or sets) a subscription's expiry timestamp.
func (s *Store) UpdateSubscriptionExpiry(ctx context.Context, subID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET expires_at = $1 WHERE id = $2`, expiresAt, subID)
	return err
}
