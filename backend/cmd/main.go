package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/db"
	"github.com/ilyas/vpn-service/backend/internal/handlers"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
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

	jwtSvc, err := auth.NewTokenService(cfg)
	if err != nil {
		sugar.Fatalf("Failed to init token service: %v", err)
	}

	st := store.New(pg)
	h := handlers.NewHandler(st, jwtSvc, cfg)

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
