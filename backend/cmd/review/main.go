package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	reviewapp "goshop/services/review/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Review Service API
// @version         1.0
// @description     Dedicated Review microservice owning product reviews, helpful votes, and reports. Rating aggregates propagate to the catalog via RabbitMQ.
// @host            localhost:8091
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("review", "review.log")
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
		logger.Log.Warn().Err(err).Msg("No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Review Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-review", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if reviewDBName := utils.GetEnv("REVIEW_DB_NAME", utils.GetEnv("DB_REVIEW_NAME", "shopping_review")); reviewDBName != "" {
		cfg.DB.DBName = reviewDBName
	}
	cfg.ServerAddress = ":" + utils.GetEnv("REVIEW_SERVER_PORT", "8091")

	app, err := reviewapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize Review Service")
	}

	if err := app.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Review Service exited with error")
	}
}
