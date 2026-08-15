package utils

import (
	"goshop/pkg/logger"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
)

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func NewLoggerWithPath(fileName string, level string) *zerolog.Logger {

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("⛔ Failed to get current working directory: " + err.Error())
	}

	logDir := filepath.Join(cwd, "internal", "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logDir = filepath.Join(cwd, "..", "..", "internal", "logs")
	}

	path := filepath.Join(logDir, fileName)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create log directory")
	}

	config := logger.LoggerConfig{
		Level:      level,
		Filename:   path,
		MaxSize:    1,
		MaxBackups: 5,
		MaxAge:     5,
		Compress:   true,
		IsDev:      GetEnv("APP_ENV", "development"),
	}
	return logger.NewLogger(config)
}
