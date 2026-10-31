package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// StorageConfig encapsulates all configuration options for storage providers.
type StorageConfig struct {
	Driver        StorageDriver
	PublicBaseURL string
	S3            S3Config
}

// NewStorageService creates and initializes the requested storage driver (S3, MinIO, Cloudflare R2).
// There is deliberately no local-disk driver: all uploads must land in object storage.
func NewStorageService(ctx context.Context, cfg StorageConfig, logger *zerolog.Logger) (StorageService, error) {
	driver := StorageDriver(strings.ToLower(string(cfg.Driver)))

	switch driver {
	case DriverS3, DriverMinIO, DriverR2:
		return NewS3StorageService(ctx, cfg.S3, logger)
	default:
		return nil, fmt.Errorf("unsupported STORAGE_DRIVER %q (supported: s3, minio, r2) — local disk uploads are disabled", cfg.Driver)
	}
}
