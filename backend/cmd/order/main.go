package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	orderapp "goshop/services/order/app"
)

// @title           GOshop Order Service API
// @version         1.0
// @description     Dedicated Order microservice providing checkout saga, order lifecycle, tracking, and cross-domain gRPC contracts.
// @host            localhost:8088
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("order", "order.log")
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
		logger.Log.Warn().Err(err).Msg("WARNING: No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Order Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-order", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if orderDBName := utils.GetEnv("ORDER_DB_NAME", utils.GetEnv("DB_ORDER_NAME", "shopping_order")); orderDBName != "" {
		cfg.DB.DBName = orderDBName
	}
	port := utils.GetEnv("ORDER_SERVER_PORT", "8088")
	cfg.ServerAddress = ":" + port

	application, err := orderapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create order application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Order application failed to run")
	}
}
