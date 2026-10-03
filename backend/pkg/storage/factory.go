package storage

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
)

// StorageConfig encapsulates all configuration options for storage providers.
type StorageConfig struct {
	Driver        StorageDriver
	UploadDir     string
	PublicBaseURL string
	S3            S3Config
}

// NewStorageService creates and initializes the requested storage driver (Local, S3, MinIO, Cloudflare R2).
func NewStorageService(ctx context.Context, cfg StorageConfig, logger *zerolog.Logger) (StorageService, error) {
	driver := StorageDriver(strings.ToLower(string(cfg.Driver)))

	switch driver {
	case DriverS3, DriverMinIO, DriverR2:
		return NewS3StorageService(ctx, cfg.S3, logger)
	case DriverLocal:
		fallthrough
	default:
		return NewLocalStorageService(cfg.UploadDir, cfg.PublicBaseURL, logger), nil
	}
}
