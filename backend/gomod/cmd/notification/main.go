package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	notificationapp "goshop/services/notification/app"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
)

// @title           GOshop Notification Service API
// @version         1.0
// @description     Dedicated Notification microservice handling In-App notifications, Email, Discord and Kafka topic events.
// @host            localhost:8082
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("notification", "notification.log")
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
		logger.Log.Warn().Msg("WARNING: No .env found")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Notification Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-notification", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if notifDBName := utils.GetEnv("NOTIFICATION_DB_NAME", utils.GetEnv("DB_NOTIFICATION_NAME", "shopping_notification")); notifDBName != "" {
		cfg.DB.DBName = notifDBName
	}
	port := utils.GetEnv("NOTIFICATION_SERVER_PORT", "8082")
	cfg.ServerAddress = ":" + port

	application, err := notificationapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create notification application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Notification application failed to run")
	}
}
