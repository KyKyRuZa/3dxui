package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccessToken_VerifyRoundTrip(t *testing.T) {
	svc := newTestService(t)
	token, err := svc.NewAccessToken(42, "testuser")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.VerifyAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, "42", claims["sub"])
	assert.Equal(t, "testuser", claims["usr"])
	assert.Equal(t, "access", claims["typ"])
	assert.Equal(t, issuer, claims["iss"])
}

func TestVerifyAccessToken_InvalidToken(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.VerifyAccessToken("not.a.jwt")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerifyAccessToken_TamperedSignature(t *testing.T) {
	svc := newTestService(t)
	token, err := svc.NewAccessToken(42, "testuser")
	require.NoError(t, err)

	tampered := token[:len(token)-4] + "dead"
	_, err = svc.VerifyAccessToken(tampered)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerifyAccessToken_WrongType(t *testing.T) {
	svc := newTestService(t)
	claims := map[string]any{
		"iss": issuer,
		"sub": "42",
		"usr": "testuser",
		"typ": "refresh",
	}
	token, err := svc.sign(claims)
	require.NoError(t, err)

	_, err = svc.VerifyAccessToken(token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerifyAccessToken_EmptyParts(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.VerifyAccessToken("onlytwo.parts")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerifyAccessToken_WrongAlgorithm(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.VerifyAccessToken("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiI0Mg.dead")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestNewRefreshToken(t *testing.T) {
	raw, hash := NewRefreshToken()
	assert.NotEmpty(t, raw)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, raw, hash)
}

func TestHashRefreshToken(t *testing.T) {
	raw, _ := NewRefreshToken()
	hash := HashRefreshToken(raw)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, raw, hash)

	hash2 := HashRefreshToken(raw)
	assert.Equal(t, hash, hash2)
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mypassword")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mypassword", hash)
}

func TestCheckPassword(t *testing.T) {
	hash, err := HashPassword("mypassword")
	require.NoError(t, err)

	assert.True(t, CheckPassword(hash, "mypassword"))
	assert.False(t, CheckPassword(hash, "wrongpassword"))
}

func TestParsePEM_PKCS8(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	pemData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	parsed, err := parsePEM(string(pemData))
	require.NoError(t, err)
	assert.Equal(t, key.X, parsed.X)
	assert.Equal(t, key.Y, parsed.Y)
}

func TestParsePEM_InvalidBlock(t *testing.T) {
	_, err := parsePEM("not-a-pem")
	assert.Error(t, err)
}

func TestParsePEM_UnsupportedFormat(t *testing.T) {
	pemData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("garbage")})
	_, err := parsePEM(string(pemData))
	assert.Error(t, err)
}

func TestAccessTTL(t *testing.T) {
	assert.Equal(t, accessTTL, AccessTTL())
}

func TestRefreshTTL(t *testing.T) {
	assert.Equal(t, refreshTTL, RefreshTTL())
}

func newTestService(t *testing.T) *TokenService {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &TokenService{priv: key, pub: &key.PublicKey}
}
