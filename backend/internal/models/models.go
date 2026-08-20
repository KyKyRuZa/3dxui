package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID            int64          `json:"id" db:"id"`
	Username      string         `json:"username" db:"username"`
	Email         string         `json:"email" db:"email"`
	PasswordHash  string         `json:"-" db:"password_hash"`
	IsActive      bool           `json:"is_active" db:"is_active"`
	TelegramID    sql.NullInt64  `json:"-" db:"telegram_id"`
	PanelUsername sql.NullString `json:"-" db:"panel_username"`
	PanelUUID     sql.NullString `json:"-" db:"panel_uuid"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
}

type Session struct {
	ID          string    `db:"id"`
	UserID      int64     `db:"user_id"`
	RefreshHash string    `db:"refresh_token_hash"`
	UserAgent   string    `db:"user_agent"`
	IP          string    `db:"ip"`
	ExpiresAt   time.Time `db:"expires_at"`
	CreatedAt   time.Time `db:"created_at"`
}

// PublicProfile is the safe subset of a user exposed over the API.
type PublicProfile struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type Subscription struct {
	ID         int64          `json:"id" db:"id"`
	UserID     int64          `json:"user_id" db:"user_id"`
	Status     string         `json:"status" db:"status"`
	PanelEmail string         `json:"panel_email" db:"panel_email"`
	PanelSubID sql.NullString `json:"panel_sub_id" db:"panel_sub_id"`
	GroupName  string         `json:"group_name" db:"group_name"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
}

func (u User) Public() PublicProfile {
	return PublicProfile{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}
