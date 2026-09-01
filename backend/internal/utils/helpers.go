package utils

import (
	"crypto/rand"
	"fmt"
)

func RandString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// RandDigits generates a random n-digit numeric string (e.g. "48271593").
func RandDigits(n int) string {
	const digits = "0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = digits[int(b[i])%len(digits)]
	}
	// Ensure first digit is not 0
	if b[0] == '0' {
		b[0] = '1'
	}
	return string(b)
}

// GenIdempotencyKey returns a hex string for YooKassa Idempotence-Key.
func GenIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
