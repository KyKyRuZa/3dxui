package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/store"
	"github.com/ilyas/vpn-service/backend/internal/utils"
)

func (h *Handler) refresh(c *gin.Context) {
	raw, err := c.Cookie("refresh_token")
	if err != nil || raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}
	ctx := c.Request.Context()
	sess, err := h.store.GetSession(ctx, auth.HashRefreshToken(raw))
	if errors.Is(err, store.ErrNotFound) {
		h.clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = h.store.DeleteSession(ctx, sess.ID)
		h.clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}

	user, err := h.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}

	// Rotate the refresh token.
	_ = h.store.DeleteSession(ctx, sess.ID)
	h.issueSession(c, user)
}

func (h *Handler) logout(c *gin.Context) {
	raw, err := c.Cookie("refresh_token")
	if err == nil && raw != "" {
		_ = h.store.DeleteSession(c.Request.Context(), auth.HashRefreshToken(raw))
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) profile(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user.Public())
}

// issueSession creates a DB session + refresh cookie and returns an access token.
func (h *Handler) issueSession(c *gin.Context, user *models.User) {
	h.log.Debugw("issueSession: start", "userID", maskInt(user.ID), "username", maskStr(user.Username))
	raw, hash := auth.NewRefreshToken()
	sess := models.Session{
		ID:          auth.HashRefreshToken(raw),
		UserID:      user.ID,
		RefreshHash: hash,
		UserAgent:   c.Request.UserAgent(),
		IP:          c.ClientIP(),
		ExpiresAt:   time.Now().Add(auth.RefreshTTL()),
	}
	if err := h.store.CreateSession(c.Request.Context(), sess); err != nil {
		h.log.Errorw("issueSession: CreateSession error", "error", err, "userID", maskInt(user.ID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.log.Debug("issueSession: CreateSession ok")
	h.setRefreshCookie(c, raw)

	access, err := h.jwt.NewAccessToken(user.ID, user.Username)
	if err != nil {
		h.log.Errorw("issueSession: NewAccessToken error", "error", err, "userID", maskInt(user.ID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.log.Debug("issueSession: NewAccessToken ok")
	c.JSON(http.StatusOK, gin.H{
		"access_token": access,
		"user":         user.Public(),
	})
}

// telegram registers/logs in a user via Telegram. The site has no email/password
// flow: identity is established exclusively from the signed Telegram WebApp
// initData. A new user is auto-created (username tg_<id>) on first sign-in.
func (h *Handler) register(c *gin.Context) {
	var body struct {
		Username    string `json:"username" binding:"required,min=3,max=32"`
		Password    string `json:"password" binding:"required,min=6,max=128"`
		ConsentHash string `json:"consent_hash"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: username (3-32 chars) and password (6+ chars) required"})
		return
	}

	ctx := c.Request.Context()

	// Check if username already exists
	if _, err := h.store.GetUserByUsername(ctx, body.Username); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Hash password
	pwHash, err := auth.HashPassword(body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Create user with password
	user, err := h.store.CreateUser(ctx, body.Username, "", pwHash)
	if err != nil {
		h.log.Errorw("register: create user error", "username", body.Username, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Record consent if provided
	if body.ConsentHash != "" {
		ip := c.ClientIP()
		_ = h.store.RecordConsent(ctx, user.ID, "privacy_policy", body.ConsentHash, ip)
	}

	h.issueSession(c, user)
}

// login authenticates a user by username and password.
func (h *Handler) login(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	user, err := h.store.GetUserByUsername(ctx, body.Username)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Verify password
	if !auth.CheckPassword(user.PasswordHash, body.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.issueSession(c, user)
}

// telegram registers/logs in a user via Telegram. Telegram-provisioned users
// may not have a usable password (placeholder), so they must use Telegram login.
func (h *Handler) telegram(c *gin.Context) {
	var body struct {
		InitData    string `json:"init_data"`
		ConsentHash string `json:"consent_hash"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.InitData == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	userID, username, err := auth.ValidateTelegramInitData(body.InitData, h.cfg.BotToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram data"})
		return
	}
	if username == "" {
		username = fmt.Sprintf("tg_%d", userID)
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		// No email/password account exists; create a Telegram-only identity.
		// password_hash is kept NOT NULL in the schema, so store a random
		// placeholder — it is never used for authentication.
		_, hash := auth.NewRefreshToken()
		if hash == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		user, err = h.store.CreateUser(ctx, username, "", hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if err := h.store.SetTelegramID(ctx, user.ID, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		// Record consent for new users (152-FZ).
		if body.ConsentHash != "" {
			ip := c.ClientIP()
			_ = h.store.RecordConsent(ctx, user.ID, "privacy_policy", body.ConsentHash, ip)
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Keep the Telegram id in sync (e.g. if the account was created elsewhere).
	if !user.TelegramID.Valid || user.TelegramID.Int64 != userID {
		_ = h.store.SetTelegramID(ctx, user.ID, userID)
	}

	h.issueSession(c, user)
}

// telegramWidget registers/logs in a user via the Telegram Login Widget. The
// widget delivers user fields (id, name, ...) plus a HMAC `hash`; this endpoint
// verifies the hash against the bot token before creating/linking the account.
// This is the only path available to users signing in from a regular browser
// (i.e. outside a Telegram WebApp).
func (h *Handler) telegramWidget(c *gin.Context) {
	var u auth.TelegramUser
	if err := c.ShouldBindJSON(&u); err != nil || u.ID == 0 || u.Hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := auth.VerifyTelegramWidget(u, h.cfg.BotToken, 86400); err != nil {
		h.log.Debugw("telegramWidget: verify failed", "telegramID", u.ID, "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram data"})
		return
	}

	username := u.Username
	if username == "" {
		username = fmt.Sprintf("tg_%d", u.ID)
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		_, hash := auth.NewRefreshToken()
		if hash == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		user, err = h.store.CreateUser(ctx, username, "", hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if err := h.store.SetTelegramID(ctx, user.ID, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !user.TelegramID.Valid || user.TelegramID.Int64 != u.ID {
		_ = h.store.SetTelegramID(ctx, user.ID, u.ID)
	}

	h.issueSession(c, user)
}

// telegramLink creates a single-use login token and returns a deep link the user
// opens in the Telegram bot. The bot claims the token via the bot API; the
// website then polls telegramLinkCheck to complete the login.
func (h *Handler) telegramLink(c *gin.Context) {
	token := utils.RandString(32)
	expires := time.Now().Add(5 * time.Minute)
	if err := h.store.CreateLoginToken(c.Request.Context(), token, expires); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"login_url": fmt.Sprintf("https://t.me/%s?start=%s", h.cfg.BotUsername, token),
		"expires_in": 300,
	})
}

// telegramLinkCheck returns the claim status of a login token. Once the bot has
// claimed it, this endpoint creates/logs in the user and issues a session.
func (h *Handler) telegramLinkCheck(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx := c.Request.Context()
	t, err := h.store.GetLoginToken(ctx, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid token"})
		return
	}
	if !t.TelegramID.Valid {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	}

	username := fmt.Sprintf("tg_%d", t.TelegramID.Int64)
	user, err := h.store.GetUserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		_, hash := auth.NewRefreshToken()
		if hash == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		user, err = h.store.CreateUser(ctx, username, "", hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if err := h.store.SetTelegramID(ctx, user.ID, t.TelegramID.Int64); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !user.TelegramID.Valid || user.TelegramID.Int64 != t.TelegramID.Int64 {
		_ = h.store.SetTelegramID(ctx, user.ID, t.TelegramID.Int64)
	}

	h.issueSession(c, user)
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (h *Handler) setRefreshCookie(c *gin.Context, raw string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    raw,
		MaxAge:   int(auth.RefreshTTL().Seconds()),
		Path:     "/api/auth",
		Secure:   h.cfg.IsProd(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(c.Writer, cookie)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		Path:     "/api/auth",
		Secure:   h.cfg.IsProd(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(c.Writer, cookie)
}

// dataExport returns all personal data for the authenticated user (152-FZ right of access).
func (h *Handler) dataExport(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx := c.Request.Context()
	data, err := h.store.ExportUserData(ctx, userID)
	if err != nil {
		h.log.Errorw("dataExport: error", "userID", maskInt(userID), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// deleteUser permanently removes the authenticated user and all associated data (152-FZ right to erasure).
func (h *Handler) deleteUser(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx := c.Request.Context()
	if err := h.store.DeleteUser(ctx, userID); err != nil {
		h.log.Errorw("deleteUser: error", "userID", maskInt(userID), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// recordConsent records the user's consent to personal data processing (152-FZ).
func (h *Handler) recordConsent(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var body struct {
		ConsentType string `json:"consent_type"`
		ConsentHash string `json:"consent_hash"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.ConsentType == "" {
		body.ConsentType = "privacy_policy"
	}
	if body.ConsentHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "consent_hash required"})
		return
	}
	ctx := c.Request.Context()
	ip := c.ClientIP()
	if err := h.store.RecordConsent(ctx, userID, body.ConsentType, body.ConsentHash, ip); err != nil {
		h.log.Errorw("recordConsent: error", "userID", maskInt(userID), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// codeTTL is how long a verification code remains valid.
const codeTTL = 5 * time.Minute

// generateBindCode creates a code for the current user to link their Telegram.
func (h *Handler) generateBindCode(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()

	// Check if user already has Telegram linked
	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if user.TelegramID.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "telegram already linked"})
		return
	}

	// Generate 8-digit code
	code := utils.RandDigits(8)
	vc := store.VerificationCode{
		Code:    code,
		Purpose: "bind",
		UserID:  &userID,
	}

	if err := h.store.SetVerificationCode(ctx, code, vc, codeTTL); err != nil {
		h.log.Errorw("generateBindCode: redis error", "userID", maskInt(userID), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	botUsername := h.cfg.BotUsername
	c.JSON(http.StatusOK, gin.H{
		"code":       code,
		"bot_link":   fmt.Sprintf("https://t.me/%s?start=bind-%s", botUsername, code),
		"expires_in": int(codeTTL.Seconds()),
	})
}

// verifyBindCode checks the code and links Telegram to the current user.
func (h *Handler) verifyBindCode(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var body struct {
		Code       string `json:"code" binding:"required"`
		TelegramID int64  `json:"telegram_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	vc, err := h.store.GetVerificationCode(ctx, body.Code)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Validate code purpose and ownership
	if vc.Purpose != "bind" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code purpose"})
		return
	}
	if vc.UserID == nil || *vc.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "code belongs to another user"})
		return
	}

	// Link Telegram to user
	if err := h.store.SetTelegramID(ctx, userID, body.TelegramID); err != nil {
		if errors.Is(err, store.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "telegram already linked to another account"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Delete used code
	_ = h.store.DeleteVerificationCode(ctx, body.Code)

	h.log.Info("Telegram linked to user", "userID", maskInt(userID), "telegramID", body.TelegramID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// generateLoginCode creates a code for a Telegram user to log in to the site.
func (h *Handler) generateLoginCode(c *gin.Context) {
	var body struct {
		TelegramID int64 `json:"telegram_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	// Check if user exists in DB (created via /start in bot)
	user, err := h.store.GetUserByTelegramID(ctx, body.TelegramID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found. Please start the bot first"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Generate 8-digit code
	code := utils.RandDigits(8)
	vc := store.VerificationCode{
		Code:       code,
		TelegramID: body.TelegramID,
		Purpose:    "login",
		UserID:     &user.ID,
	}

	if err := h.store.SetVerificationCode(ctx, code, vc, codeTTL); err != nil {
		h.log.Errorw("generateLoginCode: redis error", "telegramID", body.TelegramID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": code, "expires_in": int(codeTTL.Seconds())})
}

// verifyLoginCode checks the code and creates a session for the user.
func (h *Handler) verifyLoginCode(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	vc, err := h.store.GetVerificationCode(ctx, body.Code)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if vc.Purpose != "login" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code purpose"})
		return
	}

	// Get user by telegram_id
	user, err := h.store.GetUserByTelegramID(ctx, vc.TelegramID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Delete used code
	_ = h.store.DeleteVerificationCode(ctx, body.Code)

	// Create session
	h.issueSession(c, user)
}
