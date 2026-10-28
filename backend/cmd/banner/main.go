package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	bannerapp "goshop/services/banner/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Banner Service API
// @version         1.0
// @description     Dedicated Banner microservice providing cached banner reads for the storefront and admin CRUD management.
// @host            localhost:8089
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/banner.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Banner Service!")
	}

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
