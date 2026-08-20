package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

// Handler holds dependencies shared by all HTTP handlers.
type Handler struct {
	store *store.Store
	jwt   *auth.TokenService
	cfg   *config.Config
	panel *panel.Client
}

func NewHandler(s *store.Store, j *auth.TokenService, cfg *config.Config, p *panel.Client) *Handler {
	return &Handler{store: s, jwt: j, cfg: cfg, panel: p}
}

// RegisterRoutes wires all application routes onto the engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.health)
	api := r.Group("/api")
	{
		api.GET("/health", h.health)

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
		}
	}
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
