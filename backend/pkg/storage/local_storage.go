package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type localStorageService struct {
	baseUploadDir string
	publicBaseURL string
	logger        *zerolog.Logger
}

// NewLocalStorageService initializes a local disk storage provider.
func NewLocalStorageService(baseUploadDir, publicBaseURL string, logger *zerolog.Logger) StorageService {
	if baseUploadDir == "" {
		baseUploadDir = "./uploads"
	}
	_ = os.MkdirAll(baseUploadDir, os.ModePerm)

	return &localStorageService{
		baseUploadDir: baseUploadDir,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		logger:        logger,
	}
}

func (l *localStorageService) Upload(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error) {
	cleanFolder := strings.Trim(strings.TrimSpace(folder), "/")
	cleanFilename := strings.Trim(strings.TrimSpace(filename), "/")

	targetDir := filepath.Join(l.baseUploadDir, cleanFolder)
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return "", "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	targetPath := filepath.Join(targetDir, cleanFilename)
	out, err := os.Create(targetPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, reader); err != nil {
		return "", "", fmt.Errorf("failed to save local file: %w", err)
	}

	fileKey := filepath.ToSlash(filepath.Join(cleanFolder, cleanFilename))
	publicURL := fmt.Sprintf("/uploads/%s", fileKey)
	if l.publicBaseURL != "" {
		publicURL = fmt.Sprintf("%s/uploads/%s", l.publicBaseURL, fileKey)
	}

	return publicURL, fileKey, nil
}

func (l *localStorageService) Delete(ctx context.Context, fileKey string) error {
	clean := filepath.Clean(fileKey)
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimPrefix(clean, "\\")

	targetPath := filepath.Join(l.baseUploadDir, clean)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		if l.logger != nil {
			l.logger.Warn().Err(err).Str("filepath", targetPath).Msg("Failed to delete local file")
		}
		return err
	}
	return nil
}

func (l *localStorageService) GeneratePresignedUploadURL(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error) {
	// ponytail: local disk does not support S3 presigned PUT, direct upload fallback
	cleanFolder := strings.Trim(strings.TrimSpace(folder), "/")
	cleanFilename := strings.Trim(strings.TrimSpace(filename), "/")
	fileKey := filepath.ToSlash(filepath.Join(cleanFolder, cleanFilename))
	publicURL := fmt.Sprintf("/uploads/%s", fileKey)
	return "/api/v1/upload", publicURL, fileKey, nil
}

func (l *localStorageService) IsHealthy(ctx context.Context) bool {
	info, err := os.Stat(l.baseUploadDir)
	return err == nil && info.IsDir()
}
