package handlers

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"go.uber.org/zap"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"

	"github.com/redis/go-redis/v9"
)

// Handler holds dependencies shared by all HTTP handlers.
type Handler struct {
	store   *store.Store
	jwt     *auth.TokenService
	cfg     *config.Config
	panel   *panel.Client
	billing *billing.Client
	redis   *redis.Client
	log     *zap.SugaredLogger
}

func NewHandler(s *store.Store, j *auth.TokenService, cfg *config.Config, p *panel.Client, b *billing.Client, rdb *redis.Client, log *zap.SugaredLogger) *Handler {
	return &Handler{store: s, jwt: j, cfg: cfg, panel: p, billing: b, redis: rdb, log: log}
}

func maskInt(v int64) string {
	if v == 0 {
		return "0"
	}
	return fmt.Sprintf("***%d", v%10000)
}

func maskStr(s string) string {
	if s == "" {
		return ""
	}
	// keep domain part only for emails/panel emails
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return "***" + s[idx:]
	}
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// RegisterRoutes wires all application routes onto the engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.health)
	api := r.Group("/api")
	{
		api.GET("/health", h.health)
		api.GET("/config", h.publicConfig)

		auth := api.Group("/auth")
		{
			auth.POST("/register", middleware.AuthAttemptLimiter(h.redis), h.register)
			auth.POST("/login", middleware.AuthAttemptLimiter(h.redis), h.login)
			auth.POST("/telegram", h.telegram)
			auth.POST("/telegram/widget", h.telegramWidget)
			auth.POST("/telegram/link", h.telegramLink)
			auth.GET("/telegram/link/:token", h.telegramLinkCheck)
			auth.POST("/refresh", h.refresh)
			auth.POST("/logout", h.logout)
			auth.GET("/profile", middleware.AuthRequired(h.jwt), h.profile)
			auth.POST("/verify-login-code", middleware.CodeAttemptLimiter(h.redis), h.verifyLoginCode)
		}

		// Telegram bind/login code endpoints
		codes := api.Group("/codes")
		{
			// Public: generate login code for Telegram user (called by bot)
			codes.POST("/login", middleware.CodeAttemptLimiter(h.redis), h.generateLoginCode)
			// Authenticated: generate bind code for current user
			codes.POST("/bind", middleware.AuthRequired(h.jwt), middleware.CodeAttemptLimiter(h.redis), h.generateBindCode)
			// Authenticated: verify bind code (called by bot or frontend)
			codes.POST("/bind/verify", middleware.AuthRequired(h.jwt), middleware.CodeAttemptLimiter(h.redis), h.verifyBindCode)
		}

		sub := api.Group("/subscription")
		sub.Use(middleware.AuthRequired(h.jwt))
		{
			sub.POST("/activate", h.activateSubscription)
			sub.GET("/", h.getSubscription)
			sub.GET("/config", h.getVLESSConfig)
			sub.GET("/config/singbox", h.getSingBoxConfig)
		}

		bot := api.Group("/bot")
		bot.Use(middleware.BotRequired(h.cfg.BotAPISecret))
		{
			bot.POST("/user", h.botEnsureUser)
			bot.GET("/user/:telegram_id", h.botGetUser)
			bot.GET("/notifications/expiring", h.botExpiring)
			bot.GET("/notifications/expired", h.botExpired)
			bot.GET("/notifications/renewed", h.botRenewed)
			bot.GET("/notifications/pending", h.botNotifications)
			bot.POST("/referral", h.botReferral)
			bot.POST("/login-token/claim", h.botClaimLoginToken)
		}

		bill := api.Group("/billing")
		{
			bill.GET("/plans", middleware.AuthRequired(h.jwt), h.listPlans)
			bill.POST("/create", middleware.AuthRequired(h.jwt), h.createPayment)
			bill.POST("/webhook", middleware.WebhookLimiter(h.redis), h.billingWebhook)
		}

		ref := api.Group("/referral")
		ref.Use(middleware.AuthRequired(h.jwt))
		{
			ref.GET("/", h.webReferral)
		}

		// User data management endpoints (152-FZ compliance)
		user := api.Group("/user")
		user.Use(middleware.AuthRequired(h.jwt))
		{
			user.GET("/data-export", h.dataExport)
			user.DELETE("/", h.deleteUser)
			user.POST("/consent", h.recordConsent)
		}

		admin := api.Group("/admin")
		{
			admin.POST("/login", h.adminLogin)
			admin.POST("/logout", h.adminLogout)
			admin.Use(h.adminAuth())
			admin.GET("/plans", h.adminListPlans)
			admin.POST("/plans", h.adminCreatePlan)
			admin.PUT("/plans/:id", h.adminUpdatePlan)
			admin.DELETE("/plans/:id", h.adminDeletePlan)
			admin.GET("/discounts", h.adminListDiscounts)
			admin.POST("/discounts", h.adminCreateDiscount)
			admin.PUT("/discounts/:id", h.adminUpdateDiscount)
			admin.DELETE("/discounts/:id", h.adminDeleteDiscount)
		}
	}
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// publicConfig exposes non-sensitive, public runtime flags the frontend needs
// (e.g. whether the YooKassa store is in test mode) without requiring auth.
func (h *Handler) publicConfig(c *gin.Context) {
	c.JSON(200, gin.H{
		"yookassa_test_mode": h.cfg.YookassaTestMode,
		"bot_username":       h.cfg.BotUsername,
	})
}
