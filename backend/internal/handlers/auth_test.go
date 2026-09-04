package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestTelegram_ValidInitData(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE username = .*").
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "is_admin", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "testuser", nil, "hash", true, false, int64(699469085), nil, nil, "refcode", now))

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(sqlmock.AnyArg(), int64(1), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	initData := buildValidInitData(t, 699469085, "testuser")
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/telegram", strings.NewReader(`{"init_data":"`+initData+`"}`))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.telegram(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegram_NewUser(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE username = .*").
		WithArgs("testuser").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("testuser", "", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "is_active", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "testuser", nil, true, nil, nil, nil, now))

	mock.ExpectExec("UPDATE users SET referral_code").
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE users SET telegram_id").
		WithArgs(int64(699469085), int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(sqlmock.AnyArg(), int64(1), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	initData := buildValidInitData(t, 699469085, "testuser")
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/telegram", strings.NewReader(`{"init_data":"`+initData+`"}`))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.telegram(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegram_InvalidInitData(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/telegram", strings.NewReader(`{"init_data":"invalid&hash=xyz"}`))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.telegram(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTelegram_MissingInitData(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/telegram", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c, w := ginContext(t, req)

	h.telegram(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTelegramLink(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectExec("INSERT INTO telegram_login_tokens").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/telegram/link", nil)
	c, w := ginContext(t, req)

	h.telegramLink(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
	assert.Contains(t, w.Body.String(), "login_url")
	assert.Contains(t, w.Body.String(), "t.me/")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramLinkCheck_Pending(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT token, telegram_id, created_at, expires_at, claimed_at FROM telegram_login_tokens WHERE token = .*").
		WithArgs("testtoken").
		WillReturnRows(sqlmock.NewRows([]string{"token", "telegram_id", "created_at", "expires_at", "claimed_at"}).
			AddRow("testtoken", nil, now, now.Add(time.Minute), nil))

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/telegram/link/testtoken", nil)
	c, w := ginContext(t, req)
	c.Params = gin.Params{{Key: "token", Value: "testtoken"}}

	h.telegramLinkCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pending")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramLinkCheck_Claimed(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	mock.ExpectQuery("SELECT token, telegram_id, created_at, expires_at, claimed_at FROM telegram_login_tokens WHERE token = .*").
		WithArgs("testtoken").
		WillReturnRows(sqlmock.NewRows([]string{"token", "telegram_id", "created_at", "expires_at", "claimed_at"}).
			AddRow("testtoken", int64(699469085), now, now.Add(time.Minute), now))

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE username = .*").
		WithArgs("tg_699469085").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "is_admin", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_699469085", nil, "hash", true, false, int64(699469085), nil, nil, "refcode", now))

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(sqlmock.AnyArg(), int64(1), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/telegram/link/testtoken", nil)
	c, w := ginContext(t, req)
	c.Params = gin.Params{{Key: "token", Value: "testtoken"}}

	h.telegramLinkCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramLinkCheck_InvalidToken(t *testing.T) {
	h, _, mock := newTestHandler(t)

	mock.ExpectQuery("SELECT token, telegram_id, created_at, expires_at, claimed_at FROM telegram_login_tokens WHERE token = .*").
		WithArgs("badtoken").
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/telegram/link/badtoken", nil)
	c, w := ginContext(t, req)
	c.Params = gin.Params{{Key: "token", Value: "badtoken"}}

	h.telegramLinkCheck(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefresh_ValidSession(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	raw, hash := auth.NewRefreshToken()

	mock.ExpectQuery("SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at FROM sessions WHERE id = .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "refresh_token_hash", "user_agent", "ip", "expires_at", "created_at"}).
			AddRow("sessid", 1, hash, "test-agent", "127.0.0.1", now.Add(time.Hour), now))

	mock.ExpectQuery("SELECT id, username, email, password_hash, is_active, is_admin, telegram_id, panel_username, panel_uuid, referral_code, created_at FROM users WHERE id = .*").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_active", "is_admin", "telegram_id", "panel_username", "panel_uuid", "referral_code", "created_at"}).
			AddRow(1, "tg_1", nil, "hash", true, false, int64(1), nil, nil, "refcode", now))

	mock.ExpectExec("DELETE FROM sessions WHERE id = .*").
		WithArgs("sessid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(sqlmock.AnyArg(), int64(1), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
	c, w := ginContext(t, req)

	h.refresh(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefresh_MissingCookie(t *testing.T) {
	h, _, _ := newTestHandler(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	c, w := ginContext(t, req)

	h.refresh(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefresh_InvalidSession(t *testing.T) {
	h, _, mock := newTestHandler(t)

	raw, _ := auth.NewRefreshToken()

	mock.ExpectQuery("SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at FROM sessions WHERE id = .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
	c, w := ginContext(t, req)

	h.refresh(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefresh_ExpiredSession(t *testing.T) {
	h, _, mock := newTestHandler(t)
	now := time.Now()

	raw, hash := auth.NewRefreshToken()

	mock.ExpectQuery("SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at FROM sessions WHERE id = .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "refresh_token_hash", "user_agent", "ip", "expires_at", "created_at"}).
			AddRow("sessid", 1, hash, "test-agent", "127.0.0.1", now.Add(-time.Hour), now))

	mock.ExpectExec("DELETE FROM sessions WHERE id = .*").
		WithArgs("sessid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
	c, w := ginContext(t, req)

	h.refresh(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogout(t *testing.T) {
	h, _, mock := newTestHandler(t)

	raw, _ := auth.NewRefreshToken()

	mock.ExpectExec("DELETE FROM sessions WHERE id = .*").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
	c, w := ginContext(t, req)

	h.logout(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func buildValidInitData(t *testing.T, userID int64, username string) string {
	t.Helper()
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

	values := url.Values{}
	values.Set("user.id", fmt.Sprintf("%d", userID))
	values.Set("user.first_name", "Test")
	values.Set("user.username", username)

	parts := make([]string, 0, len(values))
	for k, v := range values {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v[0]))
	}
	sort.Strings(parts)
	dataCheck := strings.Join(parts, "\n")

	secretKey := computeHMACSHA256([]byte("WebAppData"), []byte(botToken))
	hash := computeHMACSHA256([]byte(secretKey), []byte(dataCheck))

	values.Set("hash", hash)
	return values.Encode()
}

func computeHMACSHA256(key, msg []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return fmt.Sprintf("%x", h.Sum(nil))
}
