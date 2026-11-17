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

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	v1handler "goshop/cmd/api/handler"
	chatcontract "goshop/services/chat/contract"
	identitycontract "goshop/services/identity/contract"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/ws"
)

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("ws-gateway", "ws-gateway.log")
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
		logger.Log.Warn().Msg("WARNING: No .env found for ws-gateway")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully for WS Gateway")
	}

	_ = config.NewConfig()

	// 1. Initialize Redis and Auth Services
	redisClient := config.NewRedisClient()
	cacheService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheService)

	// User lookups go through the identity service; the monolith users table is
	// gone, so the gateway no longer opens the monolith database at all.

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-ws-gateway", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	userResolver, err := identitycontract.NewGRPCIdentityClient(cfg.IdentityGRPCAddr, redisClient)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", cfg.IdentityGRPCAddr).Msg("Failed to initialize Identity gRPC client")
	}

	// Chat room resolver: chat moved into its own service; rooms are resolved
	// over gRPC instead of querying the (now stale) monolith chat tables.
	chatTarget := utils.GetEnv("CHAT_GRPC_TARGET", "localhost:50058")
	chatClient, err := chatcontract.NewGRPCChatClient(chatTarget)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", chatTarget).Msg("Failed to initialize Chat gRPC client")
	} // 3. Initialize WebSocket Hub, Redis Bridge, and presence store
	wsHub := ws.NewHub()
	wsHub.SetPresenceStore(ws.NewPresenceStore(redisClient))
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go redisBridge.StartSubscriber(ctx)

	// 4. Initialize WebSocket Handler
	wsHandler := v1handler.NewWSHandler(wsHub, tokenService, cacheService, userResolver)
	wsHandler.SetChatClient(chatClient)

	// 5. Setup Gin Engine for WS Gateway
	if utils.GetEnv("APP_ENV", "development") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.GET("/metrics", metrics.Handler(nil))

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
		logger.Log.Info().Str("port", port).Msg("ðŸš€ WebSocket Gateway listening")
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
	_ = chatClient.Close()
	if c, ok := userResolver.(*identitycontract.GRPCIdentityClient); ok {
		_ = c.Close()
	}

	logger.Log.Info().Msg("WebSocket Gateway exited cleanly")
}
