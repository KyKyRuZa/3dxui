package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/store"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *store.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)

	cfg := &config.Config{
		DefaultSubscriptionDays: 2,
		ExpiryNotifyDays:        2,
		ReferralRewardDays:      7,
		ReferralSignupBonusDays: 2,
		DefaultGroup:            "Free",
		DefaultInboundIDs:       []int{1},
		BotAPISecret:            "bot-secret",
		BotToken:                "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		BotUsername:             "AutoColorsBot",
		PanelURL:                "https://panel.example.com",
		PanelPublicURL:          "https://panel.example.com",
	}

	log, _ := zap.NewDevelopment()
	tokenSvc, err := auth.NewTokenService(cfg, log.Sugar())
	require.NoError(t, err)
	h := NewHandler(st, tokenSvc, cfg, nil, nil, nil, log.Sugar())
	return h, st, mock
}

func ginContext(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestBotNotifications(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	// ClaimBotNotifications uses UPDATE ... RETURNING for atomic claim
	mock.ExpectQuery("UPDATE bot_notifications SET notified_at = NOW").
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "kind", "data", "ref_key", "created_at", "notified_at"}).
			AddRow(1, 699469085, "referral_signup", []byte(`{"friend_name":"Test"}`), "signup:1:2", now, now).
			AddRow(2, 699469085, "payment_failed", []byte(`{}`), "payfail:pay_123", now, now))

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/notifications/pending", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botNotifications(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "referral_signup")
	assert.Contains(t, w.Body.String(), "payment_failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotNotifications_Empty(t *testing.T) {
	h, _, mock := newTestHandler(t)

	// ClaimBotNotifications uses UPDATE ... RETURNING for atomic claim
	mock.ExpectQuery("UPDATE bot_notifications SET notified_at = NOW").
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "kind", "data", "ref_key", "created_at", "notified_at"}))

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/notifications/pending", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botNotifications(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"notifications":[]`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotClaimLoginToken_Success(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("UPDATE telegram_login_tokens.*").
		WithArgs("testtoken", int64(699469085)).
		WillReturnRows(sqlmock.NewRows([]string{"token", "telegram_id", "created_at", "expires_at", "claimed_at"}).
			AddRow("testtoken", int64(699469085), now, now.Add(time.Minute), now))

	req, _ := http.NewRequest(http.MethodPost, "/api/bot/login-token/claim", strings.NewReader(`{"token":"testtoken","telegram_id":699469085}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botClaimLoginToken(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotClaimLoginToken_InvalidToken(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectQuery("UPDATE telegram_login_tokens.*").
		WithArgs("badtoken", int64(699469085)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodPost, "/api/bot/login-token/claim", strings.NewReader(`{"token":"badtoken","telegram_id":699469085}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botClaimLoginToken(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotClaimLoginToken_MissingFields(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/bot/login-token/claim", strings.NewReader(`{"token":"","telegram_id":0}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botClaimLoginToken(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebReferral(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectQuery("SELECT referral_code FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"referral_code"}).AddRow("refcode123"))

	mock.ExpectQuery("SELECT COUNT.*, COALESCE.*SUM.*reward_days.*, 0.* FROM referrals WHERE referrer_id = .* AND status = 'completed'").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "coalesce"}).AddRow(3, 21))

	req, _ := http.NewRequest(http.MethodGet, "/api/referral", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	c, w := ginContext(t, req)
	c.Set("user_id", "1")

	h.webReferral(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "refcode123")
	assert.Contains(t, w.Body.String(), "AutoColorsBot")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWebReferral_Unauthorized(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/referral", nil)
	c, w := ginContext(t, req)

	h.webReferral(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBotGetUser_NotFound(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = .*").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/user/999", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)
	c.Params = gin.Params{{Key: "telegram_id", Value: "999"}}

	h.botGetUser(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotGetUser_NoSubscription(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = .*").
		WithArgs(int64(699469085)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "is_admin", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_699469085", nil, "hash", true, false, int64(699469085), nil, nil, "refcode", now))

	mock.ExpectQuery("SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at FROM subscriptions WHERE user_id = .*").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/user/699469085", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)
	c.Params = gin.Params{{Key: "telegram_id", Value: "699469085"}}

	h.botGetUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"provisioned":false`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExpiresAtMillis(t *testing.T) {
	valid := sql.NullTime{Time: time.Unix(1700000000, 0), Valid: true}
	assert.Equal(t, int64(1700000000000), expiresAtMillis(valid))

	invalid := sql.NullTime{}
	assert.Equal(t, int64(0), expiresAtMillis(invalid))
}

func TestBotReferral(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = .*").
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "is_admin", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_99", nil, "hash", true, false, int64(99), nil, nil, "refcode123", now))

	mock.ExpectQuery("SELECT referral_code FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"referral_code"}).AddRow("refcode123"))

	mock.ExpectQuery("SELECT COUNT.*, COALESCE.*SUM.*reward_days.*, 0.* FROM referrals WHERE referrer_id = .* AND status = 'completed'").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "coalesce"}).AddRow(0, 0))

	req, _ := http.NewRequest(http.MethodPost, "/api/bot/referral", strings.NewReader(`{"telegram_id":99}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botReferral(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "refcode123")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotReferralNotFound(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = .*").
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodPost, "/api/bot/referral", strings.NewReader(`{"telegram_id":99}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botReferral(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotExpiring(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= .* AND s.expires_at > NOW.*AND u.telegram_id IS NOT NULL AND .*s.last_expiry_notify_date IS NULL OR s.last_expiry_notify_date < CURRENT_DATE.* ORDER BY s.expires_at ASC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(24*time.Hour)))

	mock.ExpectExec("UPDATE subscriptions SET last_expiry_notify_date = CURRENT_DATE WHERE id = ANY.*").
		WithArgs(pq.Array([]int64{1})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/notifications/expiring", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botExpiring(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tg_99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotExpired(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= NOW.*AND s.expires_at > .* AND u.telegram_id IS NOT NULL AND .*s.last_expired_notify_date IS NULL OR s.last_expired_notify_date < CURRENT_DATE.* ORDER BY s.expires_at DESC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(-1*time.Hour)))

	mock.ExpectExec("UPDATE subscriptions SET last_expired_notify_date = CURRENT_DATE WHERE id = ANY.*").
		WithArgs(pq.Array([]int64{1})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/notifications/expired", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botExpired(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tg_99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBotRenewed(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, telegram_id, expires_at, created_at, notified_at FROM renewal_notifications WHERE notified_at IS NULL ORDER BY created_at ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "telegram_id", "expires_at", "created_at", "notified_at"}).
			AddRow(1, 1, 99, now, now, nil))

	mock.ExpectExec("UPDATE renewal_notifications SET notified_at = NOW.*WHERE id = ANY.*").
		WithArgs(pq.Array([]int64{1})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodGet, "/api/bot/notifications/renewed", nil)
	req.Header.Set("X-Bot-Secret", "bot-secret")
	c, w := ginContext(t, req)

	h.botRenewed(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "99")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealth(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	c, w := ginContext(t, req)

	h.health(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"status":"ok"}`, w.Body.String())
}

func TestPublicConfig(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/config", nil)
	c, w := ginContext(t, req)

	h.publicConfig(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "yookassa_test_mode")
}
