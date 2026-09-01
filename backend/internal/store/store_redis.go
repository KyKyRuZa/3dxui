package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// VerificationCode represents a temporary code for Telegram bind/login.
type VerificationCode struct {
	Code       string `json:"code"`
	TelegramID int64  `json:"telegram_id"`
	Purpose    string `json:"purpose"` // "bind" or "login"
	UserID     *int64 `json:"user_id,omitempty"`
}

// SetVerificationCode stores a code in Redis with TTL (5 minutes).
func (s *Store) SetVerificationCode(ctx context.Context, code string, v VerificationCode, ttl time.Duration) error {
	if s.redis == nil {
		return errors.New("redis not available")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal verification code: %w", err)
	}
	return s.redis.Set(ctx, "vcode:"+code, data, ttl).Err()
}

// GetVerificationCode retrieves a code from Redis.
func (s *Store) GetVerificationCode(ctx context.Context, code string) (*VerificationCode, error) {
	if s.redis == nil {
		return nil, errors.New("redis not available")
	}
	data, err := s.redis.Get(ctx, "vcode:"+code).Bytes()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var v VerificationCode
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal verification code: %w", err)
	}
	return &v, nil
}

// DeleteVerificationCode removes a code from Redis.
func (s *Store) DeleteVerificationCode(ctx context.Context, code string) error {
	if s.redis == nil {
		return errors.New("redis not available")
	}
	return s.redis.Del(ctx, "vcode:"+code).Err()
}
