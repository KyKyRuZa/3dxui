package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

type credentials struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		fmt.Printf("DEBUG register: bind json error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	fmt.Printf("DEBUG register: start username=%s email=%s\n", body.Username, body.Email)
	if err := validateCredentials(body.Username, body.Email, body.Password); err != nil {
		fmt.Printf("DEBUG register: validation error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		fmt.Printf("DEBUG register: hash password error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	fmt.Printf("DEBUG register: password hashed OK\n")

	user, err := h.store.CreateUser(ctx, body.Username, body.Email, hash)
	if errors.Is(err, store.ErrConflict) {
		fmt.Printf("DEBUG register: conflict username/email already exists\n")
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
		return
	}
	if err != nil {
		fmt.Printf("DEBUG register: CreateUser store error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	fmt.Printf("DEBUG register: CreateUser local OK userID=%d\n", user.ID)

	fmt.Printf("DEBUG register: calling issueSession userID=%d\n", user.ID)
	h.issueSession(c, user)
	fmt.Printf("DEBUG register: issueSession returned\n")
}

func (h *Handler) login(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByUsername(ctx, body.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, body.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	h.issueSession(c, user)
}

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

func (h *Handler) updateProfile(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || !strings.Contains(body.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}
	if err := h.store.UpdateEmail(c.Request.Context(), userID, body.Email); errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	user, _ := h.store.GetUserByID(c.Request.Context(), userID)
	c.JSON(http.StatusOK, user.Public())
}

func (h *Handler) changePassword(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(body.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, body.CurrentPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if err := h.store.UpdatePassword(c.Request.Context(), userID, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// issueSession creates a DB session + refresh cookie and returns an access token.
func (h *Handler) issueSession(c *gin.Context, user *models.User) {
	fmt.Printf("DEBUG issueSession: start userID=%d username=%s\n", user.ID, user.Username)
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
		fmt.Printf("DEBUG issueSession: CreateSession error: %v userID=%d\n", err, user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	fmt.Printf("DEBUG issueSession: CreateSession OK\n")
	h.setRefreshCookie(c, raw)

	access, err := h.jwt.NewAccessToken(user.ID, user.Username)
	if err != nil {
		fmt.Printf("DEBUG issueSession: NewAccessToken error: %v userID=%d\n", err, user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	fmt.Printf("DEBUG issueSession: NewAccessToken OK\n")
	c.JSON(http.StatusOK, gin.H{
		"access_token": access,
		"user":         user.Public(),
	})
}

func (h *Handler) telegram(c *gin.Context) {
	var body struct {
		InitData string `json:"init_data"`
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
		_, hash := auth.NewRefreshToken()
		if err != nil {
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
	}

	h.issueSession(c, user)
}

func validateCredentials(username, email, password string) error {
	if len(username) < 3 || len(username) > 32 {
		return errors.New("username must be 3-32 characters")
	}
	if !strings.Contains(email, "@") || len(email) > 254 {
		return errors.New("invalid email")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
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
	c.SetCookie(
		"refresh_token",
		raw,
		int(auth.RefreshTTL().Seconds()),
		"/api/auth",
		"",
		h.cfg.IsProd(),
		true,
	)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetCookie("refresh_token", "", -1, "/api/auth", "", h.cfg.IsProd(), true)
}
