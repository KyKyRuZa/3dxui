package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

// Handler holds dependencies shared by all HTTP handlers.
type Handler struct {
	store   *store.Store
	jwt     *auth.TokenService
	cfg     *config.Config
	panel   *panel.Client
	billing *billing.Client
}

func NewHandler(s *store.Store, j *auth.TokenService, cfg *config.Config, p *panel.Client, b *billing.Client) *Handler {
	return &Handler{store: s, jwt: j, cfg: cfg, panel: p, billing: b}
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
			auth.POST("/register", h.register)
			auth.POST("/login", h.login)
			auth.POST("/refresh", h.refresh)
			auth.POST("/logout", h.logout)
			auth.POST("/telegram", h.telegram)
			auth.GET("/profile", middleware.AuthRequired(h.jwt), h.profile)
			auth.PATCH("/profile", middleware.AuthRequired(h.jwt), h.updateProfile)
			auth.POST("/password", middleware.AuthRequired(h.jwt), h.changePassword)
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
			bot.POST("/referral", h.botReferral)
		}

		bill := api.Group("/billing")
		{
			bill.GET("/plans", middleware.AuthRequired(h.jwt), h.listPlans)
			bill.POST("/create", middleware.AuthRequired(h.jwt), h.createPayment)
			bill.POST("/webhook", h.billingWebhook)
		}

		ref := api.Group("/referral")
		ref.Use(middleware.AuthRequired(h.jwt))
		{
			ref.GET("/", h.webReferral)
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
	})
}
