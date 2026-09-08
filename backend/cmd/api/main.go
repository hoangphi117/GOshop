package main

import (
	"goshop/internal/app"
	"goshop/internal/config"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {

	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/app.log")
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
		logger.Log.Warn().Msg("⚠️ No .env found")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in API!")
	}

	cfg := config.NewConfig()

	application, err := app.NewApplication(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create application")
	}

	if err := application.Run(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Application failed to run")
	}
}
