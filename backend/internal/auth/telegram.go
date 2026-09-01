package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ValidateTelegramInitData verifies the `initData` query string sent by Telegram
// WebApp. It returns the Telegram user ID and username on success.
// It also validates auth_date to prevent replay attacks (max age 24 hours).
func ValidateTelegramInitData(initData, botToken string) (userID int64, username string, err error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, "", fmt.Errorf("parse initData: %w", err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return 0, "", fmt.Errorf("missing hash")
	}

	// Replay protection: reject initData that is too old (max 24 hours).
	if authDateStr := values.Get("auth_date"); authDateStr != "" {
		authDate, perr := parseInt64(authDateStr)
		if perr != nil {
			return 0, "", fmt.Errorf("invalid auth_date: %w", perr)
		}
		age := nowSeconds() - authDate
		if age < 0 || age > 86400 {
			return 0, "", fmt.Errorf("stale auth_date")
		}
	}

	dataCheckParts := make([]string, 0, len(values))
	for k, v := range values {
		if k == "hash" {
			continue
		}
		dataCheckParts = append(dataCheckParts, fmt.Sprintf("%s=%s", k, v[0]))
	}
	sort.Strings(dataCheckParts)
	dataCheck := strings.Join(dataCheckParts, "\n")

	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expected := hmacSHA256([]byte(secretKey), []byte(dataCheck))

	if !hmac.Equal([]byte(hash), []byte(expected)) {
		return 0, "", fmt.Errorf("invalid hash")
	}

	userID, perr := parseInt64(values.Get("user.id"))
	if perr != nil {
		return 0, "", fmt.Errorf("invalid user.id: %w", perr)
	}
	username = values.Get("user.username")
	if username == "" {
		username = values.Get("user.first_name")
	}
	return userID, username, nil
}

// TelegramUser is the user payload delivered by the Telegram Login Widget.
type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

// VerifyTelegramWidget checks the `hash` of a Telegram Login Widget payload.
// Unlike WebApp initData (which uses HMAC with the "WebAppData" key), the Login
// Widget uses secret_key = SHA256(botToken) and excludes only `hash` from the
// data-check-string. Returns an error on mismatch or stale auth_date.
func VerifyTelegramWidget(u TelegramUser, botToken string, maxAgeSeconds int64) error {
	if u.Hash == "" {
		return fmt.Errorf("missing hash")
	}

	parts := []string{}
	add := func(k, v string) {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	add("auth_date", fmt.Sprintf("%d", u.AuthDate))
	add("first_name", u.FirstName)
	add("id", fmt.Sprintf("%d", u.ID))
	add("last_name", u.LastName)
	add("photo_url", u.PhotoURL)
	add("username", u.Username)

	dataCheck := strings.Join(parts, "\n")

	secretKey := sha256Sum([]byte(botToken))
	expected := hmacSHA256([]byte(secretKey), []byte(dataCheck))

	if !hmac.Equal([]byte(u.Hash), []byte(expected)) {
		return fmt.Errorf("invalid hash")
	}

	if maxAgeSeconds > 0 {
		age := nowSeconds() - u.AuthDate
		if age < 0 || age > maxAgeSeconds {
			return fmt.Errorf("stale auth_date")
		}
	}
	return nil
}

func sha256Sum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nowSeconds() int64 {
	return timeNow().Unix()
}

// timeNow is a var so it can be overridden in tests.
var timeNow = func() time.Time { return time.Now() }

func hmacSHA256(key, msg []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return hex.EncodeToString(h.Sum(nil))
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, fmt.Errorf("invalid int64 %q: %w", s, err)
	}
	return n, nil
}
