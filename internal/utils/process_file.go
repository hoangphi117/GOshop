package utils

import (
	"errors"
	"fmt"
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
	var allowMimes map[string]bool

	if allowImageExts[ext] {
		mediaType = MediaTypeImage
		maxSize = MaxImageSize
		allowMimes = allowImageMimeTypes
	} else if allowVideoExts[ext] {
		mediaType = MediaTypeVideo
		maxSize = MaxVideoSize
		allowMimes = allowVideoMimeTypes
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
	_, err = file.Read(buffer)
	if err != nil {
		return "", "", errors.New("cannot read file")
	}

	mimeType := http.DetectContentType(buffer)
	if !allowMimes[mimeType] && mimeType != "application/octet-stream" {
		return "", "", fmt.Errorf("invalid MIME type: %s", mimeType)
	}

	//Change file name
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return "", "", errors.New("cannot create uploads folder")
	}

	savePath := filepath.Join(uploadDir, fileName)
	if err := SaveFile(fileHeader, savePath); err != nil {
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
