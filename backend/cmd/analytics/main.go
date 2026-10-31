package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	analyticsapp "goshop/services/analytics/app"
)

// @title           GOshop Analytics Service API
// @version         1.0
// @description     Dedicated Analytics microservice: admin reporting dashboards backed by its own projection database, fed by order lifecycle events and cross-service gRPC reads.
// @host            localhost:8092
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("analytics", "analytics.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in Analytics Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-analytics", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if analyticsDBName := utils.GetEnv("ANALYTICS_DB_NAME", utils.GetEnv("DB_ANALYTICS_NAME", "shopping_analytics")); analyticsDBName != "" {
		cfg.DB.DBName = analyticsDBName
	}
	cfg.ServerAddress = ":" + utils.GetEnv("ANALYTICS_SERVER_PORT", "8092")

	app, err := analyticsapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize Analytics Service")
	}

	if err := app.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Analytics Service exited with error")
	}
}
