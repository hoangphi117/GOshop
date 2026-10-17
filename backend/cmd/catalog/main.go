package main

import (
	"context"
	"path/filepath"

	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	catalogapp "goshop/services/catalog/app"

	"github.com/joho/godotenv"
)

// @title           GOshop Catalog Service API
// @version         1.0
// @description     Dedicated Catalog microservice providing category & product management, stock enrichment, search sync, and gRPC contracts.
// @host            localhost:8086
// @BasePath        /

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/catalog.log")
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
		logger.Log.Warn().Err(err).Msg("⚠️ No .env found, using system environment variables")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in Catalog Service!")
	}

	cfg := config.NewConfig()
	if catalogDBName := utils.GetEnv("CATALOG_DB_NAME", utils.GetEnv("DB_CATALOG_NAME", "shopping_catalog")); catalogDBName != "" {
		cfg.DB.DBName = catalogDBName
	}
	port := utils.GetEnv("CATALOG_SERVER_PORT", "8086")
	cfg.ServerAddress = ":" + port

	application, err := catalogapp.New(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create catalog application")
	}

	if err := application.Run(context.Background()); err != nil {
		logger.Log.Fatal().Err(err).Msg("Catalog application failed to run")
	}
}
