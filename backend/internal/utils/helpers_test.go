package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandString(t *testing.T) {
	result := RandString(8)
	assert.Len(t, result, 8)

	result2 := RandString(16)
	assert.Len(t, result2, 16)

	result3 := RandString(0)
	assert.Len(t, result3, 0)
}

func TestRandStringUnique(t *testing.T) {
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := RandString(8)
		assert.False(t, results[s], "duplicate random string generated")
		results[s] = true
	}
}
