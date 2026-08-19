package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ValidateTelegramInitData verifies the `initData` query string sent by Telegram
// WebApp. It returns the Telegram user ID and username on success.
func ValidateTelegramInitData(initData, botToken string) (userID int64, username string, err error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, "", fmt.Errorf("parse initData: %w", err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return 0, "", fmt.Errorf("missing hash")
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

	userID = parseInt64(values.Get("user.id"))
	username = values.Get("user.username")
	if username == "" {
		username = values.Get("user.first_name")
	}
	return userID, username, nil
}

func hmacSHA256(key, msg []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return hex.EncodeToString(h.Sum(nil))
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
