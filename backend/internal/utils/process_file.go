package utils

import (
	"errors"
	"fmt"
	"goshop/pkg/logger"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

var allowImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

var allowImageMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

var allowVideoExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mov":  true,
}

var allowVideoMimeTypes = map[string]bool{
	"video/mp4":       true,
	"video/webm":      true,
	"video/quicktime": true,
}

const (
	MaxImageSize = 5 << 20
	MaxVideoSize = 30 << 20
)

func ValidateAndSaveFile(fileHeader *multipart.FileHeader, uploadDir string) (string, MediaType, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	var mediaType MediaType
	var maxSize int64

	if allowImageExts[ext] {
		mediaType = MediaTypeImage
		maxSize = MaxImageSize
	} else if allowVideoExts[ext] {
		mediaType = MediaTypeVideo
		maxSize = MaxVideoSize
	} else {
		return "", "", errors.New("unsupported file type (allowed: jpg, jpeg, png, webp, mp4, webm, mov)")
	}

	if fileHeader.Size > maxSize {
		return "", "", fmt.Errorf("file is too large (max %dMB for %s)", maxSize/(1<<20), mediaType)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", "", errors.New("cannot open file")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", "", errors.New("cannot read file")
	}

	mimeType := http.DetectContentType(buffer[:n])
	if mediaType == MediaTypeImage {
		if !allowImageMimeTypes[mimeType] {
			return "", "", fmt.Errorf("invalid image MIME type: %s", mimeType)
		}
	} else if mediaType == MediaTypeVideo {
		if !allowVideoMimeTypes[mimeType] && mimeType != "application/octet-stream" {
			return "", "", fmt.Errorf("invalid video MIME type: %s", mimeType)
		}
	}

	// Rewind file pointer back to start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", errors.New("cannot rewind file")
	}

	// Change file name
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return "", "", errors.New("cannot create uploads folder")
	}

	savePath := filepath.Join(uploadDir, fileName)
	out, err := os.Create(savePath)
	if err != nil {
		return "", "", err
	}
	defer out.Close()

	if _, err = io.Copy(out, file); err != nil {
		return "", "", err
	}

	return fileName, mediaType, nil
}

func SaveFile(fileHeader *multipart.FileHeader, destination string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)

	return err
}

func DeleteUploadedFiles(filePaths ...string) {
	for _, path := range filePaths {
		if strings.TrimSpace(path) == "" {
			continue
		}

		// Path sanitization to prevent directory traversal attacks
		clean := filepath.Clean(path)
		clean = strings.TrimPrefix(clean, string(filepath.Separator))
		clean = strings.TrimPrefix(clean, "/")
		clean = strings.TrimPrefix(clean, "\\")

		// Ensure the file is within the "uploads" directory
		if !strings.HasPrefix(clean, "uploads"+string(filepath.Separator)) && !strings.HasPrefix(clean, "uploads/") {
			continue
		}

		if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
			logger.Log.Warn().Err(err).Str("filepath", clean).Msg("Failed to delete physical uploaded file")
		}
	}
}
