package v1handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
		"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorageService struct {
	uploadFunc func(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error)
	presignFunc func(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error)
}

func (m *mockStorageService) Upload(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error) {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, folder, filename, reader, size, contentType)
	}
	return "https://cdn.example.com/" + folder + "/" + filename, folder + "/" + filename, nil
}

func (m *mockStorageService) Delete(ctx context.Context, fileKey string) error {
	return nil
}

func (m *mockStorageService) GeneratePresignedUploadURL(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error) {
	if m.presignFunc != nil {
		return m.presignFunc(ctx, folder, filename, contentType, expire)
	}
	return "https://s3.amazonaws.com/presigned-upload-url", "https://cdn.example.com/" + folder + "/" + filename, folder + "/" + filename, nil
}

func (m *mockStorageService) IsHealthy(ctx context.Context) bool {
	return true
}

func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte, extraFields map[string]string) (*http.Request, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)

	for key, val := range extraFields {
		_ = writer.WriteField(key, val)
	}

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

func TestUploadHandler_UploadSingleFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Valid sample 1x1 JPEG image bytes
	validJPEG := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x01, 0x00, 0x48,
		0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0xFF, 0xD9,
	}

	t.Run("Case 1: Upload valid image to common folder", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.POST("/api/v1/upload", handler.UploadSingleFile)

		req, _ := createMultipartRequest(t, "file", "avatar.jpg", validJPEG, map[string]string{"folder": "common"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp["status"])
	})

	t.Run("Case 2: Upload missing file field returns BadRequest", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.POST("/api/v1/upload", handler.UploadSingleFile)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 3: Upload unsupported file extension returns BadRequest", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.POST("/api/v1/upload", handler.UploadSingleFile)

		req, _ := createMultipartRequest(t, "file", "malicious.exe", []byte("bad content"), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 4: Storage failure returns InternalServerError", func(t *testing.T) {
		mockStorage := &mockStorageService{
			uploadFunc: func(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error) {
				return "", "", errors.New("s3 connection timeout")
			},
		}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.POST("/api/v1/upload", handler.UploadSingleFile)

		req, _ := createMultipartRequest(t, "file", "photo.jpg", validJPEG, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUploadHandler_GetPresignedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Case 1: Generate valid presigned URL", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.GET("/api/v1/upload/presign", handler.GetPresignedURL)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=products&filename=item.png&content_type=image/png", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		data := resp["data"].(map[string]any)
		assert.NotEmpty(t, data["upload_url"])
		assert.NotEmpty(t, data["public_url"])
		assert.NotEmpty(t, data["file_key"])
	})

	t.Run("Case 2: Missing filename query param returns BadRequest", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage)

		r := gin.New()
		r.GET("/api/v1/upload/presign", handler.GetPresignedURL)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=products", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

