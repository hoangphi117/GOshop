package main

import (
	"goshop/internal/app"
	"goshop/internal/config"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {

	rootDir := mustGetwd()

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

	loadEnv(filepath.Join(rootDir, ".env"))

	cfg := config.NewConfig()

	application := app.NewApplication(cfg)

	if err := application.Run(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Application failed to run")
	}
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	return dir
}

func loadEnv(path string) {
	if err := godotenv.Load(path); err != nil {
		logger.Log.Warn().Msg("⚠️ No .env found")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully!")
	}
}
