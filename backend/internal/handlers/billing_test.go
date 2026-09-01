package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newMockPanelServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			w.WriteHeader(http.StatusOK)
		case "/panel/api/clients/add":
			resp := map[string]any{"success": true, "obj": map[string]any{"email": "tg_1", "subId": "mocksub123", "inboundIds": []int{1}}}
			json.NewEncoder(w).Encode(resp)
		case "/panel/api/clients/groups/bulkAdd":
			w.WriteHeader(http.StatusOK)
		default:
			if len(r.URL.Path) > len("/panel/api/clients/get/") && r.URL.Path[:len("/panel/api/clients/get/")] == "/panel/api/clients/get/" {
				resp := map[string]any{"success": true, "obj": map[string]any{"email": "tg_1", "subId": "mocksub123", "inboundIds": []int{1}}}
				json.NewEncoder(w).Encode(resp)
				return
			}
			if len(r.URL.Path) > len("/panel/api/clients/update/") && r.URL.Path[:len("/panel/api/clients/update/")] == "/panel/api/clients/update/" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func newBillingTestHandler(t *testing.T) (*Handler, sqlmock.Sqlmock) {
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
		YookassaShopID:          "1268375",
		YookassaSecretKey:       "test_secret",
		YookassaReturnURL:       "https://example.com/return",
		YookassaAPIURL:          "https://api.yookassa.ru/v3",
		BotUsername:             "AutoColorsBot",
	}

	log, _ := zap.NewDevelopment()
	tokenSvc, err := newTestTokenServiceAuth(t)
	require.NoError(t, err)

	billingClient := billing.New(cfg.YookassaShopID, cfg.YookassaSecretKey, cfg.YookassaAPIURL)

	h := NewHandler(st, tokenSvc, cfg, nil, billingClient, log.Sugar())
	return h, mock
}

func newProvisionTestHandler(t *testing.T) (*Handler, sqlmock.Sqlmock, *httptest.Server) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)

	srv := newMockPanelServer(t)

	cfg := &config.Config{
		DefaultGroup:      "Free",
		DefaultInboundIDs: []int{1},
		PanelURL:          srv.URL,
		PanelPublicURL:    srv.URL,
	}
	log, _ := zap.NewDevelopment()
	p := panel.New(srv.URL, "admin", "admin", "", log.Sugar())

	h := NewHandler(st, nil, cfg, p, nil, log.Sugar())
	return h, mock, srv
}

func newTestTokenServiceAuth(t *testing.T) (*auth.TokenService, error) {
	t.Helper()
	log, _ := zap.NewDevelopment()
	return auth.NewTokenService(&config.Config{}, log.Sugar())
}

func TestBillingWebhook_PaymentSucceeded_Idempotent(t *testing.T) {
	h, mock := newBillingTestHandler(t)

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = .*").
		WithArgs("pay_123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plan_id", "status", "amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("pay_123", 1, "standard", "succeeded", int64(29900), "RUB", time.Now(), time.Now()))

	payload := `{"event":"payment.succeeded","object":{"id":"pay_123","status":"succeeded"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBillingWebhook_PaymentCanceled(t *testing.T) {
	h, mock := newBillingTestHandler(t)

	mock.ExpectExec("UPDATE payments SET status = .*, updated_at = NOW.*WHERE id = .*").
		WithArgs("pay_456", "canceled").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = .*").
		WithArgs("pay_456").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plan_id", "status", "amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("pay_456", 1, "standard", "canceled", int64(29900), "RUB", time.Now(), time.Now()))

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_1", nil, "hash", true, int64(699469085), nil, nil, "refcode", time.Now()))

	mock.ExpectExec("INSERT INTO bot_notifications").
		WithArgs(int64(699469085), "payment_failed", "payfail:pay_456", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	payload := `{"event":"payment.canceled","object":{"id":"pay_456","status":"canceled"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBillingWebhook_InvalidPayload(t *testing.T) {
	h, _ := newBillingTestHandler(t)

	payload := `{"event":""}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
}

func TestBillingWebhook_MalformedJSON(t *testing.T) {
	h, _ := newBillingTestHandler(t)

	payload := `not-json`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBillingWebhook_MissingPaymentID(t *testing.T) {
	h, _ := newBillingTestHandler(t)

	payload := `{"event":"payment.succeeded","object":{"id":"","status":"succeeded"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBillingWebhook_IgnoreNonSucceededEvent(t *testing.T) {
	h, _ := newBillingTestHandler(t)

	payload := `{"event":"payment.waiting_for_capture","object":{"id":"pay_789","status":"waiting_for_capture"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
}

func TestBillingWebhook_BillingNotConfigured(t *testing.T) {
	db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)

	cfg := &config.Config{
		DefaultSubscriptionDays: 2,
		DefaultGroup:            "Free",
		DefaultInboundIDs:       []int{1},
	}

	log, _ := zap.NewDevelopment()
	tokenSvc, err := newTestTokenServiceAuth(t)
	require.NoError(t, err)

	h := NewHandler(st, tokenSvc, cfg, nil, nil, log.Sugar())

	payload := `{"event":"payment.succeeded","object":{"id":"pay_123","status":"succeeded"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.billingWebhook(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestResolvePaymentTarget_FromDB(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = .*").
		WithArgs("pay_123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plan_id", "status", "amount_minor", "currency", "created_at", "updated_at"}).
			AddRow("pay_123", 42, "standard", "succeeded", int64(29900), "RUB", now, now))

	cfg := &config.Config{}
	log, _ := zap.NewDevelopment()
	h := NewHandler(st, nil, cfg, nil, nil, log.Sugar())

	verified := &billing.Payment{ID: "pay_123"}
	userID, planID := resolvePaymentTarget(t.Context(), h, verified, "pay_123")

	assert.Equal(t, int64(42), userID)
	assert.Equal(t, "standard", planID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResolvePaymentTarget_FromMetadata(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = .*").
		WithArgs("pay_123").
		WillReturnError(sql.ErrNoRows)

	cfg := &config.Config{}
	log, _ := zap.NewDevelopment()
	h := NewHandler(st, nil, cfg, nil, nil, log.Sugar())

	verified := &billing.Payment{
		ID: "pay_123",
		Metadata: map[string]string{
			"user_id": "42",
			"plan_id": "pro",
		},
	}
	userID, planID := resolvePaymentTarget(t.Context(), h, verified, "pay_123")

	assert.Equal(t, int64(42), userID)
	assert.Equal(t, "pro", planID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResolvePaymentTarget_NoTarget(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	st := store.New(db)

	mock.ExpectQuery("SELECT id, user_id, plan_id, status, amount_minor, currency, created_at, updated_at FROM payments WHERE id = .*").
		WithArgs("pay_123").
		WillReturnError(sql.ErrNoRows)

	cfg := &config.Config{}
	log, _ := zap.NewDevelopment()
	h := NewHandler(st, nil, cfg, nil, nil, log.Sugar())

	verified := &billing.Payment{ID: "pay_123"}
	userID, planID := resolvePaymentTarget(t.Context(), h, verified, "pay_123")

	assert.Equal(t, int64(0), userID)
	assert.Empty(t, planID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProvisionPlan_NewUser(t *testing.T) {
	h, mock, srv := newProvisionTestHandler(t)
	defer srv.Close()
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_1", nil, "hash", true, int64(699469085), nil, nil, "refcode", now))

	mock.ExpectQuery("SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at FROM subscriptions WHERE user_id = .*").
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("INSERT INTO subscriptions").
		WithArgs(int64(1), "active", "tg_1", sqlmock.AnyArg(), "Free", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, now))

	mock.ExpectExec("INSERT INTO renewal_notifications").
		WithArgs(int64(1), int64(699469085), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	plan := &models.Plan{
		ID:           "standard",
		Name:         "Standard",
		DurationDays: 30,
		PriceMinor:   29900,
		Currency:     "RUB",
		GroupName:    "Free",
	}

	err := h.provisionPlan(t.Context(), 1, plan)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProvisionPlan_ExistingUser(t *testing.T) {
	h, mock, srv := newProvisionTestHandler(t)
	defer srv.Close()
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_1", nil, "hash", true, int64(699469085), nil, nil, "refcode", now))

	mock.ExpectQuery("SELECT id, user_id, status, panel_email, panel_sub_id, group_name, created_at, expires_at FROM subscriptions WHERE user_id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "panel_email", "panel_sub_id", "group_name", "created_at", "expires_at"}).
			AddRow(1, 1, "active", "tg_1", "sub123", "Free", now, now.Add(24*time.Hour)))

	mock.ExpectExec("UPDATE subscriptions SET expires_at = .* WHERE id = .*").
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO renewal_notifications").
		WithArgs(int64(1), int64(699469085), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	plan := &models.Plan{
		ID:           "standard",
		Name:         "Standard",
		DurationDays: 30,
		PriceMinor:   29900,
		Currency:     "RUB",
		GroupName:    "Free",
	}

	err := h.provisionPlan(t.Context(), 1, plan)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAmountMinorFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"299.00", 29900},
		{"799.00", 79900},
		{"0.00", 0},
		{"", 0},
		{"invalid", 0},
		{"299", 29900},
		{"299.5", 29950},
		{"299.505", 29950},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := amountMinorFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
