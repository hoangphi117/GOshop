package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goshop/shared/pkg/discord"
	"goshop/shared/pkg/messaging"
)

type MockRabbitMQ struct {
	mock.Mock
}

func (m *MockRabbitMQ) Publish(ctx context.Context, queue string, message any) error {
	return nil
}

func (m *MockRabbitMQ) Consume(ctx context.Context, queue string, handler messaging.Handler) error {
	return nil
}

func (m *MockRabbitMQ) PublishTopic(ctx context.Context, routingKey string, message any) error {
	return nil
}

func (m *MockRabbitMQ) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	return nil
}

func (m *MockRabbitMQ) SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler messaging.Handler) error {
	return nil
}

func (m *MockRabbitMQ) SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler messaging.Handler) error {
	return nil
}

func (m *MockRabbitMQ) Close() error {
	return nil
}

func (m *MockRabbitMQ) IsHealthy() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Upload(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error) {
	return "", "", nil
}

func (m *MockStorage) Delete(ctx context.Context, fileKey string) error {
	return nil
}

func (m *MockStorage) GeneratePresignedUploadURL(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error) {
	return "", "", "", nil
}

func (m *MockStorage) Stat(ctx context.Context, fileKey string) (int64, string, error) {
	return 0, "", nil
}

func (m *MockStorage) PublicURL(fileKey string) string {
	return ""
}

func (m *MockStorage) RangeReader(ctx context.Context, fileKey string, n int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (m *MockStorage) Open(ctx context.Context, fileKey string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (m *MockStorage) IsHealthy(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func TestHealthHandler_LivenessCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewHealthHandler(nil, nil, nil, nil, nil, nil)
	r.GET("/healthz/live", handler.LivenessCheck)

	req, _ := http.NewRequest(http.MethodGet, "/healthz/live", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "alive", resp["status"])
}

func TestHealthHandler_HealthCheck_AllHealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	mockRabbit := new(MockRabbitMQ)
	mockRabbit.On("IsHealthy").Return(true)

	mockStore := new(MockStorage)
	mockStore.On("IsHealthy", mock.Anything).Return(true)

	handler := NewHealthHandler(nil, nil, mockRabbit, mockStore, discord.NewDiscordNotifier("", nil), nil)
	r.GET("/healthz", handler.HealthCheck)

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])

	services := resp["services"].(map[string]any)
	assert.Equal(t, "up", services["rabbitmq"])
	assert.Equal(t, "up", services["storage"])
}

func TestHealthHandler_HealthCheck_Degraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	mockRabbit := new(MockRabbitMQ)
	mockRabbit.On("IsHealthy").Return(false) // Down

	mockStore := new(MockStorage)
	mockStore.On("IsHealthy", mock.Anything).Return(true)

	handler := NewHealthHandler(nil, nil, mockRabbit, mockStore, discord.NewDiscordNotifier("", nil), nil)
	r.GET("/healthz", handler.HealthCheck)

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "degraded", resp["status"])

	services := resp["services"].(map[string]any)
	assert.Equal(t, "down", services["rabbitmq"])
}

func TestHealthHandler_ReadinessCheck_DoesNotSendAlert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	mockRabbit := new(MockRabbitMQ)
	mockRabbit.On("IsHealthy").Return(false)
	mockStore := new(MockStorage)
	mockStore.On("IsHealthy", mock.Anything).Return(true)

	handler := NewHealthHandler(nil, nil, mockRabbit, mockStore, nil, nil)
	r.GET("/healthz/ready", handler.ReadinessCheck)

	req, _ := http.NewRequest(http.MethodGet, "/healthz/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	mockRabbit.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
