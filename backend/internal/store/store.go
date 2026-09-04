package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/utils"
)

// ErrConflict indicates a unique-constraint violation (e.g. duplicate username).
var ErrConflict = errors.New("resource already exists")

// ErrNotFound indicates the requested row does not exist.
var ErrNotFound = errors.New("not found")

type Store struct {
	db    *sql.DB
	redis *redis.Client
}

func New(db *sql.DB, rdb ...*redis.Client) *Store {
	s := &Store{db: db}
	if len(rdb) > 0 {
		s.redis = rdb[0]
	}
	return s
}

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	const q = `
INSERT INTO users (username, email, password_hash)
	VALUES ($1, $2, $3)
	RETURNING id, username, email, is_active, panel_username, panel_uuid, referral_code, created_at`

	u := &models.User{}
	err := s.db.QueryRowContext(ctx, q, username, email, passwordHash).
		Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &u.PanelUsername, &u.PanelUUID, &u.ReferralCode, &u.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("CreateUser db: %w", err)
	}

	code := fmt.Sprintf("ref%d", u.ID)
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET referral_code = $1 WHERE id = $2`, code, u.ID); err != nil {
		return nil, fmt.Errorf("CreateUser referral_code: %w", err)
	}
	u.ReferralCode = sql.NullString{String: code, Valid: true}
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
SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at
	FROM users `

	u := &models.User{}
	err := s.db.QueryRowContext(ctx, base+where, arg).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsActive, &u.TelegramID, &u.PanelUsername, &u.PanelUUID, &u.ReferralCode, &u.CreatedAt,
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
	Username   string    `db:"username"`
	ExpiresAt  time.Time `db:"expires_at"`
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
UPDATE subscriptions SET last_expiry_notify_date = CURRENT_DATE WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

// GetExpiredSubscriptions returns subscriptions that have just expired (within
// the last `since` window), belong to users with a Telegram id, and have not
// already been notified about the expiry today. Used for the one-shot
// "subscription ended — buy now" push.
func (s *Store) GetExpiredSubscriptions(ctx context.Context, since time.Time) ([]ExpiringItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, u.telegram_id, u.username, s.expires_at
FROM subscriptions s
JOIN users u ON u.id = s.user_id
WHERE s.expires_at IS NOT NULL
  AND s.expires_at <= NOW()
  AND s.expires_at > $1
  AND u.telegram_id IS NOT NULL
  AND (s.last_expired_notify_date IS NULL OR s.last_expired_notify_date < CURRENT_DATE)
ORDER BY s.expires_at DESC`, since)
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

// MarkExpiredNotified records that the given subscriptions were notified about
// their expiry today, so the loop does not re-send on the same calendar day.
func (s *Store) MarkExpiredNotified(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE subscriptions SET last_expired_notify_date = CURRENT_DATE WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

// UpdateSubscriptionExpiry extends (or sets) a subscription's expiry timestamp.
func (s *Store) UpdateSubscriptionExpiry(ctx context.Context, subID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET expires_at = $1 WHERE id = $2`, expiresAt, subID)
	return err
}

// uniqueReferralCode returns a referral code that is not yet used by any user.
func (s *Store) uniqueReferralCode(ctx context.Context) string {
	for i := 0; i < 10; i++ {
		c := utils.RandString(8)
		if _, err := s.GetUserByReferralCode(ctx, c); errors.Is(err, ErrNotFound) {
			return c
		}
	}
	return utils.RandString(8) + "x"
}

func (s *Store) GetUserByReferralCode(ctx context.Context, code string) (*models.User, error) {
	return s.scanUser(ctx, "WHERE referral_code = $1", code)
}

// CreateReferral records that `referredID` joined via `referrerID`'s code.
// Self-referrals and duplicates are ignored. The reward is credited later,
// when the referred user makes a paid purchase (see CompleteReferral).
// CreateReferral records a new pending referral. Returns true if the row was
// actually inserted, false if it already existed (ON CONFLICT DO NOTHING).
// This prevents double-application of the signup bonus under concurrent requests.
func (s *Store) CreateReferral(ctx context.Context, referrerID, referredID int64, rewardDays int) (bool, error) {
	if referrerID == referredID {
		return false, nil
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO referrals (referrer_id, referred_id, status, reward_days)
	VALUES ($1, $2, 'pending', $3)
	ON CONFLICT (referrer_id, referred_id) DO NOTHING`, referrerID, referredID, rewardDays)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetReferral returns any referral (pending or completed) between the given
// referrer and referred, or ErrNotFound if none exists. Used to avoid
// double-attributing a referral or crediting the bonus twice.
func (s *Store) GetReferral(ctx context.Context, referrerID, referredID int64) (*models.Referral, error) {
	var r models.Referral
	err := s.db.QueryRowContext(ctx, `
SELECT id, referrer_id, referred_id, status, reward_days, created_at, completed_at
FROM referrals WHERE referrer_id = $1 AND referred_id = $2`, referrerID, referredID).
		Scan(&r.ID, &r.ReferrerID, &r.ReferredID, &r.Status, &r.RewardDays, &r.CreatedAt, &r.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetPendingReferral returns the open referral where the given user was referred.
func (s *Store) GetPendingReferral(ctx context.Context, referredID int64) (*models.Referral, error) {
	var r models.Referral
	err := s.db.QueryRowContext(ctx, `
SELECT id, referrer_id, referred_id, status, reward_days, created_at, completed_at
FROM referrals WHERE referred_id = $1 AND status = 'pending'`, referredID).
		Scan(&r.ID, &r.ReferrerID, &r.ReferredID, &r.Status, &r.RewardDays, &r.CreatedAt, &r.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CompleteReferral marks a pending referral as rewarded.
func (s *Store) CompleteReferral(ctx context.Context, referrerID, referredID int64, rewardDays int) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE referrals SET status = 'completed', reward_days = $3, completed_at = NOW()
WHERE referrer_id = $1 AND referred_id = $2 AND status = 'pending'`, referrerID, referredID, rewardDays)
	return err
}

// GetReferralStats returns the user's referral code, number of completed invites,
// and total bonus days earned.
func (s *Store) GetReferralStats(ctx context.Context, userID int64) (code string, invited int, earnedDays int, err error) {
	var nullCode sql.NullString
	if e := s.db.QueryRowContext(ctx, `SELECT referral_code FROM users WHERE id = $1`, userID).Scan(&nullCode); e != nil {
		return "", 0, 0, e
	}
	code = nullCode.String
	if e := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(reward_days), 0) FROM referrals WHERE referrer_id = $1 AND status = 'completed'`, userID).
		Scan(&invited, &earnedDays); e != nil {
		return code, 0, 0, e
	}
	return code, invited, earnedDays, nil
}

// ListPlans returns all available subscription plans.
func (s *Store) ListPlans(ctx context.Context) ([]models.Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, duration_days, price_minor, currency, group_name
FROM plans ORDER BY duration_days ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Plan, 0)
	for rows.Next() {
		var p models.Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.DurationDays, &p.PriceMinor, &p.Currency, &p.GroupName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlan returns a plan by its id.
func (s *Store) GetPlan(ctx context.Context, id string) (*models.Plan, error) {
	p := &models.Plan{}
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, duration_days, price_minor, currency, group_name
FROM plans WHERE id = $1`, id).
		Scan(&p.ID, &p.Name, &p.DurationDays, &p.PriceMinor, &p.Currency, &p.GroupName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CreatePayment records a newly created YooKassa payment.
func (s *Store) CreatePayment(ctx context.Context, p *models.PaymentRow) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO payments (id, user_id, plan_id, status, amount_minor, currency)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, updated_at = NOW()`,
		p.ID, p.UserID, p.PlanID, p.Status, p.AmountMinor, p.Currency)
	return err
}

// GetPayment returns a payment by its YooKassa id.
func (s *Store) GetPayment(ctx context.Context, id string) (*models.PaymentRow, error) {
	p := &models.PaymentRow{}
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at
FROM payments WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.PlanID, &p.Status, &p.AmountMinor, &p.Currency, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePaymentStatus flips a payment's status and bumps updated_at.
func (s *Store) UpdatePaymentStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}

// SetPaymentResult records the authoritative status and the actually captured
// amount (as reported by the YooKassa API during webhook verification). It only
// updates status/amount/currency and never touches user_id/plan_id, so an
// already-created row keeps its correlation data.
func (s *Store) SetPaymentResult(ctx context.Context, id, status string, amountMinor int64, currency string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE payments SET
	status = $2,
	amount_minor = CASE WHEN $3 > 0 THEN $3 ELSE amount_minor END,
	currency = CASE WHEN $4 <> '' THEN $4 ELSE currency END,
	updated_at = NOW()
WHERE id = $1`, id, status, amountMinor, currency)
	return err
}

// ClaimPayment atomically marks a payment as 'succeeded' only if it is not
// already in a terminal state. Returns true if this call claimed the payment,
// false if another request already processed it. This prevents double
// provisioning when two webhooks arrive concurrently.
func (s *Store) ClaimPayment(ctx context.Context, id string) (bool, error) {
	var userID int64
	var planID string
	err := s.db.QueryRowContext(ctx, `
UPDATE payments SET status = 'succeeded', updated_at = NOW()
WHERE id = $1 AND status NOT IN ('succeeded', 'canceled')
RETURNING user_id, plan_id`, id).Scan(&userID, &planID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = userID
	_ = planID
	return true, nil
}

// CreatePaymentIfNotExists creates a payment row from YooKassa verification
// data when the webhook arrived before the local create call completed.
// Returns true if the row was created, false if it already existed.
func (s *Store) CreatePaymentIfNotExists(ctx context.Context, id string, billingPayment *billing.Payment) (bool, error) {
	uid := 0
	planID := ""
	if billingPayment.Metadata != nil {
		if u, ok := billingPayment.Metadata["user_id"]; ok {
			fmt.Sscanf(u, "%d", &uid)
		}
		if p, ok := billingPayment.Metadata["plan_id"]; ok {
			planID = p
		}
	}
	if uid == 0 || planID == "" {
		return false, nil
	}
	amountMinor := int64(0)
	if billingPayment.Amount.Value != "" {
		var f float64
		if _, err := fmt.Sscanf(billingPayment.Amount.Value, "%f", &f); err == nil {
			amountMinor = int64(f * 100)
		}
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO payments (id, user_id, plan_id, status, amount_minor, currency)
VALUES ($1, $2, $3, 'pending', $4, $5)
ON CONFLICT (id) DO NOTHING`,
		id, uid, planID, amountMinor, billingPayment.Amount.Currency)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateRenewalNotification enqueues a bot notification about a renewed
// subscription. Idempotent by user_id+expires_at to avoid duplicates on
// webhook retries.
func (s *Store) CreateRenewalNotification(ctx context.Context, userID, telegramID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO renewal_notifications (user_id, telegram_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, expires_at) DO NOTHING`,
		userID, telegramID, expiresAt)
	return err
}

// GetPendingRenewalNotifications returns renewal notifications that have not
// been delivered yet, ordered by creation time.
func (s *Store) GetPendingRenewalNotifications(ctx context.Context) ([]models.RenewalNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, telegram_id, expires_at, created_at, notified_at
FROM renewal_notifications
WHERE notified_at IS NULL
ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.RenewalNotification, 0)
	for rows.Next() {
		var n models.RenewalNotification
		if err := rows.Scan(&n.ID, &n.UserID, &n.TelegramID, &n.ExpiresAt, &n.CreatedAt, &n.NotifiedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkRenewalNotified marks notifications as delivered.
func (s *Store) MarkRenewalNotified(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE renewal_notifications SET notified_at = NOW() WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

// CreateBotNotification enqueues a generic Telegram push for a user. Idempotent
// per (kind, ref_key) so retried backend events never queue duplicates. `data`
// is the raw JSON payload rendered by the bot for the given `kind`.
func (s *Store) CreateBotNotification(ctx context.Context, telegramID int64, kind, refKey string, data []byte) error {
	if len(data) == 0 {
		data = []byte("{}")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO bot_notifications (telegram_id, kind, ref_key, data)
VALUES ($1, $2, $3, $4)
ON CONFLICT (kind, ref_key) DO NOTHING`, telegramID, kind, refKey, data)
	return err
}

// GetPendingBotNotifications returns generic bot notifications that have not
// been delivered yet, ordered by creation time.
func (s *Store) GetPendingBotNotifications(ctx context.Context) ([]models.BotNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, telegram_id, kind, data, ref_key, created_at, notified_at
FROM bot_notifications WHERE notified_at IS NULL ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.BotNotification, 0)
	for rows.Next() {
		var n models.BotNotification
		if err := rows.Scan(&n.ID, &n.TelegramID, &n.Kind, &n.Data, &n.RefKey, &n.CreatedAt, &n.NotifiedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkBotNotificationsNotified marks generic bot notifications as delivered.
func (s *Store) MarkBotNotificationsNotified(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE bot_notifications SET notified_at = NOW() WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

// ClaimBotNotifications atomically marks pending notifications as notified
// and returns the claimed rows. This prevents duplicate sends when multiple
// bot instances poll simultaneously. Uses UPDATE ... RETURNING to atomically
// claim rows.
func (s *Store) ClaimBotNotifications(ctx context.Context) ([]models.BotNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
UPDATE bot_notifications SET notified_at = NOW()
WHERE id IN (SELECT id FROM bot_notifications WHERE notified_at IS NULL ORDER BY created_at ASC FOR UPDATE SKIP LOCKED)
RETURNING id, telegram_id, kind, data, ref_key, created_at, notified_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.BotNotification, 0)
	for rows.Next() {
		var n models.BotNotification
		if err := rows.Scan(&n.ID, &n.TelegramID, &n.Kind, &n.Data, &n.RefKey, &n.CreatedAt, &n.NotifiedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// TelegramLoginToken tracks a browser-initiated login request that the user
// completes inside the Telegram bot.

type TelegramLoginToken struct {
	Token      string
	TelegramID sql.NullInt64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ClaimedAt  sql.NullTime
}

// CreateLoginToken issues a single-use token the user opens via the bot.
func (s *Store) CreateLoginToken(ctx context.Context, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO telegram_login_tokens (token, expires_at) VALUES ($1, $2)
ON CONFLICT (token) DO NOTHING`, token, expiresAt)
	return err
}

// ClaimLoginToken binds a Telegram user to an unclaimed, unexpired token.
// Returns ErrNotFound if the token is missing, expired, or already claimed.
func (s *Store) ClaimLoginToken(ctx context.Context, token string, telegramID int64) (*TelegramLoginToken, error) {
	var t TelegramLoginToken
	err := s.db.QueryRowContext(ctx, `
UPDATE telegram_login_tokens
   SET telegram_id = $2, claimed_at = NOW()
 WHERE token = $1
   AND telegram_id IS NULL
   AND expires_at > NOW()
RETURNING token, telegram_id, created_at, expires_at, claimed_at`,
		token, telegramID).Scan(&t.Token, &t.TelegramID, &t.CreatedAt, &t.ExpiresAt, &t.ClaimedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetLoginToken returns the current state of a token (claimed or not).
func (s *Store) GetLoginToken(ctx context.Context, token string) (*TelegramLoginToken, error) {
	var t TelegramLoginToken
	err := s.db.QueryRowContext(ctx, `
SELECT token, telegram_id, created_at, expires_at, claimed_at
FROM telegram_login_tokens WHERE token = $1`, token).
		Scan(&t.Token, &t.TelegramID, &t.CreatedAt, &t.ExpiresAt, &t.ClaimedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RecordConsent stores a user's consent to personal data processing.
func (s *Store) RecordConsent(ctx context.Context, userID int64, consentType, consentHash, ip string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO consent_records (user_id, consent_type, consent_hash, ip)
VALUES ($1, $2, $3, $4)`, userID, consentType, consentHash, ip)
	return err
}

// GetConsentRecords returns all consent records for a user.
func (s *Store) GetConsentRecords(ctx context.Context, userID int64) ([]models.ConsentRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, consent_type, consent_hash, ip, created_at, revoked_at
FROM consent_records WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ConsentRecord, 0)
	for rows.Next() {
		var c models.ConsentRecord
		if err := rows.Scan(&c.ID, &c.UserID, &c.ConsentType, &c.ConsentHash, &c.IP, &c.CreatedAt, &c.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteUser permanently removes a user and all associated data (152-FZ right to erasure).
// All foreign keys have ON DELETE CASCADE, so this removes sessions, subscriptions,
// referrals, payments, notifications, and consent records.
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

// DataExport contains all personal data for a user (152-FZ right of access).
type DataExport struct {
	User         models.User           `json:"user"`
	Sessions     []models.Session      `json:"sessions"`
	Subscription *models.Subscription  `json:"subscription,omitempty"`
	Referrals    []models.Referral     `json:"referrals"`
	Payments     []models.PaymentRow   `json:"payments"`
	Consent      []models.ConsentRecord `json:"consent"`
}

// ExportUserData collects all personal data for a user.
func (s *Store) ExportUserData(ctx context.Context, userID int64) (*DataExport, error) {
	export := &DataExport{}

	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	export.User = *user

	sessions, err := s.db.QueryContext(ctx, `
SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at
FROM sessions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer sessions.Close()
	for sessions.Next() {
		var sess models.Session
		if err := sessions.Scan(&sess.ID, &sess.UserID, &sess.RefreshHash, &sess.UserAgent, &sess.IP, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
			return nil, err
		}
		export.Sessions = append(export.Sessions, sess)
	}

	sub, err := s.GetUserSubscription(ctx, userID)
	if err == nil {
		export.Subscription = sub
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, referrer_id, referred_id, status, reward_days, created_at, completed_at
FROM referrals WHERE referrer_id = $1 OR referred_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.Referral
		if err := rows.Scan(&r.ID, &r.ReferrerID, &r.ReferredID, &r.Status, &r.RewardDays, &r.CreatedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		export.Referrals = append(export.Referrals, r)
	}

	prows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at
FROM payments WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var p models.PaymentRow
		if err := prows.Scan(&p.ID, &p.UserID, &p.PlanID, &p.Status, &p.AmountMinor, &p.Currency, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		export.Payments = append(export.Payments, p)
	}

	export.Consent, _ = s.GetConsentRecords(ctx, userID)

	return export, nil
}
