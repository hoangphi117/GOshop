package storage

import (
	"context"
	"io"
	"time"
)

// StorageDriver represents the underlying storage provider type.
type StorageDriver string

const (
	DriverLocal StorageDriver = "local"
	DriverS3    StorageDriver = "s3"
	DriverMinIO StorageDriver = "minio"
	DriverR2    StorageDriver = "r2"
)

// StorageService defines the contract for object storage operations across providers (Local, S3, MinIO, Cloudflare R2).
type StorageService interface {
	// Upload streams a file to the storage provider and returns its publicly accessible URL.
	Upload(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (publicURL string, fileKey string, err error)

	// Delete removes a file by its object key from the storage provider.
	Delete(ctx context.Context, fileKey string) error

	// GeneratePresignedUploadURL generates a direct PUT URL allowing the client to upload directly to S3/R2.
	GeneratePresignedUploadURL(ctx context.Context, folder, filename, contentType string, expire time.Duration) (uploadURL string, publicURL string, fileKey string, err error)

	// IsHealthy verifies connectivity with the storage service.
	IsHealthy(ctx context.Context) bool
}
