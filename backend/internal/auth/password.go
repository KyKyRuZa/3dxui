package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	return bcryptHash(password)
}

// CheckPassword reports whether the plaintext matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcryptCompare(hash, password)
}

// NewRefreshToken returns a cryptographically random refresh token (hex).
func NewRefreshToken() (raw string, hash string) {
	b := make([]byte, refreshBytes)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	h := sha256.Sum256(b)
	hash = hex.EncodeToString(h[:])
	return raw, hash
}

// HashRefreshToken hashes a refresh token for storage/lookup.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
