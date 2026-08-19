package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ilyas/vpn-service/backend/internal/config"
)

const (
	issuer       = "nexus"
	accessTTL    = 15 * time.Minute
	refreshTTL   = 30 * 24 * time.Hour
	refreshBytes = 32
)

// ErrInvalidToken is returned when a JWT fails to verify.
var ErrInvalidToken = errors.New("invalid token")

// TokenService signs and verifies ES256 JWTs.
type TokenService struct {
	priv *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
}

// NewTokenService loads (or generates) the EC P-256 key used for signing.
func NewTokenService(cfg *config.Config) (*TokenService, error) {
	if pem := os.Getenv("JWT_PRIVATE_KEY"); pem != "" {
		key, err := parsePEM(pem)
		if err != nil {
			return nil, fmt.Errorf("parse JWT_PRIVATE_KEY: %w", err)
		}
		return &TokenService{priv: key, pub: &key.PublicKey}, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	fmt.Println("[warn] JWT_PRIVATE_KEY not set: generated ephemeral EC key; refresh tokens will not survive restarts")
	return &TokenService{priv: key, pub: &key.PublicKey}, nil
}

// NewAccessToken issues a short-lived access token for the user.
func (t *TokenService) NewAccessToken(userID int64, username string) (string, error) {
	now := time.Now()
	claims := map[string]any{
		"iss": issuer,
		"sub": fmt.Sprintf("%d", userID),
		"usr": username,
		"typ": "access",
		"iat": now.Unix(),
		"exp": now.Add(accessTTL).Unix(),
	}
	return t.sign(claims)
}

// VerifyAccessToken validates an access token and returns its claims.
func (t *TokenService) VerifyAccessToken(token string) (map[string]any, error) {
	claims, err := t.verify(token)
	if err != nil {
		return nil, err
	}
	if typ, _ := claims["typ"].(string); typ != "access" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// AccessTTL exposes the configured access-token lifetime.
func AccessTTL() time.Duration { return accessTTL }

// RefreshTTL exposes the configured refresh-token lifetime.
func RefreshTTL() time.Duration { return refreshTTL }

// --- low level JWT (ES256, raw R||S signature) ---

func (t *TokenService) sign(claims map[string]any) (string, error) {
	header := map[string]any{"alg": "ES256", "typ": "JWT"}
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	p, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64url(h) + "." + b64url(p)
	sum := sha256.Sum256([]byte(signingInput))

	var r, s *big.Int
	r, s, err = ecdsa.Sign(rand.Reader, t.priv, sum[:])
	if err != nil {
		return "", err
	}
	sig := append(fixedBytes(r, 32), fixedBytes(s, 32)...)
	return signingInput + "." + b64url(sig), nil
}

func (t *TokenService) verify(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	headerJSON, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "ES256" {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := b64urlDecode(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, ErrInvalidToken
		}
	}

	sig, err := b64urlDecode(parts[2])
	if err != nil || len(sig) != 64 {
		return nil, ErrInvalidToken
	}
	var r, s big.Int
	r.SetBytes(sig[:32])
	s.SetBytes(sig[32:])

	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(t.pub, sum[:], &r, &s) {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func b64url(b []byte) string                { return base64.RawURLEncoding.EncodeToString(b) }
func b64urlDecode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func fixedBytes(b *big.Int, n int) []byte {
	out := make([]byte, n)
	raw := b.Bytes()
	copy(out[n-len(raw):], raw)
	return out
}
