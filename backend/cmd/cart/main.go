package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	cartapp "goshop/services/cart/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Cart Service API
// @version         1.0
// @description     Dedicated Cart microservice providing persistent shopping cart management, stock verification, and gRPC endpoints.
// @host            localhost:8087
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("cart", "cart.log")
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
		logger.Log.Warn().Err(err).Msg("âš ï¸ No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Cart Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-cart", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if cartDBName := utils.GetEnv("CART_DB_NAME", utils.GetEnv("DB_CART_NAME", "shopping_cart")); cartDBName != "" {
		cfg.DB.DBName = cartDBName
	}
	port := utils.GetEnv("CART_SERVER_PORT", "8087")
	cfg.ServerAddress = ":" + port

	application, err := cartapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create cart application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Cart application failed to run")
	}
}
