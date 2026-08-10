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

var allowExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

var allowMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

const maxSize = 5 << 20

func ValidateAndSaveFile(fileHeader *multipart.FileHeader, uploadDir string) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowExts[ext] {
		return "", errors.New("unsupported file extension")
	}

	if fileHeader.Size > maxSize {
		return "", errors.New("file is too large (max 5MB)")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("cannot open file")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", errors.New("cannot read file")
	}

	mimeType := http.DetectContentType(buffer)
	if !allowMimeTypes[mimeType] {
		return "", fmt.Errorf("invalid MIME type: %s", mimeType)
	}

	//Change file name
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	if err := os.MkdirAll("./uploads", os.ModePerm); err != nil {
		return "", errors.New("cannot create uploads folder")
	}

	savePath := filepath.Join(uploadDir, fileName)
	if err := SaveFile(fileHeader, savePath); err != nil {
		return "", err
	}

	return fileName, nil
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
