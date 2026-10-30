package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	inventoryapp "goshop/services/inventory/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Inventory Service API
// @version         1.0
// @description     Dedicated Inventory microservice providing stock deduction, restoration, concurrency controls, and gRPC contracts.
// @host            localhost:8084
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("inventory", "inventory.log")
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
		logger.Log.Warn().Err(err).Msg("âš ï¸ No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Inventory Service!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-inventory", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()
	if inventoryDBName := utils.GetEnv("INVENTORY_DB_NAME", utils.GetEnv("DB_INVENTORY_NAME", "shopping_inventory")); inventoryDBName != "" {
		cfg.DB.DBName = inventoryDBName
	}
	port := utils.GetEnv("INVENTORY_SERVER_PORT", "8090")
	cfg.ServerAddress = ":" + port

	application, err := inventoryapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create inventory application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Inventory application failed to run")
	}
}
