package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	chatapp "goshop/services/chat/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Chat Service API
// @version         1.0
// @description     Dedicated Chat microservice: customer-admin conversations and messages in its own database, realtime fan-out through the WebSocket gateway's Redis bridge, presence over the shared Redis ZSET.
// @host            localhost:8093
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/chat.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Chat Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-chat", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if chatDBName := utils.GetEnv("CHAT_DB_NAME", utils.GetEnv("DB_CHAT_NAME", "shopping_chat")); chatDBName != "" {
		cfg.DB.DBName = chatDBName
	}

	app, err := chatapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize Chat Service")
	}

	if err := app.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Chat Service exited with error")
	}
}
