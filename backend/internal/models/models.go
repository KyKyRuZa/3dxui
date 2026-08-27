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
	ReferralCode  sql.NullString `json:"referral_code" db:"referral_code"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
}

// Referral records that `referred_id` joined via `referrer_id`'s code.
// Status flips from "pending" to "completed" once the referred user pays,
// at which point the referrer is rewarded with bonus days.
type Referral struct {
	ID          int64        `db:"id"`
	ReferrerID  int64        `db:"referrer_id"`
	ReferredID  int64        `db:"referred_id"`
	Status      string       `db:"status"`
	RewardDays  int          `db:"reward_days"`
	CreatedAt   time.Time    `db:"created_at"`
	CompletedAt sql.NullTime `db:"completed_at"`
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
	ID                   int64          `json:"id" db:"id"`
	UserID               int64          `json:"user_id" db:"user_id"`
	Status               string         `json:"status" db:"status"`
	PanelEmail           string         `json:"panel_email" db:"panel_email"`
	PanelSubID           sql.NullString `json:"panel_sub_id" db:"panel_sub_id"`
	GroupName            string         `json:"group_name" db:"group_name"`
	CreatedAt            time.Time      `json:"created_at" db:"created_at"`
	ExpiresAt            sql.NullTime   `json:"expires_at" db:"expires_at"`
	LastExpiryNotifyDate sql.NullTime   `json:"-" db:"last_expiry_notify_date"`
	LastExpiredNotifyDate sql.NullTime  `json:"-" db:"last_expired_notify_date"`
}

// Plan describes a purchasable VPN subscription tier. Price is stored in the
// smallest currency unit (kopecks for RUB) to avoid floating point rounding.
type Plan struct {
	ID           string `db:"id" json:"id"`
	Name         string `db:"name" json:"name"`
	DurationDays int    `db:"duration_days" json:"duration_days"`
	PriceMinor   int64  `db:"price_minor" json:"price_minor"`
	Currency     string `db:"currency" json:"currency"`
	GroupName    string `db:"group_name" json:"group_name"`
}

// PaymentRow tracks a YooKassa payment and links it to the buying user/plan.
type PaymentRow struct {
	ID          string    `db:"id" json:"id"`
	UserID      int64     `db:"user_id" json:"user_id"`
	PlanID      string    `db:"plan_id" json:"plan_id"`
	Status      string    `db:"status" json:"status"`
	AmountMinor int64     `db:"amount_minor" json:"amount_minor"`
	Currency    string    `db:"currency" json:"currency"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
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
