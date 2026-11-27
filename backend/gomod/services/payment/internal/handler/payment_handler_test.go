package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	paymentdto "goshop/services/payment/internal/dto"
	paymenthandler "goshop/services/payment/internal/handler"
	paymentservice "goshop/services/payment/internal/service"
	"goshop/shared/pkg/utils"
)

type mockPaymentService struct {
	paymentservice.PaymentService
	createIntentFn  func(ctx context.Context, in paymentdto.CreatePaymentIntentInput) (paymentdto.PaymentIntentDTO, error)
	getTxnsFn       func(ctx context.Context, u uuid.UUID) ([]paymentdto.PaymentTransactionDTO, error)
	handleWebhookFn func(ctx context.Context, provider string, body []byte, headers map[string]string) (paymentservice.WebhookOutcome, error)
	refundFn        func(ctx context.Context, in paymentdto.RefundPaymentInput) (paymentdto.PaymentTransactionDTO, error)
}

func (m *mockPaymentService) CreatePaymentIntent(ctx context.Context, in paymentdto.CreatePaymentIntentInput) (paymentdto.PaymentIntentDTO, error) {
	if m.createIntentFn != nil {
		return m.createIntentFn(ctx, in)
	}
	return paymentdto.PaymentIntentDTO{}, nil
}
func (m *mockPaymentService) GetPaymentTransactionsByOrderUUID(ctx context.Context, u uuid.UUID) ([]paymentdto.PaymentTransactionDTO, error) {
	if m.getTxnsFn != nil {
		return m.getTxnsFn(ctx, u)
	}
	return nil, nil
}
func (m *mockPaymentService) HandleWebhook(ctx context.Context, provider string, body []byte, headers map[string]string) (paymentservice.WebhookOutcome, error) {
	if m.handleWebhookFn != nil {
		return m.handleWebhookFn(ctx, provider, body, headers)
	}
	return paymentservice.WebhookProcessed, nil
}
func (m *mockPaymentService) RefundPayment(ctx context.Context, in paymentdto.RefundPaymentInput) (paymentdto.PaymentTransactionDTO, error) {
	if m.refundFn != nil {
		return m.refundFn(ctx, in)
	}
	return paymentdto.PaymentTransactionDTO{}, nil
}

func setupPaymentRouter(h *paymenthandler.PaymentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments/create-intent", h.CreatePaymentIntent)
	r.GET("/payments/order/:order_uuid", h.GetPaymentTransactions)
	r.POST("/webhooks/payment/:provider", h.HandleWebhook)
	r.POST("/admin/payments/refund", h.RefundPayment)
	return r
}

func TestPaymentHandler_CreatePaymentIntent(t *testing.T) {
	orderUUID := uuid.New()
	mockSvc := &mockPaymentService{
		createIntentFn: func(_ context.Context, in paymentdto.CreatePaymentIntentInput) (paymentdto.PaymentIntentDTO, error) {
			if in.OrderUUID == orderUUID {
				return paymentdto.PaymentIntentDTO{PaymentURL: "https://pay.momo.vn/123", Provider: "momo"}, nil
			}
			return paymentdto.PaymentIntentDTO{}, errors.New("order not found")
		},
	}
	r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

	t.Run("success returns 200 with payment URL", func(t *testing.T) {
		body, _ := json.Marshal(paymentdto.CreatePaymentIntentInput{OrderUUID: orderUUID, Provider: "momo"})
		req := httptest.NewRequest(http.MethodPost, "/payments/create-intent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "https://pay.momo.vn/123")
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payments/create-intent", bytes.NewReader([]byte("bad json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPaymentHandler_GetPaymentTransactions(t *testing.T) {
	orderUUID := uuid.New()
	mockSvc := &mockPaymentService{
		getTxnsFn: func(_ context.Context, u uuid.UUID) ([]paymentdto.PaymentTransactionDTO, error) {
			if u == orderUUID {
				return []paymentdto.PaymentTransactionDTO{{UUID: uuid.New().String(), Status: "PAID"}}, nil
			}
			return nil, errors.New("db error")
		},
	}
	r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

	t.Run("valid uuid returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/order/"+orderUUID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "PAID")
	})

	t.Run("invalid uuid returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/order/not-a-uuid", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPaymentHandler_HandleWebhook(t *testing.T) {
	t.Run("vnpay webhook returns 200 with RspCode", func(t *testing.T) {
		mockSvc := &mockPaymentService{
			handleWebhookFn: func(_ context.Context, provider string, _ []byte, _ map[string]string) (paymentservice.WebhookOutcome, error) {
				return paymentservice.WebhookAlreadyProcessed, nil
			},
		}
		r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

		req := httptest.NewRequest(http.MethodPost, "/webhooks/payment/vnpay?vnp_TxnRef=123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"RspCode":"02"`)
	})

	t.Run("momo webhook success returns 200", func(t *testing.T) {
		mockSvc := &mockPaymentService{
			handleWebhookFn: func(context.Context, string, []byte, map[string]string) (paymentservice.WebhookOutcome, error) {
				return paymentservice.WebhookProcessed, nil
			},
		}
		r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

		req := httptest.NewRequest(http.MethodPost, "/webhooks/payment/momo", bytes.NewReader([]byte(`{"orderId":"1"}`)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"success"`)
	})

	t.Run("momo webhook failure returns 400", func(t *testing.T) {
		mockSvc := &mockPaymentService{
			handleWebhookFn: func(context.Context, string, []byte, map[string]string) (paymentservice.WebhookOutcome, error) {
				return paymentservice.WebhookUnknown, utils.NewError(utils.ErrCodeBadRequest, "bad signature")
			},
		}
		r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

		req := httptest.NewRequest(http.MethodPost, "/webhooks/payment/momo", bytes.NewReader([]byte(`{"orderId":"1"}`)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPaymentHandler_RefundPayment(t *testing.T) {
	orderUUID := uuid.New()
	mockSvc := &mockPaymentService{
		refundFn: func(_ context.Context, in paymentdto.RefundPaymentInput) (paymentdto.PaymentTransactionDTO, error) {
			if in.OrderUUID == orderUUID.String() {
				return paymentdto.PaymentTransactionDTO{Status: "REFUNDED"}, nil
			}
			return paymentdto.PaymentTransactionDTO{}, errors.New("refund error")
		},
	}
	r := setupPaymentRouter(paymenthandler.NewPaymentHandler(mockSvc))

	t.Run("valid refund returns 200", func(t *testing.T) {
		body, _ := json.Marshal(paymentdto.RefundPaymentInput{OrderUUID: orderUUID.String(), Amount: 50000, Reason: "Defective item"})
		req := httptest.NewRequest(http.MethodPost, "/admin/payments/refund", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "REFUNDED")
	})
}
