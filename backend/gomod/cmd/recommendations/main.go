package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	recsapp "goshop/services/recommendations/app"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
)

// @title           GOshop Recommendations Service API
// @version         1.0
// @description     Best-seller shelves for the storefront, derived from order history with a rating fallback.
// @host            localhost:8095
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("recommendations", "recommendations.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Recommendations Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-recommendations", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if recsDBName := utils.GetEnv("RECOMMENDATIONS_DB_NAME", utils.GetEnv("DB_RECOMMENDATIONS_NAME", "shopping_recs")); recsDBName != "" {
		cfg.DB.DBName = recsDBName
	}
	port := utils.GetEnv("RECOMMENDATIONS_SERVER_PORT", "8095")
	cfg.ServerAddress = ":" + port

	application, err := recsapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create recommendations application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Recommendations application failed to run")
	}
}
