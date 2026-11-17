package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	assistantapp "goshop/services/assistant/app"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
)

// @title           GOshop Assistant Service API
// @version         1.0
// @description     AI shopping assistant: grounded product advice and FAQ answers.
// @host            localhost:8097
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("assistant", "assistant.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Assistant Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-assistant", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()

	application, err := assistantapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create assistant application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Assistant application failed to run")
	}
}
