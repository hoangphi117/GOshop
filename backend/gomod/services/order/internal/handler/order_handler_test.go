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
	getMyOrdersFunc func(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	getByUUIDFunc   func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error)
	cancelFunc      func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error)
	updateShipFunc  func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error)
	updatePayFunc   func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error)
}

func (m *mockCustomerOrderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, userUUID, input)
	}
	return &v1dto.OrderDTO{UUID: uuid.New().String(), OrderNumber: "ORD-001"}, nil
}

func (m *mockCustomerOrderService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if m.getMyOrdersFunc != nil {
		return m.getMyOrdersFunc(ctx, userUUID, input)
	}
	return []v1dto.OrderSummaryDTO{{UUID: uuid.New().String(), OrderNumber: "ORD-001"}}, 1, nil
}

func (m *mockCustomerOrderService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(ctx, userUUID, orderUUID)
	}
	return &v1dto.OrderDTO{UUID: orderUUID.String(), OrderNumber: "ORD-001"}, nil
}

func (m *mockCustomerOrderService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error) {
	if m.cancelFunc != nil {
		return m.cancelFunc(ctx, userUUID, orderUUID, reason)
	}
	return &v1dto.OrderSummaryDTO{UUID: orderUUID.String(), Status: "Cancelled"}, nil
}

func (m *mockCustomerOrderService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error) {
	if m.updateShipFunc != nil {
		return m.updateShipFunc(ctx, userUUID, orderUUID, input)
	}
	return &v1dto.OrderDTO{UUID: orderUUID.String()}, nil
}

func (m *mockCustomerOrderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
	if m.updatePayFunc != nil {
		return m.updatePayFunc(ctx, userUUID, orderUUID, input)
	}
	return &v1dto.OrderDTO{UUID: orderUUID.String()}, nil
}

type mockAdminOrderService struct {
	orderservice.OrderAdminService
	getAllFunc   func(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	updateStatus func(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error)
	bulkStatus   func(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	bulkCancel   func(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

func (m *mockAdminOrderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx, input)
	}
	return []v1dto.OrderSummaryDTO{{UUID: uuid.New().String()}}, 1, nil
}

func (m *mockAdminOrderService) UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error) {
	if m.updateStatus != nil {
		return m.updateStatus(ctx, orderUUID, input)
	}
	return &v1dto.OrderDTO{UUID: orderUUID.String(), Status: "Processing"}, nil
}

func (m *mockAdminOrderService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.bulkStatus != nil {
		return m.bulkStatus(ctx, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

func (m *mockAdminOrderService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.bulkCancel != nil {
		return m.bulkCancel(ctx, userUUID, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

type mockInvoiceService struct {
	orderservice.OrderInvoiceService
	getPDFFunc func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
}

func (m *mockInvoiceService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	if m.getPDFFunc != nil {
		return m.getPDFFunc(ctx, userUUID, orderUUID)
	}
	return []byte("%PDF-1.4 test invoice"), "invoice-123.pdf", nil
}

type mockLocationService struct {
	orderservice.LocationService
	provincesFunc func(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	districtsFunc func(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	wardsFunc     func(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)
}

func (m *mockLocationService) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	if m.provincesFunc != nil {
		return m.provincesFunc(ctx)
	}
	return []v1dto.ProvinceDTO{{ProvinceID: 1, ProvinceName: "Hanoi"}}, nil
}

func (m *mockLocationService) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	if m.districtsFunc != nil {
		return m.districtsFunc(ctx, provinceID)
	}
	return []v1dto.DistrictDTO{{DistrictID: 10, DistrictName: "Ba Dinh"}}, nil
}

func (m *mockLocationService) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	if m.wardsFunc != nil {
		return m.wardsFunc(ctx, districtID)
	}
	return []v1dto.WardDTO{{WardCode: "1001", WardName: "Phuc Xa"}}, nil
}

type mockShippingService struct {
	orderservice.OrderShippingService
	createShipFn   func(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error)
	timelineFn     func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error)
	calcFeeFunc    func(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error)
	confirmFunc    func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error)
	cancelShipFn   func(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error)
	printBillFn    func(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error)
	syncTrackingFn func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error)
	adminSyncFn    func(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error)
}

func (m *mockShippingService) SyncOrderTracking(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error) {
	if m.syncTrackingFn != nil {
		return m.syncTrackingFn(ctx, userUUID, orderUUID)
	}
	return v1dto.SyncTrackingResponseDTO{OrderUUID: orderUUID.String(), ShippingCode: "GHN123"}, nil
}

func (m *mockShippingService) AdminSyncActiveShippingOrders(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error) {
	if m.adminSyncFn != nil {
		return m.adminSyncFn(ctx, userUUID)
	}
	return v1dto.SyncActiveShippingStatsDTO{ScannedCount: 2, SyncedCount: 2}, nil
}

func (m *mockShippingService) CreateShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error) {
	if m.createShipFn != nil {
		return m.createShipFn(ctx, orderUUID, input)
	}
	return v1dto.AsyncShippingResponseDTO{OrderUUID: orderUUID.String(), Status: "processing", Message: "queued"}, nil
}

func (m *mockShippingService) GetOrderTimeline(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error) {
	if m.timelineFn != nil {
		return m.timelineFn(ctx, userUUID, orderUUID)
	}
	return v1dto.OrderTimelineResponseDTO{OrderUUID: orderUUID.String(), OrderNumber: "ORD-001"}, nil
}

func (m *mockShippingService) CalculateShippingFee(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error) {
	if m.calcFeeFunc != nil {
		return m.calcFeeFunc(ctx, input)
	}
	return v1dto.ShippingFeeResponseDTO{Provider: input.Provider, ShippingFee: 25000}, nil
}

func (m *mockShippingService) ConfirmOrderDelivery(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error) {
	if m.confirmFunc != nil {
		return m.confirmFunc(ctx, userUUID, orderUUID)
	}
	return v1dto.ConfirmDeliveryResponseDTO{OrderUUID: orderUUID.String(), OrderStatus: 4}, nil
}

func (m *mockShippingService) CancelShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error) {
	if m.cancelShipFn != nil {
		return m.cancelShipFn(ctx, orderUUID, input)
	}
	return v1dto.CancelShippingResponseDTO{OrderUUID: orderUUID.String(), Message: "cancelled"}, nil
}

func (m *mockShippingService) GetPrintBill(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error) {
	if m.printBillFn != nil {
		return m.printBillFn(ctx, orderUUID)
	}
	return v1dto.PrintBillResponseDTO{OrderUUID: orderUUID.String(), PrintA5: "https://example.com/print"}, nil
}

type mockOrderReturnService struct {
	orderservice.OrderReturnService
	getMyReturnsFn  func(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error)
	getDetailsFn    func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error)
	adminReturnsFn  func(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error)
	adminDetailFn   func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error)
	adminUpdateFn   func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error)
	cancelFn        func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error)
	submitShipFn    func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error)
	updateBankFn    func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error)
	processRefundFn func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error)
}

func (m *mockOrderReturnService) SubmitReturnShipping(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error) {
	if m.submitShipFn != nil {
		return m.submitShipFn(ctx, userUUID, orderUUID, input)
	}
	return v1dto.SubmitReturnShippingResponseDTO{ReturnUUID: uuid.New().String(), ShippingCode: input.ShippingCode}, nil
}

func (m *mockOrderReturnService) UpdateReturnBankAccount(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error) {
	if m.updateBankFn != nil {
		return m.updateBankFn(ctx, userUUID, orderUUID, input)
	}
	return v1dto.UpdateReturnBankAccountResponseDTO{ReturnUUID: uuid.New().String(), BankName: input.BankName}, nil
}

func (m *mockOrderReturnService) AdminProcessReturnRefund(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error) {
	if m.processRefundFn != nil {
		return m.processRefundFn(ctx, userUUID, returnUUID, input)
	}
	return v1dto.AdminReturnRefundResponseDTO{ReturnUUID: returnUUID.String(), RefundAmount: 100}, nil
}

func (m *mockOrderReturnService) GetMyOrderReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error) {
	if m.getMyReturnsFn != nil {
		return m.getMyReturnsFn(ctx, userUUID, query)
	}
	return []v1dto.UserReturnSummaryDTO{{ReturnUUID: uuid.New().String(), OrderNumber: "ORD-001"}}, 1, nil
}

func (m *mockOrderReturnService) GetOrderReturnDetails(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error) {
	if m.getDetailsFn != nil {
		return m.getDetailsFn(ctx, userUUID, orderUUID)
	}
	return v1dto.OrderReturnDetailDTO{ReturnUUID: uuid.New().String(), OrderUUID: orderUUID.String(), Reason: "Damaged"}, nil
}

func (m *mockOrderReturnService) AdminGetReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error) {
	if m.adminReturnsFn != nil {
		return m.adminReturnsFn(ctx, userUUID, query)
	}
	return []v1dto.AdminReturnSummaryDTO{{ReturnUUID: uuid.New().String(), OrderNumber: "ORD-001"}}, 1, nil
}

func (m *mockOrderReturnService) AdminGetReturnDetail(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error) {
	if m.adminDetailFn != nil {
		return m.adminDetailFn(ctx, userUUID, returnUUID)
	}
	return v1dto.AdminReturnDetailDTO{ReturnUUID: returnUUID.String(), Reason: "Defective"}, nil
}

func (m *mockOrderReturnService) AdminUpdateReturnStatus(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error) {
	if m.adminUpdateFn != nil {
		return m.adminUpdateFn(ctx, userUUID, returnUUID, input)
	}
	return v1dto.AdminReturnSummaryDTO{ReturnUUID: returnUUID.String()}, nil
}

func (m *mockOrderReturnService) CancelOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error) {
	if m.cancelFn != nil {
		return m.cancelFn(ctx, userUUID, orderUUID, input)
	}
	return v1dto.CancelOrderReturnResponseDTO{ReturnUUID: uuid.New().String(), Status: 5, Message: "cancelled"}, nil
}

func setupFullOrderRouter(h *orderhandler.OrderHandler, userUUID *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if userUUID != nil {
		r.Use(func(c *gin.Context) {
			c.Set("user_uuid", userUUID.String())
			c.Set("user_id", int32(101))
			c.Set("user_role", "customer")
			c.Next()
		})
	}

	// Customer routes
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders", h.GetMyOrders)
	r.GET("/orders/:uuid", h.GetOrderByUUID)
	r.PATCH("/orders/:uuid/cancel", h.CancelOrder)
	r.PUT("/orders/:uuid/shipping-info", h.UpdateOrderShippingInfo)
	r.PUT("/orders/:uuid/payment-status", h.UpdateOrderPaymentStatus)
	r.GET("/orders/:uuid/invoice", h.DownloadInvoice)
	r.POST("/orders/calculate-fee", h.CalculateShippingFee)
	r.POST("/orders/:uuid/confirm-delivery", h.ConfirmOrderDelivery)

	// Admin routes
	r.GET("/admin/orders", h.GetAllOrders)
	r.PATCH("/admin/orders/:uuid", h.UpdateOrderStatus)
	r.POST("/admin/orders/bulk-status", h.BulkUpdateOrderStatus)
	r.POST("/admin/orders/bulk-cancel", h.BulkCancelOrders)

	// Location routes
	r.GET("/shipping/provinces", h.GetProvinces)
	r.GET("/shipping/districts", h.GetDistricts)
	r.GET("/shipping/wards", h.GetWards)

	// Shipping management routes
	r.POST("/admin/orders/:uuid/shipping", h.CreateShipping)
	r.GET("/orders/:uuid/timeline", h.GetOrderTimeline)
	r.POST("/admin/orders/:uuid/shipping/cancel", h.CancelShipping)
	r.GET("/admin/orders/:uuid/shipping/print-bill", h.GetPrintBill)

	// Return routes
	r.GET("/orders/returns", h.GetMyOrderReturns)
	r.GET("/orders/:uuid/return", h.GetOrderReturnDetails)
	r.POST("/orders/:uuid/return/cancel", h.CancelOrderReturn)
	r.POST("/orders/:uuid/return/shipping", h.SubmitReturnShipping)
	r.PUT("/orders/:uuid/return/bank-account", h.UpdateReturnBankAccount)
	r.GET("/admin/returns", h.AdminGetReturns)
	r.GET("/admin/returns/:uuid", h.AdminGetReturnDetail)
	r.PATCH("/admin/returns/:uuid", h.AdminUpdateReturnStatus)
	r.POST("/admin/returns/:uuid/refund", h.AdminProcessReturnRefund)
	r.POST("/orders/:uuid/sync-tracking", h.SyncOrderTracking)
	r.POST("/admin/orders/sync-active-tracking", h.AdminSyncActiveTracking)

	return r
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	userUUID := uuid.New()
	mockCust := &mockCustomerOrderService{}
	h := orderhandler.NewOrderHandler(mockCust, nil, nil)
	r := setupFullOrderRouter(h, &userUUID)

	t.Run("success with idempotency key", func(t *testing.T) {
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
		req.Header.Set("X-Idempotency-Key", "idemp-key-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "Order created successfully")
	})

	t.Run("conflict 409", func(t *testing.T) {
		errCust := &mockCustomerOrderService{
			createOrderFunc: func(ctx context.Context, u uuid.UUID, in v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
				return nil, utils.NewError(utils.ErrCodeConflict, "checkout in progress")
			},
		}
		errH := orderhandler.NewOrderHandler(errCust, nil, nil)
		errR := setupFullOrderRouter(errH, &userUUID)

		reqBody := v1dto.CreateOrderInput{
			Items:           []v1dto.CreateOrderItemInput{{ProductUUID: uuid.New().String(), Quantity: 1}},
			ShippingName:    "Alice",
			ShippingPhone:   "0987654321",
			ShippingAddress: "456 Avenue",
			ShippingFee:     10.0,
			PaymentMethod:   1,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		errR.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte(`{"items": "invalid"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestOrderHandler_CustomerEndpoints(t *testing.T) {
	userUUID := uuid.New()
	mockCust := &mockCustomerOrderService{}
	mockInv := &mockInvoiceService{}
	mockShip := &mockShippingService{}
	h := orderhandler.NewOrderHandler(mockCust, mockShip, nil).WithInvoiceService(mockInv)
	r := setupFullOrderRouter(h, &userUUID)

	t.Run("GetMyOrders returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orders?page=1&limit=10", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Get my orders successfully")
	})

	t.Run("GetOrderByUUID valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodGet, "/orders/"+oUUID, nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/orders/not-a-uuid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CancelOrder valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		body := `{"reason":"changed mind"}`
		reqOK := httptest.NewRequest(http.MethodPatch, "/orders/"+oUUID+"/cancel", bytes.NewReader([]byte(body)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPatch, "/orders/invalid-uuid/cancel", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("UpdateOrderShippingInfo valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		name := "Bob"
		phone := "0912345678"
		addr := "123 Street"
		input := v1dto.UpdateOrderShippingInfoInput{
			ShippingName:    &name,
			ShippingPhone:   &phone,
			ShippingAddress: &addr,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPut, "/orders/"+oUUID+"/shipping-info", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPut, "/orders/bad-uuid/shipping-info", bytes.NewReader(raw))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("UpdateOrderPaymentStatus valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		input := v1dto.UpdateOrderPaymentStatusInput{
			PaymentStatus: 2,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPut, "/orders/"+oUUID+"/payment-status", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		badInput := `{"payment_status": 999}`
		reqBad := httptest.NewRequest(http.MethodPut, "/orders/"+oUUID+"/payment-status", bytes.NewReader([]byte(badInput)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("DownloadInvoice inline and download", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqInline := httptest.NewRequest(http.MethodGet, "/orders/"+oUUID+"/invoice", nil)
		wInline := httptest.NewRecorder()
		r.ServeHTTP(wInline, reqInline)
		assert.Equal(t, http.StatusOK, wInline.Code)
		assert.Equal(t, "application/pdf", wInline.Header().Get("Content-Type"))

		reqDown := httptest.NewRequest(http.MethodGet, "/orders/"+oUUID+"/invoice?action=download", nil)
		wDown := httptest.NewRecorder()
		r.ServeHTTP(wDown, reqDown)
		assert.Equal(t, http.StatusOK, wDown.Code)
		assert.Contains(t, wDown.Header().Get("Content-Disposition"), "attachment")
	})

	t.Run("ConfirmOrderDelivery valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodPost, "/orders/"+oUUID+"/confirm-delivery", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/orders/invalid-uuid/confirm-delivery", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})
}

func TestOrderHandler_AdminEndpoints(t *testing.T) {
	userUUID := uuid.New()
	mockAdmin := &mockAdminOrderService{}
	h := orderhandler.NewOrderHandler(nil, nil, nil).WithAdminService(mockAdmin)
	r := setupFullOrderRouter(h, &userUUID)

	t.Run("GetAllOrders returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/orders?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Get all orders successfully")
	})

	t.Run("UpdateOrderStatus valid and invalid status", func(t *testing.T) {
		oUUID := uuid.New().String()
		bodyOK := `{"status": 2}`
		reqOK := httptest.NewRequest(http.MethodPatch, "/admin/orders/"+oUUID, bytes.NewReader([]byte(bodyOK)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		bodyBad := `{"status": 99}`
		reqBad := httptest.NewRequest(http.MethodPatch, "/admin/orders/"+oUUID, bytes.NewReader([]byte(bodyBad)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("BulkUpdateOrderStatus valid and invalid", func(t *testing.T) {
		input := v1dto.BulkUpdateOrderStatusInput{
			OrderUUIDs: []string{uuid.New().String(), uuid.New().String()},
			Status:     3,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPost, "/admin/orders/bulk-status", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/admin/orders/bulk-status", bytes.NewReader([]byte(`{"order_uuids":[]}`)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("BulkCancelOrders valid and invalid", func(t *testing.T) {
		reason := "out of stock"
		input := v1dto.BulkCancelOrdersInput{
			OrderUUIDs: []string{uuid.New().String()},
			Reason:     &reason,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPost, "/admin/orders/bulk-cancel", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/admin/orders/bulk-cancel", bytes.NewReader([]byte(`{"order_uuids":[]}`)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})
}

func TestOrderHandler_LocationAndShipping(t *testing.T) {
	userUUID := uuid.New()
	mockLoc := &mockLocationService{}
	mockShip := &mockShippingService{}
	h := orderhandler.NewOrderHandler(nil, mockShip, nil).WithLocationService(mockLoc)
	r := setupFullOrderRouter(h, &userUUID)

	t.Run("GetProvinces returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/shipping/provinces", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Hanoi")
	})

	t.Run("GetDistricts valid and invalid", func(t *testing.T) {
		reqOK := httptest.NewRequest(http.MethodGet, "/shipping/districts?province_id=1", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)
		assert.Contains(t, wOK.Body.String(), "Ba Dinh")

		reqBad := httptest.NewRequest(http.MethodGet, "/shipping/districts?province_id=abc", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("GetWards valid and invalid", func(t *testing.T) {
		reqOK := httptest.NewRequest(http.MethodGet, "/shipping/wards?district_id=10", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)
		assert.Contains(t, wOK.Body.String(), "Phuc Xa")

		reqBad := httptest.NewRequest(http.MethodGet, "/shipping/wards?district_id=invalid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CalculateShippingFee valid and invalid", func(t *testing.T) {
		input := v1dto.CalculateShippingFeeInput{
			Provider:   "ghn",
			DistrictID: 10,
			WardCode:   "1001",
			Weight:     500,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPost, "/orders/calculate-fee", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)
		assert.Contains(t, wOK.Body.String(), "25000")

		badInput := `{"provider":"unsupported_carrier"}`
		reqBad := httptest.NewRequest(http.MethodPost, "/orders/calculate-fee", bytes.NewReader([]byte(badInput)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CreateShipping valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		input := v1dto.CreateShippingInput{
			Provider:   "ghn",
			DistrictID: 10,
			WardCode:   "1001",
			Weight:     500,
			Length:     10,
			Width:      10,
			Height:     10,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPost, "/admin/orders/"+oUUID+"/shipping", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusCreated, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/admin/orders/bad-uuid/shipping", bytes.NewReader(raw))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("GetOrderTimeline valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodGet, "/orders/"+oUUID+"/timeline", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/orders/bad-uuid/timeline", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CancelShipping and GetPrintBill", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqCancel := httptest.NewRequest(http.MethodPost, "/admin/orders/"+oUUID+"/shipping/cancel", bytes.NewReader([]byte(`{}`)))
		reqCancel.Header.Set("Content-Type", "application/json")
		wCancel := httptest.NewRecorder()
		r.ServeHTTP(wCancel, reqCancel)
		assert.Equal(t, http.StatusOK, wCancel.Code)

		reqBill := httptest.NewRequest(http.MethodGet, "/admin/orders/"+oUUID+"/shipping/print-bill", nil)
		wBill := httptest.NewRecorder()
		r.ServeHTTP(wBill, reqBill)
		assert.Equal(t, http.StatusOK, wBill.Code)
	})

	t.Run("SyncOrderTracking and AdminSyncActiveTracking", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqSync := httptest.NewRequest(http.MethodPost, "/orders/"+oUUID+"/sync-tracking", nil)
		wSync := httptest.NewRecorder()
		r.ServeHTTP(wSync, reqSync)
		assert.Equal(t, http.StatusOK, wSync.Code)

		reqAdminSync := httptest.NewRequest(http.MethodPost, "/admin/orders/sync-active-tracking", nil)
		wAdminSync := httptest.NewRecorder()
		r.ServeHTTP(wAdminSync, reqAdminSync)
		assert.Equal(t, http.StatusOK, wAdminSync.Code)
	})
}

func TestOrderHandler_Returns(t *testing.T) {
	userUUID := uuid.New()
	mockRet := &mockOrderReturnService{}
	h := orderhandler.NewOrderHandler(nil, nil, mockRet)
	r := setupFullOrderRouter(h, &userUUID)

	t.Run("GetMyOrderReturns returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orders/returns?page=1&limit=10", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetOrderReturnDetails valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodGet, "/orders/"+oUUID+"/return", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/orders/bad-uuid/return", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CancelOrderReturn valid and invalid", func(t *testing.T) {
		oUUID := uuid.New().String()
		body := `{"reason":"mistake"}`
		reqOK := httptest.NewRequest(http.MethodPost, "/orders/"+oUUID+"/return/cancel", bytes.NewReader([]byte(body)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/orders/bad-uuid/return/cancel", bytes.NewReader([]byte(body)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("AdminGetReturns and AdminGetReturnDetail", func(t *testing.T) {
		reqAll := httptest.NewRequest(http.MethodGet, "/admin/returns?page=1&limit=10", nil)
		wAll := httptest.NewRecorder()
		r.ServeHTTP(wAll, reqAll)
		assert.Equal(t, http.StatusOK, wAll.Code)

		retUUID := uuid.New().String()
		reqDetail := httptest.NewRequest(http.MethodGet, "/admin/returns/"+retUUID, nil)
		wDetail := httptest.NewRecorder()
		r.ServeHTTP(wDetail, reqDetail)
		assert.Equal(t, http.StatusOK, wDetail.Code)
	})

	t.Run("AdminUpdateReturnStatus valid and invalid", func(t *testing.T) {
		retUUID := uuid.New().String()
		bodyOK := `{"status": 2}`
		reqOK := httptest.NewRequest(http.MethodPatch, "/admin/returns/"+retUUID, bytes.NewReader([]byte(bodyOK)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		bodyBad := `{"status": 99}`
		reqBad := httptest.NewRequest(http.MethodPatch, "/admin/returns/"+retUUID, bytes.NewReader([]byte(bodyBad)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("SubmitReturnShipping and UpdateReturnBankAccount", func(t *testing.T) {
		oUUID := uuid.New().String()
		shipInput := v1dto.SubmitReturnShippingInput{
			ShippingProvider: "ghn",
			ShippingCode:     "GHN12345",
		}
		rawShip, _ := json.Marshal(shipInput)
		reqShip := httptest.NewRequest(http.MethodPost, "/orders/"+oUUID+"/return/shipping", bytes.NewReader(rawShip))
		reqShip.Header.Set("Content-Type", "application/json")
		wShip := httptest.NewRecorder()
		r.ServeHTTP(wShip, reqShip)
		assert.Equal(t, http.StatusOK, wShip.Code)

		bankInput := v1dto.UpdateReturnBankAccountInput{
			BankName:        "Vietcombank",
			BankAccount:     "1234567890",
			BankAccountName: "Nguyen Van A",
		}
		rawBank, _ := json.Marshal(bankInput)
		reqBank := httptest.NewRequest(http.MethodPut, "/orders/"+oUUID+"/return/bank-account", bytes.NewReader(rawBank))
		reqBank.Header.Set("Content-Type", "application/json")
		wBank := httptest.NewRecorder()
		r.ServeHTTP(wBank, reqBank)
		assert.Equal(t, http.StatusOK, wBank.Code)
	})

	t.Run("AdminProcessReturnRefund", func(t *testing.T) {
		retUUID := uuid.New().String()
		req := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retUUID+"/refund", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestOrderHandler_UnauthorizedWhenNoUserInContext(t *testing.T) {
	mockCust := &mockCustomerOrderService{}
	h := orderhandler.NewOrderHandler(mockCust, nil, nil)
	r := setupFullOrderRouter(h, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
