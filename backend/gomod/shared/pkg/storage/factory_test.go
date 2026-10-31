package storage_test

import (
	"context"
	"testing"

	"goshop/shared/pkg/storage"

	"github.com/rs/zerolog"
)

// Local disk uploads are removed: any driver other than s3/minio/r2 must fail
// loudly at startup instead of silently falling back to disk.
func TestNewStorageService_RejectsNonObjectStorageDrivers(t *testing.T) {
	logger := zerolog.Nop()

	for _, driver := range []string{"", "local", "disk", "s4"} {
		_, err := storage.NewStorageService(context.Background(), storage.StorageConfig{
			Driver: storage.StorageDriver(driver),
		}, &logger)
		if err == nil {
			t.Fatalf("driver %q: expected error, got nil", driver)
		}
	}
}
