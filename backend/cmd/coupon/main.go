package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	couponapp "goshop/services/coupon/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Coupon Service API
// @version         1.0
// @description     Dedicated Coupon microservice providing discount calculation, reservation, idempotency, and gRPC contracts.
// @host            localhost:8084
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("coupon", "coupon.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Coupon Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-coupon", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if couponDBName := utils.GetEnv("COUPON_DB_NAME", utils.GetEnv("DB_COUPON_NAME", "shopping_coupon")); couponDBName != "" {
		cfg.DB.DBName = couponDBName
	}
	port := utils.GetEnv("COUPON_SERVER_PORT", "8084")
	cfg.ServerAddress = ":" + port

	application, err := couponapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create coupon application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Coupon application failed to run")
	}
}
