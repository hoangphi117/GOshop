package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goshop_storage_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	svc := NewLocalStorageService(tempDir, "http://localhost:8080", nil)
	assert.True(t, svc.IsHealthy(context.Background()))

	// 1. Test Upload
	content := []byte("hello storage test")
	reader := bytes.NewReader(content)
	publicURL, fileKey, err := svc.Upload(context.Background(), "products", "test.txt", reader, int64(len(content)), "text/plain")
	require.NoError(t, err)
	assert.Equal(t, "products/test.txt", fileKey)
	assert.Contains(t, publicURL, "/uploads/products/test.txt")

	// Verify file on disk
	diskPath := filepath.Join(tempDir, "products", "test.txt")
	savedContent, err := os.ReadFile(diskPath)
	require.NoError(t, err)
	assert.Equal(t, content, savedContent)

	// 2. Test Presigned Upload URL Fallback
	uploadURL, presignPublicURL, presignKey, err := svc.GeneratePresignedUploadURL(context.Background(), "chat", "image.png", "image/png", 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/upload", uploadURL)
	assert.Equal(t, "chat/image.png", presignKey)
	assert.Contains(t, presignPublicURL, "/uploads/chat/image.png")

	// 3. Test Delete
	err = svc.Delete(context.Background(), fileKey)
	require.NoError(t, err)
	_, err = os.Stat(diskPath)
	assert.True(t, os.IsNotExist(err))
}

func TestStorageFactory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goshop_factory_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Local Driver
	localSvc, err := NewStorageService(context.Background(), StorageConfig{
		Driver:    DriverLocal,
		UploadDir: tempDir,
	}, nil)
	require.NoError(t, err)
	assert.NotNil(t, localSvc)
	assert.True(t, localSvc.IsHealthy(context.Background()))
}
