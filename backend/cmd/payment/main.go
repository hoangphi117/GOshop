package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	paymentapp "goshop/services/payment/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Payment Service API
// @version         1.0
// @description     Dedicated Payment microservice handling MoMo/COD payments, webhooks, transaction reconciliation and gRPC contracts.
// @host            localhost:8083
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/payment.log")
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
		logger.Log.Warn().Err(err).Msg("⚠️ No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Payment Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-payment", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if paymentDBName := utils.GetEnv("PAYMENT_DB_NAME", utils.GetEnv("DB_PAYMENT_NAME", "shopping_payment")); paymentDBName != "" {
		cfg.DB.DBName = paymentDBName
	}
	port := utils.GetEnv("PAYMENT_SERVER_PORT", "8083")
	cfg.ServerAddress = ":" + port

	application, err := paymentapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create payment application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Payment application failed to run")
	}
}
