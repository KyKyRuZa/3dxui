package billing

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenIdempotencyKey(t *testing.T) {
	key := GenIdempotencyKey()
	assert.Len(t, key, 32)

	key2 := GenIdempotencyKey()
	assert.NotEqual(t, key, key2)
}

func TestAuthHeader(t *testing.T) {
	client := New("shop123", "secret456", "https://api.yookassa.ru/v3")
	header := client.authHeader()
	assert.True(t, strings.HasPrefix(header, "Basic "))
	decoded, err := base64.StdEncoding.DecodeString(header[len("Basic "):])
	assert.NoError(t, err)
	assert.Equal(t, "shop123:secret456", string(decoded))
}
