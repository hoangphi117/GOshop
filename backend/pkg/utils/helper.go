package utils

import (
	"crypto/rand"
	"encoding/base64"
	"goshop/pkg/logger"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/rs/zerolog"
	"golang.org/x/text/unicode/norm"
)

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(defaultValue)
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

func GenerateRandomString(length int) (string, error) {
	nonce := make([]byte, length)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(nonce), nil
}

func MustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	return dir
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))

	slug = strings.NewReplacer(
		"đ", "d",
	).Replace(slug)

	slug = norm.NFD.String(slug)

	var builder strings.Builder

	for _, r := range slug {
		if unicode.Is(unicode.Mn, r) {
			continue
		}

		builder.WriteRune(r)
	}

	slug = builder.String()
	slug = nonAlphanumericRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	return slug
}

func MaskName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "***"
	}

	runes := []rune(trimmed)
	n := len(runes)
	if n == 1 {
		return string(runes[0]) + "***"
	}
	if n == 2 {
		return string(runes[0]) + "***" + string(runes[1])
	}

	return string(runes[0]) + "***" + string(runes[n-1])
}
