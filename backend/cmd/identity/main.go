package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	identityapp "goshop/services/identity/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Identity Service API
// @version         1.0
// @description     Dedicated Identity microservice hosting Auth, User, and Address modules.
// @host            localhost:8081
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/identity.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Identity Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-identity", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if identityDBName := utils.GetEnv("IDENTITY_DB_NAME", utils.GetEnv("DB_IDENTITY_NAME", "shopping_identity")); identityDBName != "" {
		cfg.DB.DBName = identityDBName
	}
	port := utils.GetEnv("IDENTITY_SERVER_PORT", "8081")
	cfg.ServerAddress = ":" + port

	application, err := identityapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create identity application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Identity application failed to run")
	}
}
