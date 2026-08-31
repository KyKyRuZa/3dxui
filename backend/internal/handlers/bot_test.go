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
		PanelURL:                "https://panel.example.com",
		PanelPublicURL:          "https://panel.example.com",
	}

	log, _ := zap.NewDevelopment()
	tokenSvc, err := auth.NewTokenService(cfg, log.Sugar())
	require.NoError(t, err)
	h := NewHandler(st, tokenSvc, cfg, nil, nil, log.Sugar())
	return h, st, mock
}

func ginContext(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestBotReferral(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = \\$1").
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_99", nil, "hash", true, int64(99), nil, nil, "refcode123", now))

	mock.ExpectQuery("SELECT referral_code FROM users WHERE id = \\$1").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"referral_code"}).AddRow("refcode123"))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\), COALESCE\\(SUM\\(reward_days\\), 0\\) FROM referrals WHERE referrer_id = \\$1 AND status = 'completed'").
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

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE telegram_id = \\$1").
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

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= \\$1 AND s.expires_at > NOW\\(\\) AND u.telegram_id IS NOT NULL AND \\(s.last_expiry_notify_date IS NULL OR s.last_expiry_notify_date < CURRENT_DATE\\) ORDER BY s.expires_at ASC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(24*time.Hour)))

	mock.ExpectExec("UPDATE subscriptions SET last_expiry_notify_date = CURRENT_DATE WHERE id = ANY\\(\\$1\\)").
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

	mock.ExpectQuery("SELECT s.id, u.telegram_id, u.username, s.expires_at FROM subscriptions s JOIN users u ON u.id = s.user_id WHERE s.expires_at IS NOT NULL AND s.expires_at <= NOW\\(\\) AND s.expires_at > \\$1 AND u.telegram_id IS NOT NULL AND \\(s.last_expired_notify_date IS NULL OR s.last_expired_notify_date < CURRENT_DATE\\) ORDER BY s.expires_at DESC").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "telegram_id", "username", "expires_at"}).
			AddRow(1, 99, "tg_99", now.Add(-1*time.Hour)))

	mock.ExpectExec("UPDATE subscriptions SET last_expired_notify_date = CURRENT_DATE WHERE id = ANY\\(\\$1\\)").
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

	mock.ExpectExec("UPDATE renewal_notifications SET notified_at = NOW\\(\\) WHERE id = ANY\\(\\$1\\)").
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
