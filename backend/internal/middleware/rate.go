package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	prefix string
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration, prefix string) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window, prefix: prefix}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.rdb == nil || rl.limit <= 0 {
			c.Next()
			return
		}

		key := rl.key(c)
		ctx := c.Request.Context()
		count, err := rl.rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			_ = rl.rdb.Expire(ctx, key, rl.window).Err()
		}

		if count > int64(rl.limit) {
			c.Header("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) key(c *gin.Context) string {
	ip := c.ClientIP()
	id := c.GetString("user_id")
	if id == "" {
		id = ip
	}
	return rl.prefix + ":" + id
}

func AuthAttemptLimiter(rdb *redis.Client) gin.HandlerFunc {
	return NewRateLimiter(rdb, 10, 5*time.Minute, "auth_attempt").Middleware()
}

func CodeAttemptLimiter(rdb *redis.Client) gin.HandlerFunc {
	return NewRateLimiter(rdb, 10, 5*time.Minute, "code_attempt").Middleware()
}

func WebhookLimiter(rdb *redis.Client) gin.HandlerFunc {
	return NewRateLimiter(rdb, 100, time.Minute, "webhook").Middleware()
}

func isPrivateIP(ip string) bool {
	private := []string{
		"127.0.0.1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1",
	}
	for _, cidr := range private {
		if strings.Contains(cidr, "/") {
			_, network, err := parseCIDR(cidr)
			if err == nil && network.Contains(net.ParseIP(ip)) {
				return true
			}
			continue
		}
		if ip == cidr {
			return true
		}
	}
	return false
}

func parseCIDR(cidr string) (net.IP, *net.IPNet, error) {
	_, network, err := net.ParseCIDR(cidr)
	return nil, network, err
}
