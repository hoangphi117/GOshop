package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	bannerapp "goshop/services/banner/app"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
)

// @title           GOshop Banner Service API
// @version         1.0
// @description     Dedicated Banner microservice providing cached banner reads for the storefront and admin CRUD management.
// @host            localhost:8089
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("banner", "banner.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Banner Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-banner", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if bannerDBName := utils.GetEnv("BANNER_DB_NAME", utils.GetEnv("DB_BANNER_NAME", "shopping_banner")); bannerDBName != "" {
		cfg.DB.DBName = bannerDBName
	}
	port := utils.GetEnv("BANNER_SERVER_PORT", "8089")
	cfg.ServerAddress = ":" + port

	application, err := bannerapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create banner application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Banner application failed to run")
	}
}
