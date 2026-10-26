package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	mediaapp "goshop/services/media/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Media Service API
// @version         1.0
// @description     Dedicated Media microservice: file uploads (multipart fallback via the monolith and direct-to-storage presigned URLs) plus /uploads serving. Stateless — no database, backends are local disk, S3, MinIO or Cloudflare R2 through pkg/storage.
// @host            localhost:8094
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/media.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Media Service!")
	}

	cfg := config.NewConfig()

	app, err := mediaapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize Media Service")
	}

	if err := app.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Media Service exited with error")
	}
}
