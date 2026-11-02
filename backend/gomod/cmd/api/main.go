package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	app "goshop/cmd/api/app"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
)

// @title           GOshop E-Commerce API
// @version         1.0
// @description     High-performance E-Commerce platform API built with Go, Gin, PostgreSQL, Redis, Kafka, GHN, and VNPay.
// @termsOfService  http://swagger.io/terms/

// @contact.name   GOshop API Support
// @contact.url    http://www.goshop.vn/support
// @contact.email  support@goshop.vn

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-KEY
// @description Client API Key for application authorization.

func main() {

	rootDir := utils.MustGetwd()

	logFile := utils.RuntimeLogPath("api", "app.log")
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
		logger.Log.Info().Msg("Loaded .env successfully in API!")
	}

	shutdownTracer, tracerErr := telemetry.InitTracer(context.Background(), "goshop-api", "1.0.0")
	if tracerErr != nil {
		logger.Log.Warn().Err(tracerErr).Msg("Tracing disabled")
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	cfg := config.NewConfig()

	application, err := app.NewApplication(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create application")
	}

	if err := application.Run(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Application failed to run")
	}
}
