package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	return New(db), mock
}

func TestCreateUser(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO users (username, email, password_hash, referral_code) VALUES ($1, $2, $3, $4) RETURNING id, username, email, is_active, panel_username, panel_uuid, referral_code, created_at").
		WithArgs("testuser", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "is_active", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "testuser", nil, true, nil, nil, "refabc123", time.Now()))

	user, err := store.CreateUser(ctx, "testuser", "", "hash")
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "refabc123", user.ReferralCode.String)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUserConflict(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO users (username, email, password_hash, referral_code) VALUES ($1, $2, $3, $4) RETURNING id, username, email, is_active, panel_username, panel_uuid, referral_code, created_at").
		WithArgs("testuser", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(&pq.Error{Code: "23505"})

	_, err := store.CreateUser(ctx, "testuser", "", "hash")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestGetUserByUsername(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE username = $1").
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "testuser", nil, "hash", true, int64(99), nil, nil, "refcode123", now))

	user, err := store.GetUserByUsername(ctx, "testuser")
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, int64(99), user.TelegramID.Int64)
	assert.Equal(t, "refcode123", user.ReferralCode.String)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE username = $1").
		WithArgs("nouser").
		WillReturnError(sql.ErrNoRows)

	_, err := store.GetUserByUsername(ctx, "nouser")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetUserByTelegramID(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = $1").
		WithArgs(int64(12345)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_12345", nil, "hash", true, int64(12345), nil, nil, nil, now))

	user, err := store.GetUserByTelegramID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "tg_12345", user.Username)
	assert.Equal(t, int64(12345), user.TelegramID.Int64)
	assert.False(t, user.ReferralCode.Valid)
}

func TestSetTelegramID(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE users SET telegram_id = $1 WHERE id = $2").
		WithArgs(12345, 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.SetTelegramID(ctx, 1, 12345)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetTelegramIDConflict(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE users SET telegram_id = $1 WHERE id = $2").
		WithArgs(12345, 1).
		WillReturnError(&pq.Error{Code: "23505"})

	err := store.SetTelegramID(ctx, 1, 12345)
	assert.ErrorIs(t, err, ErrConflict)
}

func TestCreateSubscription(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("INSERT INTO subscriptions (user_id, status, panel_email, panel_sub_id, group_name, expires_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at").
		WithArgs(int64(1), "active", "tg_1", "sub123", "Free", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, now))

	sub := &models.Subscription{
		UserID:     1,
		Status:     "active",
		PanelEmail: "tg_1",
		PanelSubID: sql.NullString{String: "sub123", Valid: true},
		GroupName:  "Free",
		ExpiresAt:  sql.NullTime{Time: now, Valid: true},
	}
	err := store.CreateSubscription(ctx, sub)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sub.ID)
	assert.Equal(t, int64(1), sub.UserID)
}

func TestGetUserSubscription(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at FROM subscriptions WHERE user_id = $1").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "panel_email", "panel_sub_id", "group_name", "created_at", "expires_at"}).
			AddRow(1, 1, "active", "tg_1", "sub123", "Free", now, now))

	sub, err := store.GetUserSubscription(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sub.ID)
	assert.Equal(t, "sub123", sub.PanelSubID.String)
	assert.True(t, sub.ExpiresAt.Valid)
}

func TestGetUserSubscriptionNotFound(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at FROM subscriptions WHERE user_id = $1").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	_, err := store.GetUserSubscription(ctx, 1)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreateReferral(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO referrals (referrer_id, referred_id, status, reward_days) VALUES ($1, $2, 'pending', $3) ON CONFLICT (referrer_id, referred_id) DO NOTHING").
		WithArgs(int64(1), int64(2), 7).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.CreateReferral(ctx, 1, 2, 7)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReferralSelfReferral(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	err := store.CreateReferral(ctx, 1, 1, 7)
	assert.NoError(t, err)
}

func TestGetPendingReferral(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, referrer_id, referred_id, status, reward_days, created_at, completed_at FROM referrals WHERE referred_id = $1 AND status = 'pending'").
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "referrer_id", "referred_id", "status", "reward_days", "created_at", "completed_at"}).
			AddRow(1, 1, 2, "pending", 7, now, nil))

	ref, err := store.GetPendingReferral(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ref.ReferrerID)
	assert.Equal(t, 7, ref.RewardDays)
}

func TestCompleteReferral(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE referrals SET status = 'completed', reward_days = $3, completed_at = NOW() WHERE referrer_id = $1 AND referred_id = $2 AND status = 'pending'").
		WithArgs(int64(1), int64(2), 7).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.CompleteReferral(ctx, 1, 2, 7)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReferralStats(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT referral_code FROM users WHERE id = $1").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"referral_code"}).AddRow("refcode123"))

	mock.ExpectQuery("SELECT COUNT(*), COALESCE(SUM(reward_days), 0) FROM referrals WHERE referrer_id = $1 AND status = 'completed'").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "coalesce"}).AddRow(3, 21))

	code, invited, earned, err := store.GetReferralStats(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "refcode123", code)
	assert.Equal(t, 3, invited)
	assert.Equal(t, 21, earned)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListPlans(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, name, duration_days, price_minor, currency, group_name FROM plans ORDER BY duration_days ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "duration_days", "price_minor", "currency", "group_name"}).
			AddRow("standard", "Standard", 30, int64(29900), "RUB", "Free").
			AddRow("pro", "Pro", 90, int64(79900), "RUB", "Free"))

	plans, err := store.ListPlans(ctx)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	assert.Equal(t, "standard", plans[0].ID)
	assert.Equal(t, 30, plans[0].DurationDays)
	assert.Equal(t, int64(29900), plans[0].PriceMinor)
	assert.Equal(t, "pro", plans[1].ID)
	assert.Equal(t, int64(79900), plans[1].PriceMinor)
}

func TestCreatePayment(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO payments (id, user_id, plan_id, status, amount_minor, currency) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, updated_at = NOW()").
		WithArgs("pay123", int64(1), "standard", "pending", int64(29900), "RUB").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.CreatePayment(ctx, &models.PaymentRow{
		ID: "pay123", UserID: 1, PlanID: "standard", Status: "pending", AmountMinor: 29900, Currency: "RUB",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPayment(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = $1").
		WithArgs("pay123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plan_id", "status", "amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("pay123", 1, "standard", "succeeded", int64(29900), "RUB", now, now))

	payment, err := store.GetPayment(ctx, "pay123")
	require.NoError(t, err)
	assert.Equal(t, "pay123", payment.ID)
	assert.Equal(t, "succeeded", payment.Status)
	assert.Equal(t, int64(29900), payment.AmountMinor)
}

func TestUpdatePaymentStatus(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1").
		WithArgs("pay123", "canceled").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.UpdatePaymentStatus(ctx, "pay123", "canceled")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateRenewalNotification(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("INSERT INTO renewal_notifications (user_id, telegram_id, expires_at) VALUES ($1, $2, $3) ON CONFLICT (user_id, expires_at) DO NOTHING").
		WithArgs(int64(1), int64(99), now).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.CreateRenewalNotification(ctx, 1, 99, now)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPendingRenewalNotifications(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, telegram_id, expires_at, created_at, notified_at FROM renewal_notifications WHERE notified_at IS NULL ORDER BY created_at ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "telegram_id", "expires_at", "created_at", "notified_at"}).
			AddRow(1, 1, 99, now, now, nil))

	items, err := store.GetPendingRenewalNotifications(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), items[0].UserID)
	assert.Equal(t, int64(99), items[0].TelegramID)
	assert.False(t, items[0].NotifiedAt.Valid)
}

func TestMarkRenewalNotified(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE renewal_notifications SET notified_at = NOW() WHERE id = ANY($1)").
		WithArgs(pq.Array([]int64{1, 2})).
		WillReturnResult(sqlmock.NewResult(1, 2))

	err := store.MarkRenewalNotified(ctx, []int64{1, 2})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkRenewalNotifiedEmpty(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	err := store.MarkRenewalNotified(ctx, []int64{})
	assert.NoError(t, err)
}

func TestGetExpiringSubscriptions(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	before := now.Add(48 * time.Hour)

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= $1 AND s.expires_at > NOW() AND u.telegram_id IS NOT NULL AND (s.last_expiry_notify_date IS NULL OR s.last_expiry_notify_date < CURRENT_DATE) ORDER BY s.expires_at ASC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(24*time.Hour)))

	items, err := store.GetExpiringSubscriptions(ctx, before)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(99), items[0].TelegramID)
	assert.Equal(t, "tg_99", items[0].Username)
}

func TestGetExpiredSubscriptions(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= NOW() AND s.expires_at > $1 AND u.telegram_id IS NOT NULL AND (s.last_expired_notify_date IS NULL OR s.last_expired_notify_date < CURRENT_DATE) ORDER BY s.expires_at DESC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(-1*time.Hour)))

	items, err := store.GetExpiredSubscriptions(ctx, since)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(99), items[0].TelegramID)
}

func TestMarkExpiryNotified(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()

	mock.ExpectExec("UPDATE subscriptions SET last_expiry_notify_date = CURRENT_DATE WHERE id = ANY($1)").
		WithArgs(pq.Array([]int64{1, 2})).
		WillReturnResult(sqlmock.NewResult(1, 2))

	err := store.MarkExpiryNotified(ctx, []int64{1, 2})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSubscriptionExpiry(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	newExpiry := time.Now().Add(30 * 24 * time.Hour)

	mock.ExpectExec("UPDATE subscriptions SET expires_at = $1 WHERE id = $2").
		WithArgs(newExpiry, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.UpdateSubscriptionExpiry(ctx, 1, newExpiry)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserByReferralCode(t *testing.T) {
	store, mock := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE referral_code = $1").
		WithArgs("refcode123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "referrer", nil, "hash", true, int64(1), nil, nil, "refcode123", now))

	user, err := store.GetUserByReferralCode(ctx, "refcode123")
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "referrer", user.Username)
}
