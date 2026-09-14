package testutils

import (
	"context"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ v1service.OrderPricingService = (*MockOrderPricingService)(nil)
	_ v1service.InventoryService    = (*MockInventoryService)(nil)
	_ v1service.NotificationService = (*MockNotificationService)(nil)
	_ v1service.CartService         = (*MockCartService)(nil)
	_ rabbitmq.RabbitMQService      = (*MockRabbitMQService)(nil)
	_ shipping.ShippingProvider     = (*MockShippingProvider)(nil)
	_ payment.PaymentProvider       = (*MockPaymentProvider)(nil)
)

type MockOrderPricingService struct {
	CalculateItemsFunc               func(items []v1service.ItemPriceInput) ([]v1service.ItemPriceResult, float64, error)
	ValidateAndCalculateDiscountFunc func(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error)
	CalculateOrderTotalsFunc         func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error)
}

func (m *MockOrderPricingService) CalculateItems(items []v1service.ItemPriceInput) ([]v1service.ItemPriceResult, float64, error) {
	if m.CalculateItemsFunc != nil {
		return m.CalculateItemsFunc(items)
	}
	return nil, 0, nil
}

func (m *MockOrderPricingService) ValidateAndCalculateDiscount(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error) {
	if m.ValidateAndCalculateDiscountFunc != nil {
		return m.ValidateAndCalculateDiscountFunc(coupon, subtotal, now)
	}
	return 0, nil
}

func (m *MockOrderPricingService) CalculateOrderTotals(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
	if m.CalculateOrderTotalsFunc != nil {
		return m.CalculateOrderTotalsFunc(ctx, items, couponCode, userID, shippingFee, now)
	}
	return &v1service.OrderPricingResult{}, nil
}

type MockInventoryService struct {
	LockAndDeductStockFunc func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStockFunc       func(ctx context.Context, items []v1service.RestoreItem) error
}

func (m *MockInventoryService) LockAndDeductStock(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if m.LockAndDeductStockFunc != nil {
		return m.LockAndDeductStockFunc(ctx, items)
	}
	return nil, nil
}

func (m *MockInventoryService) RestoreStock(ctx context.Context, items []v1service.RestoreItem) error {
	if m.RestoreStockFunc != nil {
		return m.RestoreStockFunc(ctx, items)
	}
	return nil
}

type MockNotificationService struct {
	SendFunc                       func(ctx context.Context, input v1dto.SendNotificationInput) error
	CreateNotificationFunc         func(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error)
	BroadcastNotificationFunc      func(ctx *gin.Context, input v1dto.BroadcastNotificationInput) error
	GetMyNotificationsFunc         func(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetUnreadNotificationCountFunc func(ctx *gin.Context) (v1dto.UnreadCountDTO, error)
	MarkNotificationAsReadFunc     func(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error)
	MarkAllNotificationsAsReadFunc func(ctx *gin.Context) error
	DeleteNotificationFunc         func(ctx *gin.Context, notificationUUID uuid.UUID) error
}

func (m *MockNotificationService) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) CreateNotification(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error) {
	if m.CreateNotificationFunc != nil {
		return m.CreateNotificationFunc(ctx, input)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) BroadcastNotification(ctx *gin.Context, input v1dto.BroadcastNotificationInput) error {
	if m.BroadcastNotificationFunc != nil {
		return m.BroadcastNotificationFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) GetMyNotifications(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	if m.GetMyNotificationsFunc != nil {
		return m.GetMyNotificationsFunc(ctx, query)
	}
	return nil, 0, nil
}

func (m *MockNotificationService) GetUnreadNotificationCount(ctx *gin.Context) (v1dto.UnreadCountDTO, error) {
	if m.GetUnreadNotificationCountFunc != nil {
		return m.GetUnreadNotificationCountFunc(ctx)
	}
	return v1dto.UnreadCountDTO{}, nil
}

func (m *MockNotificationService) MarkNotificationAsRead(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error) {
	if m.MarkNotificationAsReadFunc != nil {
		return m.MarkNotificationAsReadFunc(ctx, notificationUUID)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) MarkAllNotificationsAsRead(ctx *gin.Context) error {
	if m.MarkAllNotificationsAsReadFunc != nil {
		return m.MarkAllNotificationsAsReadFunc(ctx)
	}
	return nil
}

func (m *MockNotificationService) DeleteNotification(ctx *gin.Context, notificationUUID uuid.UUID) error {
	if m.DeleteNotificationFunc != nil {
		return m.DeleteNotificationFunc(ctx, notificationUUID)
	}
	return nil
}

type MockCartService struct {
	GetMyCartFunc              func(ctx *gin.Context) (v1dto.CartDTO, error)
	AddItemToCartFunc          func(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantityFunc func(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCartFunc     func(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCartFunc            func(ctx *gin.Context) error
}

func (m *MockCartService) GetMyCart(ctx *gin.Context) (v1dto.CartDTO, error) {
	if m.GetMyCartFunc != nil {
		return m.GetMyCartFunc(ctx)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) AddItemToCart(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	if m.AddItemToCartFunc != nil {
		return m.AddItemToCartFunc(ctx, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) UpdateCartItemQuantity(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	if m.UpdateCartItemQuantityFunc != nil {
		return m.UpdateCartItemQuantityFunc(ctx, itemUUID, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) RemoveItemFromCart(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	if m.RemoveItemFromCartFunc != nil {
		return m.RemoveItemFromCartFunc(ctx, itemUUID)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) ClearMyCart(ctx *gin.Context) error {
	if m.ClearMyCartFunc != nil {
		return m.ClearMyCartFunc(ctx)
	}
	return nil
}

// =========================================================================
// MockRabbitMQService
// =========================================================================
type MockRabbitMQService struct {
	PublishFunc   func(ctx context.Context, queue string, message any) error
	ConsumeFunc   func(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error
	CloseFunc     func() error
	IsHealthyFunc func() bool
}

func (m *MockRabbitMQService) Publish(ctx context.Context, queue string, message any) error {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, queue, message)
	}
	return nil
}

func (m *MockRabbitMQService) Consume(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error {
	if m.ConsumeFunc != nil {
		return m.ConsumeFunc(ctx, queue, handler)
	}
	return nil
}

func (m *MockRabbitMQService) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockRabbitMQService) IsHealthy() bool {
	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return true
}

// =========================================================================
// MockShippingProvider
// =========================================================================
type MockShippingProvider struct {
	NameFunc                func() string
	CreateShippingOrderFunc func(ctx context.Context, req shipping.CreateOrderRequest) (*shipping.CreateOrderResponse, error)
	GetTrackingInfoFunc     func(ctx context.Context, shippingCode string) (*shipping.TrackingInfo, error)
	CancelShippingOrderFunc func(ctx context.Context, shippingCode string) error
	CalculateFeeFunc        func(ctx context.Context, req shipping.CreateOrderRequest) (float64, error)
	GetPrintBillURLFunc     func(ctx context.Context, shippingCode string) (*shipping.PrintBillResponse, error)
}

func (m *MockShippingProvider) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "ghn"
}

func (m *MockShippingProvider) CreateShippingOrder(ctx context.Context, req shipping.CreateOrderRequest) (*shipping.CreateOrderResponse, error) {
	if m.CreateShippingOrderFunc != nil {
		return m.CreateShippingOrderFunc(ctx, req)
	}
	return &shipping.CreateOrderResponse{}, nil
}

func (m *MockShippingProvider) GetTrackingInfo(ctx context.Context, shippingCode string) (*shipping.TrackingInfo, error) {
	if m.GetTrackingInfoFunc != nil {
		return m.GetTrackingInfoFunc(ctx, shippingCode)
	}
	return &shipping.TrackingInfo{}, nil
}

func (m *MockShippingProvider) CancelShippingOrder(ctx context.Context, shippingCode string) error {
	if m.CancelShippingOrderFunc != nil {
		return m.CancelShippingOrderFunc(ctx, shippingCode)
	}
	return nil
}

func (m *MockShippingProvider) CalculateFee(ctx context.Context, req shipping.CreateOrderRequest) (float64, error) {
	if m.CalculateFeeFunc != nil {
		return m.CalculateFeeFunc(ctx, req)
	}
	return 0, nil
}

func (m *MockShippingProvider) GetPrintBillURL(ctx context.Context, shippingCode string) (*shipping.PrintBillResponse, error) {
	if m.GetPrintBillURLFunc != nil {
		return m.GetPrintBillURLFunc(ctx, shippingCode)
	}
	return &shipping.PrintBillResponse{}, nil
}

// =========================================================================
// MockPaymentProvider
// =========================================================================
type MockPaymentProvider struct {
	CreatePaymentURLFunc       func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error)
	VerifyWebhookFunc          func(payload []byte, headers map[string]string) error
	ParseWebhookPayloadFunc    func(payload []byte) (payment.WebhookResult, error)
	QueryTransactionStatusFunc func(ctx context.Context, providerTxnID string) (payment.TransactionStatus, error)
	RefundFunc                 func(ctx context.Context, req payment.RefundRequest) (payment.RefundResponse, error)
}

func (m *MockPaymentProvider) CreatePaymentURL(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
	if m.CreatePaymentURLFunc != nil {
		return m.CreatePaymentURLFunc(ctx, req)
	}
	return payment.CreatePaymentResponse{}, nil
}

func (m *MockPaymentProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	if m.VerifyWebhookFunc != nil {
		return m.VerifyWebhookFunc(payload, headers)
	}
	return nil
}

func (m *MockPaymentProvider) ParseWebhookPayload(payload []byte) (payment.WebhookResult, error) {
	if m.ParseWebhookPayloadFunc != nil {
		return m.ParseWebhookPayloadFunc(payload)
	}
	return payment.WebhookResult{}, nil
}

func (m *MockPaymentProvider) QueryTransactionStatus(ctx context.Context, providerTxnID string) (payment.TransactionStatus, error) {
	if m.QueryTransactionStatusFunc != nil {
		return m.QueryTransactionStatusFunc(ctx, providerTxnID)
	}
	return payment.TxnStatusSuccess, nil
}

func (m *MockPaymentProvider) Refund(ctx context.Context, req payment.RefundRequest) (payment.RefundResponse, error) {
	if m.RefundFunc != nil {
		return m.RefundFunc(ctx, req)
	}
	return payment.RefundResponse{}, nil
}


