package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAmountMinorFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"299.00", 29900},
		{"799.00", 79900},
		{"0.00", 0},
		{"", 0},
		{"invalid", 0},
		{"299", 29900},
		{"299.5", 29950},
		{"299.505", 29950},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := amountMinorFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
