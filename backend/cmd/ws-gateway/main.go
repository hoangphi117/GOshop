package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"goshop/internal/config"
	"goshop/internal/db"
	v1handler "goshop/internal/handler/v1"
	"goshop/pkg/middleware"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/ws"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/ws-gateway.log")
	logger.InitLogger(logger.LoggerConfig{
		Level:      "info",
		Filename:   logFile,
		MaxSize:    1,
		MaxBackups: 5,
		MaxAge:     5,
		Compress:   true,
		IsDev:      utils.GetEnv("APP_ENV", "development"),
	})

	if err := godotenv.Load(filepath.Join(rootDir, ".env")); err != nil {
		logger.Log.Warn().Msg("No .env found for ws-gateway")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully for WS Gateway")
	}

	_ = config.NewConfig()

	// 1. Initialize Database for User & Chat authentication
	if err := db.InitDB(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for WS Gateway")
	}

	// 2. Initialize Redis and Auth Services
	redisClient := config.NewRedisClient()
	cacheService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheService)

	userRepo := repository.NewSqlUserRepository(db.DB)
	chatRepo := repository.NewSqlChatRepository(db.DB)

	// 3. Initialize WebSocket Hub & Redis Bridge
	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go redisBridge.StartSubscriber(ctx)

	// 4. Initialize WebSocket Handler
	wsHandler := v1handler.NewWSHandler(wsHub, tokenService, cacheService, userRepo)
	wsHandler.SetChatRepo(chatRepo)

	// 5. Setup Gin Engine for WS Gateway
	if utils.GetEnv("APP_ENV", "development") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	// Health check endpoint for ingress / load balancer
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "ws-gateway",
			"has_admin": wsHub.HasOnlineAdmin(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// WebSocket connection endpoints
	r.GET("/ws", wsHandler.HandleWS)
	r.GET("/api/v1/ws", wsHandler.HandleWS)

	port := utils.GetEnv("WS_GATEWAY_PORT", "8085")
	addr := fmt.Sprintf(":%s", port)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		logger.Log.Info().Str("port", port).Msg("🚀 WebSocket Gateway listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("WebSocket Gateway failed to serve")
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("Shutting down WebSocket Gateway...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Server forced to shutdown")
	}

	redisBridge.Stop()
	wsHub.Shutdown()

	logger.Log.Info().Msg("WebSocket Gateway exited cleanly")
}
