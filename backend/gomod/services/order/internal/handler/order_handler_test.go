package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1dto "goshop/services/order/internal/dto"
	orderhandler "goshop/services/order/internal/handler"
	orderservice "goshop/services/order/internal/service"
	"goshop/shared/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCustomerOrderService struct {
	orderservice.CustomerOrderService
	createOrderFunc func(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error)
}

func (m *mockCustomerOrderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, userUUID, input)
	}
	return &v1dto.OrderDTO{}, nil
}

func TestOrderHandler_CreateOrder_WithIdempotencyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userUUID := uuid.New()
	var capturedInput v1dto.CreateOrderInput
	var capturedUserUUID uuid.UUID

	mockCustomer := &mockCustomerOrderService{
		createOrderFunc: func(ctx context.Context, u uuid.UUID, in v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
			capturedUserUUID = u
			capturedInput = in
			return &v1dto.OrderDTO{
				UUID:        uuid.New().String(),
				OrderNumber: "261018-01-OK",
				TotalAmount: 150.0,
			}, nil
		},
	}

	handler := orderhandler.NewOrderHandler(mockCustomer, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", userUUID.String())
		c.Set("user_id", int32(101))
		c.Next()
	})
	r.POST("/orders", handler.CreateOrder)

	reqBody := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: uuid.New().String(), Quantity: 1},
		},
		ShippingName:    "Alice",
		ShippingPhone:   "0987654321",
		ShippingAddress: "456 Avenue",
		ShippingFee:     10.0,
		PaymentMethod:   1,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Send with X-Idempotency-Key
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "test-header-key-999")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, userUUID, capturedUserUUID)
	assert.Equal(t, "test-header-key-999", capturedInput.IdempotencyKey)
}

func TestOrderHandler_CreateOrder_Conflict_409(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userUUID := uuid.New()
	mockCustomer := &mockCustomerOrderService{
		createOrderFunc: func(ctx context.Context, u uuid.UUID, in v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
			return nil, utils.NewError(utils.ErrCodeConflict, "A checkout request with this idempotency key is already in progress")
		},
	}

	handler := orderhandler.NewOrderHandler(mockCustomer, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", userUUID.String())
		c.Next()
	})
	r.POST("/orders", handler.CreateOrder)

	reqBody := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: uuid.New().String(), Quantity: 1},
		},
		ShippingName:    "Alice",
		ShippingPhone:   "0987654321",
		ShippingAddress: "456 Avenue",
		ShippingFee:     10.0,
		PaymentMethod:   1,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "test-conflict-key")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}
