package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"goshop/pkg/middleware"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorageService struct {
	uploadFunc  func(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error)
	presignFunc func(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error)
	statFunc    func(ctx context.Context, fileKey string) (int64, string, error)
	rangeFunc   func(ctx context.Context, fileKey string, n int64) (io.ReadCloser, error)
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

func (m *mockStorageService) Stat(ctx context.Context, fileKey string) (int64, string, error) {
	if m.statFunc != nil {
		return m.statFunc(ctx, fileKey)
	}
	return 1024, "image/jpeg", nil
}

func (m *mockStorageService) PublicURL(fileKey string) string {
	return "https://cdn.example.com/" + fileKey
}

func (m *mockStorageService) RangeReader(ctx context.Context, fileKey string, n int64) (io.ReadCloser, error) {
	if m.rangeFunc != nil {
		return m.rangeFunc(ctx, fileKey, n)
	}
	return io.NopCloser(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})), nil
}

func (m *mockStorageService) Open(ctx context.Context, fileKey string) (io.ReadCloser, error) {
	if m.rangeFunc != nil {
		return m.rangeFunc(ctx, fileKey, 0)
	}
	return io.NopCloser(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})), nil
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

func TestUploadHandler_GetPresignedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Case 1: Generate valid presigned URL", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage, nil)

		r := gin.New()
		r.GET("/api/v1/upload/presign", func(c *gin.Context) {
			c.Set("user_role", middleware.RoleAdmin)
			handler.GetPresignedURL(c)
		})

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

	t.Run("Case 1b: Non-admin cannot presign into an admin-only folder", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{}, nil)

		r := gin.New()
		r.GET("/api/v1/upload/presign", func(c *gin.Context) {
			c.Set("user_role", middleware.RoleMember)
			handler.GetPresignedURL(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=products&filename=item.png&content_type=image/png", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Case 1c: Unknown folder is rejected instead of silently defaulting", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{}, nil)

		r := gin.New()
		r.GET("/api/v1/upload/presign", handler.GetPresignedURL)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=secrets&filename=item.png&content_type=image/png", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 2: Missing filename query param returns BadRequest", func(t *testing.T) {
		mockStorage := &mockStorageService{}
		handler := NewUploadHandler(mockStorage, nil)

		r := gin.New()
		r.GET("/api/v1/upload/presign", handler.GetPresignedURL)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=chat", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 3: Content type that contradicts the extension is rejected", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{}, nil)

		r := gin.New()
		r.GET("/api/v1/upload/presign", handler.GetPresignedURL)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/presign?folder=chat&filename=clip.mp4&content_type=text/html", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestUploadHandler_ConfirmUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Case 1: Existing object is confirmed with its verified URL", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{
			statFunc: func(_ context.Context, _ string) (int64, string, error) {
				return 2048, "image/jpeg", nil
			},
		}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "chat/abc.jpg"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data := resp["data"].(map[string]any)
		assert.Equal(t, "chat/abc.jpg", data["file_key"])
		assert.Equal(t, "https://cdn.example.com/chat/abc.jpg", data["public_url"])
		assert.EqualValues(t, 2048, data["size"])
	})

	t.Run("Case 2: Missing file in storage returns 404", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{
			statFunc: func(_ context.Context, _ string) (int64, string, error) {
				return 0, "", errors.New("NotFound")
			},
		}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "chat/missing.jpg"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Case 3: Oversized object is rejected", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{
			statFunc: func(_ context.Context, _ string) (int64, string, error) {
				return 500 << 20, "image/jpeg", nil
			},
		}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "chat/huge.jpg"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 4: Path traversal in file_key is rejected", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "../../etc/passwd.jpg"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 5: Polyglot text renamed to .jpg is rejected by magic-byte sniff", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{
			statFunc: func(_ context.Context, _ string) (int64, string, error) {
				return 128, "image/jpeg", nil
			},
			rangeFunc: func(_ context.Context, _ string, _ int64) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("<script>alert('xss')</script>")), nil
			},
		}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "chat/xss.jpg"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Case 6: Video whose magic bytes are later in the file is accepted", func(t *testing.T) {
		handler := NewUploadHandler(&mockStorageService{
			statFunc: func(_ context.Context, _ string) (int64, string, error) {
				return 9 << 20, "video/mp4", nil
			},
			rangeFunc: func(_ context.Context, _ string, _ int64) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(make([]byte, 512))), nil
			},
		}, nil)

		r := gin.New()
		r.POST("/api/v1/upload/confirm", handler.ConfirmUpload)

		body, _ := json.Marshal(map[string]string{"file_key": "chat/clip.mp4"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
