package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
)

// CORS allows only the configured origins. It never reflects an arbitrary
// Origin, which would be unsafe together with credentials.
func CORS(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allowedSet[strings.TrimSpace(o)] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowedSet[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthRequired validates the Bearer access token and stores claims in context.
func AuthRequired(jwt *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := c.GetHeader("Authorization")
		if bearer == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		parts := strings.SplitN(bearer, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims, err := jwt.VerifyAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims["sub"])
		c.Set("username", claims["usr"])
		c.Next()
	}
}

// AdminRequired validates the admin shared secret from X-Admin-Secret header.
func AdminRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not configured"})
			return
		}
		token := c.GetHeader("X-Admin-Secret")
		if token == "" || token != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin secret"})
			return
		}
		c.Next()
	}
}

// BotRequired validates the bot shared secret from X-Bot-Secret header.
// This protects the bot-facing API used by the Telegram bot to provision
// users and deliver notifications.
func BotRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not configured"})
			return
		}
		token := c.GetHeader("X-Bot-Secret")
		if token == "" || token != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid bot secret"})
			return
		}
		c.Next()
	}
}
