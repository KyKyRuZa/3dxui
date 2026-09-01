package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/db"
	"github.com/ilyas/vpn-service/backend/internal/handlers"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	pg, err := db.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		sugar.Fatalf("Failed to connect to database: %v", err)
	}
	defer pg.Close()

	if err := db.Migrate(pg); err != nil {
		sugar.Fatalf("Failed to migrate database: %v", err)
	}

	redisClient, err := db.NewRedis(cfg.RedisURL)
	if err != nil {
		sugar.Fatalf("Failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	jwtSvc, err := auth.NewTokenService(cfg, sugar)
	if err != nil {
		sugar.Fatalf("Failed to init token service: %v", err)
	}

	st := store.New(pg, redisClient)
	panelClient := panel.New(cfg.PanelURL, cfg.PanelUsername, cfg.PanelPassword, cfg.PanelAPIToken, sugar)
	billingClient := billing.New(cfg.YookassaShopID, cfg.YookassaSecretKey, cfg.YookassaAPIURL)
	h := handlers.NewHandler(st, jwtSvc, cfg, panelClient, billingClient, sugar)

	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.CORS(cfg.CorsOrigins()))

	h.RegisterRoutes(r)

	port := cfg.Port
	sugar.Infow("starting server", "port", port)
	if err := r.Run(":" + port); err != nil {
		sugar.Fatalw("server shutdown", "error", err)
	}
}
