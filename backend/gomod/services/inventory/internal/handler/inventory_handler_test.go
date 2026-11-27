package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inventorydto "goshop/services/inventory/internal/dto"
	inventoryhandler "goshop/services/inventory/internal/handler"
	inventoryservice "goshop/services/inventory/internal/service"
)

type mockInventoryService struct {
	inventoryservice.InventoryService
	listMovementsFn func(ctx context.Context, params inventorydto.GetStockMovementsParam) ([]inventorydto.StockMovementDTO, int64, error)
	adjustStockFn   func(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input inventorydto.AdjustStockInput) (*inventorydto.StockMovementDTO, error)
	getStockFn      func(ctx context.Context, productUUIDs []string, productIDs []int32) ([]inventorydto.ItemStockDTO, error)
}

func (m *mockInventoryService) ListStockMovements(ctx context.Context, params inventorydto.GetStockMovementsParam) ([]inventorydto.StockMovementDTO, int64, error) {
	if m.listMovementsFn != nil {
		return m.listMovementsFn(ctx, params)
	}
	return []inventorydto.StockMovementDTO{
		{
			MovementUUID:   uuid.New().String(),
			ProductUUID:    uuid.New().String(),
			ChangeType:     "RESTOCK",
			QuantityChange: 10,
			StockBefore:    0,
			StockAfter:     10,
			CreatedAt:      time.Now(),
		},
	}, 1, nil
}

func (m *mockInventoryService) AdjustStock(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input inventorydto.AdjustStockInput) (*inventorydto.StockMovementDTO, error) {
	if m.adjustStockFn != nil {
		return m.adjustStockFn(ctx, actorUUID, actorName, input)
	}
	return &inventorydto.StockMovementDTO{
		MovementUUID:   uuid.New().String(),
		ProductUUID:    input.ProductUUID,
		ChangeType:     input.ChangeType,
		QuantityChange: input.QuantityChange,
		StockBefore:    5,
		StockAfter:     5 + input.QuantityChange,
		CreatedAt:      time.Now(),
	}, nil
}

func (m *mockInventoryService) GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]inventorydto.ItemStockDTO, error) {
	if m.getStockFn != nil {
		return m.getStockFn(ctx, productUUIDs, productIDs)
	}
	res := make([]inventorydto.ItemStockDTO, 0, len(productUUIDs))
	for _, u := range productUUIDs {
		res = append(res, inventorydto.ItemStockDTO{
			ProductUUID:       u,
			Quantity:          50,
			ReservedQuantity:  10,
			AvailableQuantity: 40,
		})
	}
	return res, nil
}

func setupInventoryRouter(h *inventoryhandler.InventoryHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/inventory/movements", h.GetStockMovements)
	r.POST("/admin/inventory/adjust", h.AdjustStock)
	r.GET("/api/v1/inventory/stock", h.GetStock)
	return r
}

func TestInventoryHandler_GetStockMovements(t *testing.T) {
	mockSvc := &mockInventoryService{}
	r := setupInventoryRouter(inventoryhandler.NewInventoryHandler(mockSvc))

	t.Run("default pagination returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/inventory/movements", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Stock movements retrieved")
		assert.Contains(t, w.Body.String(), "RESTOCK")
	})

	t.Run("with custom query params returns 200", func(t *testing.T) {
		pUUID := uuid.New().String()
		req := httptest.NewRequest(http.MethodGet, "/admin/inventory/movements?page=2&limit=20&product_uuid="+pUUID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("service error returns error response", func(t *testing.T) {
		errSvc := &mockInventoryService{
			listMovementsFn: func(ctx context.Context, params inventorydto.GetStockMovementsParam) ([]inventorydto.StockMovementDTO, int64, error) {
				return nil, 0, errors.New("database failed")
			},
		}
		errRouter := setupInventoryRouter(inventoryhandler.NewInventoryHandler(errSvc))
		req := httptest.NewRequest(http.MethodGet, "/admin/inventory/movements", nil)
		w := httptest.NewRecorder()
		errRouter.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

func TestInventoryHandler_AdjustStock(t *testing.T) {
	mockSvc := &mockInventoryService{}
	r := setupInventoryRouter(inventoryhandler.NewInventoryHandler(mockSvc))

	t.Run("valid input returns 200", func(t *testing.T) {
		pUUID := uuid.New().String()
		input := inventorydto.AdjustStockInput{
			ProductUUID:    pUUID,
			QuantityChange: 15,
			ChangeType:     "RESTOCK",
		}
		raw, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/inventory/adjust", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Stock adjusted successfully")
	})

	t.Run("invalid change_type returns 400 validation error", func(t *testing.T) {
		raw := []byte(`{"product_uuid":"` + uuid.New().String() + `","quantity_change":5,"change_type":"INVALID_TYPE"}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/inventory/adjust", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service error returns error response", func(t *testing.T) {
		errSvc := &mockInventoryService{
			adjustStockFn: func(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input inventorydto.AdjustStockInput) (*inventorydto.StockMovementDTO, error) {
				return nil, errors.New("cannot adjust stock")
			},
		}
		errRouter := setupInventoryRouter(inventoryhandler.NewInventoryHandler(errSvc))
		input := inventorydto.AdjustStockInput{
			ProductUUID:    uuid.New().String(),
			QuantityChange: 5,
			ChangeType:     "MANUAL_ADJUSTMENT",
		}
		raw, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/inventory/adjust", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		errRouter.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

func TestInventoryHandler_GetStock(t *testing.T) {
	mockSvc := &mockInventoryService{}
	r := setupInventoryRouter(inventoryhandler.NewInventoryHandler(mockSvc))

	t.Run("valid product_uuids returns 200", func(t *testing.T) {
		u1 := uuid.New().String()
		u2 := uuid.New().String()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stock?product_uuids="+u1+"&product_uuids="+u2, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Stock retrieved successfully")
	})

	t.Run("service failure returns error response", func(t *testing.T) {
		errSvc := &mockInventoryService{
			getStockFn: func(ctx context.Context, productUUIDs []string, productIDs []int32) ([]inventorydto.ItemStockDTO, error) {
				return nil, errors.New("stock lookup error")
			},
		}
		errRouter := setupInventoryRouter(inventoryhandler.NewInventoryHandler(errSvc))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stock?product_uuids="+uuid.New().String(), nil)
		w := httptest.NewRecorder()
		errRouter.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}
