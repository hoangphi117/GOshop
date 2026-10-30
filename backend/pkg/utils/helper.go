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

// runtimeLogRoot walks up from cwd until it finds the backend module root
// (the dir holding go.mod), so every caller — cmd/<svc>, tests run from a
// package dir, the API — resolves the same .runtime/logs tree.
func runtimeLogRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("⛔ Failed to get current working directory: %v", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, ".runtime", "logs")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// No go.mod above us (e.g. running a built binary elsewhere):
			// fall back to cwd and let MkdirAll create the tree.
			return filepath.Join(cwd, ".runtime", "logs")
		}
	}
}

// RuntimeLogDir returns (and creates) .runtime/logs/<service>.
func RuntimeLogDir(service string) string {
	dir := runtimeLogRoot()
	if service != "" {
		dir = filepath.Join(dir, service)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Log.Fatal().Err(err).Str("dir", dir).Msg("Failed to create log directory")
	}
	return dir
}

// RuntimeLogPath returns .runtime/logs/<service>/<fileName>, creating the dir.
func RuntimeLogPath(service, fileName string) string {
	return filepath.Join(RuntimeLogDir(service), fileName)
}

// NewLoggerWithPath logs to .runtime/logs/<service>/<fileName>. Pass "" as
// the service for shared root files. SERVICE_NAME names the bucket when the
// container supplies it; a host run falls back to the binary name (cmd/<svc>).
func NewLoggerWithPath(service, fileName string, level string) *zerolog.Logger {
	path := RuntimeLogPath(service, fileName)

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

// ServiceName is the log bucket for this process: SERVICE_NAME when the
// container supplies it, else the binary name (go run / go build ./cmd/x).
func ServiceName() string {
	if svc := GetEnv("SERVICE_NAME", ""); svc != "" {
		return svc
	}
	if exe, err := os.Executable(); err == nil {
		return strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	return ""
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
