package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func computeInitDataHash(initData, botToken string) string {
	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	return hmacSHA256([]byte(secretKey), []byte(initData))
}

func buildInitData(t *testing.T, values url.Values, botToken string) string {
	t.Helper()

	parts := make([]string, 0, len(values))
	for k, v := range values {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v[0]))
	}
	sort.Strings(parts)
	dataCheck := strings.Join(parts, "\n")

	hash := computeInitDataHash(dataCheck, botToken)
	values.Set("hash", hash)
	return values.Encode()
}

func TestValidateTelegramInitData_Valid(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	userID := int64(699469085)
	username := "testuser"

	values := url.Values{}
	values.Set("user.id", fmt.Sprintf("%d", userID))
	values.Set("user.first_name", "Test")
	values.Set("user.username", username)
	initData := buildInitData(t, values, botToken)

	gotID, gotUsername, err := ValidateTelegramInitData(initData, botToken)
	require.NoError(t, err)
	assert.Equal(t, userID, gotID)
	assert.Equal(t, username, gotUsername)
}

func TestValidateTelegramInitData_FallbackToFirstName(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	userID := int64(699469085)

	values := url.Values{}
	values.Set("user.id", fmt.Sprintf("%d", userID))
	values.Set("user.first_name", "Test")
	initData := buildInitData(t, values, botToken)

	gotID, gotUsername, err := ValidateTelegramInitData(initData, botToken)
	require.NoError(t, err)
	assert.Equal(t, userID, gotID)
	assert.Equal(t, "Test", gotUsername)
}

func TestValidateTelegramInitData_InvalidHash(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	initData := `user={"id":699469085}&hash=deadbeef`

	_, _, err := ValidateTelegramInitData(initData, botToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hash")
}

func TestValidateTelegramInitData_MissingHash(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	initData := `user={"id":699469085}`

	_, _, err := ValidateTelegramInitData(initData, botToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing hash")
}

func TestValidateTelegramInitData_MalformedQuery(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

	values := url.Values{}
	values.Set("user", `{"id":699469085}`)
	initData := buildInitData(t, values, botToken)
	_, _, err := ValidateTelegramInitData(initData+"%zz", botToken)
	assert.Error(t, err)
}

func TestValidateTelegramInitData_WrongBotToken(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	wrongToken := "999999:WRONG"

	values := url.Values{}
	values.Set("user", `{"id":699469085}`)
	initData := buildInitData(t, values, botToken)

	_, _, err := ValidateTelegramInitData(initData, wrongToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hash")
}

func computeWidgetHash(parts []string, botToken string) string {
	secretKey := sha256Sum([]byte(botToken))
	dataCheck := ""
	for i, p := range parts {
		if i > 0 {
			dataCheck += "\n"
		}
		dataCheck += p
	}
	return hmacSHA256([]byte(secretKey), []byte(dataCheck))
}

func TestVerifyTelegramWidget_Valid(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix()

	parts := []string{
		fmt.Sprintf("auth_date=%d", authDate),
		"first_name=Test",
		fmt.Sprintf("id=%d", 699469085),
		"username=testuser",
	}
	hash := computeWidgetHash(parts, botToken)

	u := TelegramUser{
		ID:        699469085,
		FirstName: "Test",
		Username:  "testuser",
		AuthDate:  authDate,
		Hash:      hash,
	}
	err := VerifyTelegramWidget(u, botToken, 86400)
	require.NoError(t, err)
}

func TestVerifyTelegramWidget_InvalidHash(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix()

	u := TelegramUser{
		ID:        699469085,
		FirstName: "Test",
		AuthDate:  authDate,
		Hash:      "deadbeef",
	}
	err := VerifyTelegramWidget(u, botToken, 86400)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hash")
}

func TestVerifyTelegramWidget_MissingHash(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	u := TelegramUser{ID: 699469085, AuthDate: time.Now().Unix()}
	err := VerifyTelegramWidget(u, botToken, 86400)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing hash")
}

func TestVerifyTelegramWidget_StaleAuthDate(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix() - 100000

	parts := []string{
		fmt.Sprintf("auth_date=%d", authDate),
		fmt.Sprintf("id=%d", 699469085),
	}
	hash := computeWidgetHash(parts, botToken)

	u := TelegramUser{
		ID:       699469085,
		AuthDate: authDate,
		Hash:     hash,
	}
	err := VerifyTelegramWidget(u, botToken, 86400)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale auth_date")
}

func TestVerifyTelegramWidget_NoMaxAge(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix() - 100000

	parts := []string{
		fmt.Sprintf("auth_date=%d", authDate),
		fmt.Sprintf("id=%d", 699469085),
	}
	hash := computeWidgetHash(parts, botToken)

	u := TelegramUser{
		ID:       699469085,
		AuthDate: authDate,
		Hash:     hash,
	}
	err := VerifyTelegramWidget(u, botToken, 0)
	require.NoError(t, err)
}

func TestVerifyTelegramWidget_FutureAuthDate(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix() + 100000

	parts := []string{
		fmt.Sprintf("auth_date=%d", authDate),
		fmt.Sprintf("id=%d", 699469085),
	}
	hash := computeWidgetHash(parts, botToken)

	u := TelegramUser{
		ID:       699469085,
		AuthDate: authDate,
		Hash:     hash,
	}
	err := VerifyTelegramWidget(u, botToken, 86400)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale auth_date")
}

func TestVerifyTelegramWidget_OptionalFields(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	authDate := time.Now().Unix()

	parts := []string{
		fmt.Sprintf("auth_date=%d", authDate),
		"first_name=Test",
		fmt.Sprintf("id=%d", 699469085),
		"last_name=User",
		"photo_url=https://example.com/photo.jpg",
		"username=testuser",
	}
	hash := computeWidgetHash(parts, botToken)

	u := TelegramUser{
		ID:        699469085,
		FirstName: "Test",
		LastName:  "User",
		Username:  "testuser",
		PhotoURL:  "https://example.com/photo.jpg",
		AuthDate:  authDate,
		Hash:      hash,
	}
	err := VerifyTelegramWidget(u, botToken, 86400)
	require.NoError(t, err)
}

func TestSha256Sum(t *testing.T) {
	result := sha256Sum([]byte("test"))
	expected := sha256.Sum256([]byte("test"))
	assert.Equal(t, hex.EncodeToString(expected[:]), result)
}

func TestHmacSHA256(t *testing.T) {
	key := []byte("key")
	msg := []byte("message")
	result := hmacSHA256(key, msg)

	h := hmac.New(sha256.New, key)
	h.Write(msg)
	expected := hex.EncodeToString(h.Sum(nil))

	assert.Equal(t, expected, result)
}
