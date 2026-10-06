package testutils

import (
	"context"
	"io"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/mail"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ v1service.OrderPricingService  = (*MockOrderPricingService)(nil)
	_ v1service.InventoryService     = (*MockInventoryService)(nil)
	_ v1service.NotificationService  = (*MockNotificationService)(nil)
	_ v1service.CartService          = (*MockCartService)(nil)
	_ v1service.CouponService        = (*MockCouponService)(nil)
	_ v1service.ReviewService        = (*MockReviewService)(nil)
	_ v1service.AddressService       = (*MockAddressService)(nil)
	_ v1service.PaymentService       = (*MockPaymentService)(nil)
	_ v1service.PaymentRefundService = (*MockPaymentService)(nil)
	_ v1service.PaymentRefundService = (*MockPaymentRefundService)(nil)
	_ v1service.OrderService         = (*MockOrderService)(nil)
	_ v1service.CustomerOrderService = (*MockOrderService)(nil)
	_ v1service.OrderAdminService    = (*MockOrderService)(nil)
	_ v1service.OrderInvoiceService  = (*MockOrderService)(nil)
	_ v1service.OrderShippingService = (*MockOrderShippingService)(nil)
	_ v1service.OrderReturnService   = (*MockOrderReturnService)(nil)
	_ v1service.LocationService      = (*MockLocationService)(nil)
	_ v1service.ProductService       = (*MockProductService)(nil)
	_ v1service.ProductQueryService  = (*MockProductService)(nil)
	_ v1service.AdminProductService  = (*MockProductService)(nil)
	_ rabbitmq.RabbitMQService       = (*MockRabbitMQService)(nil)
	_ shipping.ShippingProvider      = (*MockShippingProvider)(nil)
	_ payment.PaymentProvider        = (*MockPaymentProvider)(nil)
	_ auth.TokenService              = (*MockTokenService)(nil)
	_ mail.EmailProviderService      = (*MockEmailProviderService)(nil)
	_ cache.RedisCacheService        = (*MockRedisCacheService)(nil)
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
	LockAndDeductStockFunc        func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	LockAndDeductStockWithRefFunc func(ctx context.Context, items []v1service.DeductItem, refID string, refType string, actorID *int32, note *string) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStockFunc              func(ctx context.Context, items []v1service.RestoreItem) error
	RestoreStockWithRefFunc       func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error
	AdjustStockFunc               func(ctx context.Context, actorUUID uuid.UUID, input v1dto.AdjustStockInput) (sqlc.StockMovement, error)
	ListStockMovementsFunc        func(ctx context.Context, params v1dto.GetStockMovementsParam) ([]v1dto.StockMovementDTO, int64, error)
}

func (m *MockInventoryService) LockAndDeductStock(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if m.LockAndDeductStockFunc != nil {
		return m.LockAndDeductStockFunc(ctx, items)
	}
	return nil, nil
}

func (m *MockInventoryService) LockAndDeductStockWithRef(ctx context.Context, items []v1service.DeductItem, refID string, refType string, actorID *int32, note *string) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if m.LockAndDeductStockWithRefFunc != nil {
		return m.LockAndDeductStockWithRefFunc(ctx, items, refID, refType, actorID, note)
	}
	return m.LockAndDeductStock(ctx, items)
}

func (m *MockInventoryService) RestoreStock(ctx context.Context, items []v1service.RestoreItem) error {
	if m.RestoreStockFunc != nil {
		return m.RestoreStockFunc(ctx, items)
	}
	return nil
}

func (m *MockInventoryService) RestoreStockWithRef(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
	if m.RestoreStockWithRefFunc != nil {
		return m.RestoreStockWithRefFunc(ctx, items, refID, refType, actorID, note)
	}
	return m.RestoreStock(ctx, items)
}

func (m *MockInventoryService) AdjustStock(ctx context.Context, actorUUID uuid.UUID, input v1dto.AdjustStockInput) (sqlc.StockMovement, error) {
	if m.AdjustStockFunc != nil {
		return m.AdjustStockFunc(ctx, actorUUID, input)
	}
	return sqlc.StockMovement{}, nil
}

func (m *MockInventoryService) ListStockMovements(ctx context.Context, params v1dto.GetStockMovementsParam) ([]v1dto.StockMovementDTO, int64, error) {
	if m.ListStockMovementsFunc != nil {
		return m.ListStockMovementsFunc(ctx, params)
	}
	return []v1dto.StockMovementDTO{}, 0, nil
}


type MockNotificationService struct {
	SendFunc                       func(ctx context.Context, input v1dto.SendNotificationInput) error
	SendAsyncFunc                  func(ctx context.Context, input v1dto.SendNotificationInput) error
	CreateNotificationFunc         func(ctx context.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error)
	BroadcastNotificationFunc      func(ctx context.Context, input v1dto.BroadcastNotificationInput) error
	GetMyNotificationsFunc         func(ctx context.Context, userUUID uuid.UUID, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetUnreadNotificationCountFunc func(ctx context.Context, userUUID uuid.UUID) (v1dto.UnreadCountDTO, error)
	MarkNotificationAsReadFunc     func(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error)
	MarkAllNotificationsAsReadFunc func(ctx context.Context, userUUID uuid.UUID) error
	DeleteNotificationFunc         func(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error
	NotifyAdminsFunc               func(ctx context.Context, input v1dto.AdminNotifyInput) error
	NotifyAdminsAsyncFunc          func(ctx context.Context, input v1dto.AdminNotifyInput) error
	GetAdminNotificationsFunc      func(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	PurgeStaleNotificationsFunc    func(ctx context.Context, retentionDays int) (int64, error)
}

func (m *MockNotificationService) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) SendAsync(ctx context.Context, input v1dto.SendNotificationInput) error {
	if m.SendAsyncFunc != nil {
		return m.SendAsyncFunc(ctx, input)
	}
	if m.SendFunc != nil {
		return m.SendFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) CreateNotification(ctx context.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error) {
	if m.CreateNotificationFunc != nil {
		return m.CreateNotificationFunc(ctx, input)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) BroadcastNotification(ctx context.Context, input v1dto.BroadcastNotificationInput) error {
	if m.BroadcastNotificationFunc != nil {
		return m.BroadcastNotificationFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	if m.GetMyNotificationsFunc != nil {
		return m.GetMyNotificationsFunc(ctx, userUUID, query)
	}
	return nil, 0, nil
}

func (m *MockNotificationService) GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (v1dto.UnreadCountDTO, error) {
	if m.GetUnreadNotificationCountFunc != nil {
		return m.GetUnreadNotificationCountFunc(ctx, userUUID)
	}
	return v1dto.UnreadCountDTO{}, nil
}

func (m *MockNotificationService) MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error) {
	if m.MarkNotificationAsReadFunc != nil {
		return m.MarkNotificationAsReadFunc(ctx, userUUID, notificationUUID)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	if m.MarkAllNotificationsAsReadFunc != nil {
		return m.MarkAllNotificationsAsReadFunc(ctx, userUUID)
	}
	return nil
}

func (m *MockNotificationService) DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error {
	if m.DeleteNotificationFunc != nil {
		return m.DeleteNotificationFunc(ctx, userUUID, notificationUUID)
	}
	return nil
}

func (m *MockNotificationService) NotifyAdmins(ctx context.Context, input v1dto.AdminNotifyInput) error {
	if m.NotifyAdminsFunc != nil {
		return m.NotifyAdminsFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) NotifyAdminsAsync(ctx context.Context, input v1dto.AdminNotifyInput) error {
	if m.NotifyAdminsAsyncFunc != nil {
		return m.NotifyAdminsAsyncFunc(ctx, input)
	}
	if m.NotifyAdminsFunc != nil {
		return m.NotifyAdminsFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	if m.GetAdminNotificationsFunc != nil {
		return m.GetAdminNotificationsFunc(ctx, userUUID, query)
	}
	return nil, 0, nil
}

func (m *MockNotificationService) PurgeStaleNotifications(ctx context.Context, retentionDays int) (int64, error) {
	if m.PurgeStaleNotificationsFunc != nil {
		return m.PurgeStaleNotificationsFunc(ctx, retentionDays)
	}
	return 0, nil
}

type MockCartService struct {
	GetMyCartFunc              func(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error)
	AddItemToCartFunc          func(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantityFunc func(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCartFunc     func(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCartFunc            func(ctx context.Context, userUUID uuid.UUID) error
	RemovePurchasedItemsFunc   func(ctx context.Context, userID int32, productIDs []int32) error
}

func (m *MockCartService) GetMyCart(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error) {
	if m.GetMyCartFunc != nil {
		return m.GetMyCartFunc(ctx, userUUID)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) AddItemToCart(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	if m.AddItemToCartFunc != nil {
		return m.AddItemToCartFunc(ctx, userUUID, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	if m.UpdateCartItemQuantityFunc != nil {
		return m.UpdateCartItemQuantityFunc(ctx, userUUID, itemUUID, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	if m.RemoveItemFromCartFunc != nil {
		return m.RemoveItemFromCartFunc(ctx, userUUID, itemUUID)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) ClearMyCart(ctx context.Context, userUUID uuid.UUID) error {
	if m.ClearMyCartFunc != nil {
		return m.ClearMyCartFunc(ctx, userUUID)
	}
	return nil
}

func (m *MockCartService) RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error {
	if m.RemovePurchasedItemsFunc != nil {
		return m.RemovePurchasedItemsFunc(ctx, userID, productIDs)
	}
	return nil
}

// =========================================================================
// MockCouponService
// =========================================================================
type MockCouponService struct {
	CreateCouponFunc     func(ctx context.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error)
	GetAllCouponsFunc    func(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error)
	GetCouponByUUIDFunc  func(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	UpdateCouponFunc     func(ctx context.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error)
	SoftDeleteCouponFunc func(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	RestoreCouponFunc    func(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	TrashCouponFunc      func(ctx context.Context, couponUUID uuid.UUID) error
	PreviewCouponFunc    func(ctx context.Context, userUUID uuid.UUID, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error)
	LockAndApplyCouponFunc func(ctx context.Context, couponID int32, userID int32, orderID int32) error
}

func (m *MockCouponService) CreateCoupon(ctx context.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error) {
	if m.CreateCouponFunc != nil {
		return m.CreateCouponFunc(ctx, input)
	}
	return v1dto.CouponDTO{}, nil
}

func (m *MockCouponService) GetAllCoupons(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error) {
	if m.GetAllCouponsFunc != nil {
		return m.GetAllCouponsFunc(ctx, search, orderBy, sort, coupon_type, isActive, page, limit, deleted)
	}
	return nil, 0, nil
}

func (m *MockCouponService) GetCouponByUUID(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	if m.GetCouponByUUIDFunc != nil {
		return m.GetCouponByUUIDFunc(ctx, couponUUID)
	}
	return v1dto.CouponDTO{}, nil
}

func (m *MockCouponService) UpdateCoupon(ctx context.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error) {
	if m.UpdateCouponFunc != nil {
		return m.UpdateCouponFunc(ctx, couponUUID, input)
	}
	return v1dto.CouponDTO{}, nil
}

func (m *MockCouponService) SoftDeleteCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	if m.SoftDeleteCouponFunc != nil {
		return m.SoftDeleteCouponFunc(ctx, couponUUID)
	}
	return v1dto.CouponDTO{}, nil
}

func (m *MockCouponService) RestoreCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	if m.RestoreCouponFunc != nil {
		return m.RestoreCouponFunc(ctx, couponUUID)
	}
	return v1dto.CouponDTO{}, nil
}

func (m *MockCouponService) TrashCoupon(ctx context.Context, couponUUID uuid.UUID) error {
	if m.TrashCouponFunc != nil {
		return m.TrashCouponFunc(ctx, couponUUID)
	}
	return nil
}

func (m *MockCouponService) PreviewCoupon(ctx context.Context, userUUID uuid.UUID, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error) {
	if m.PreviewCouponFunc != nil {
		return m.PreviewCouponFunc(ctx, userUUID, input)
	}
	return v1dto.PreviewCouponDTO{}, nil
}

func (m *MockCouponService) LockAndApplyCoupon(ctx context.Context, couponID int32, userID int32, orderID int32) error {
	if m.LockAndApplyCouponFunc != nil {
		return m.LockAndApplyCouponFunc(ctx, couponID, userID, orderID)
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

// MockTokenService
type MockTokenService struct {
	GenerateAccessTokenFunc           func(user sqlc.User) (string, error)
	GenerateAccessTokenWithFamilyFunc func(user sqlc.User, familyID string) (string, error)
	GenerateRefreshTokenFunc          func(user sqlc.User) (auth.RefreshToken, error)
	ParseTokenFunc                    func(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayloadFunc     func(tokenString string) (*auth.EncryptedPayload, error)
	StoreRefreshTokenFunc             func(token auth.RefreshToken) error
	ValidateRefreshTokenFunc          func(token string) (auth.RefreshToken, error)
	RevokeRefreshTokenFunc            func(token string) error
	RotateRefreshTokenFunc            func(oldToken string) (string, string, error)
	RevokeTokenFamilyFunc             func(familyID string) error
	GetTokenFamilyIDFunc              func(rawToken string) (string, error)
}

func (m *MockTokenService) GenerateAccessToken(user sqlc.User) (string, error) {
	if m.GenerateAccessTokenFunc != nil {
		return m.GenerateAccessTokenFunc(user)
	}
	return "mock-access-token", nil
}

func (m *MockTokenService) GenerateAccessTokenWithFamily(user sqlc.User, familyID string) (string, error) {
	if m.GenerateAccessTokenWithFamilyFunc != nil {
		return m.GenerateAccessTokenWithFamilyFunc(user, familyID)
	}
	if m.GenerateAccessTokenFunc != nil {
		return m.GenerateAccessTokenFunc(user)
	}
	return "mock-access-token", nil
}

func (m *MockTokenService) GenerateRefreshToken(user sqlc.User) (auth.RefreshToken, error) {
	if m.GenerateRefreshTokenFunc != nil {
		return m.GenerateRefreshTokenFunc(user)
	}
	return auth.RefreshToken{Token: "mock-refresh-token", UserUUID: user.UserUuid.String()}, nil
}

func (m *MockTokenService) ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	if m.ParseTokenFunc != nil {
		return m.ParseTokenFunc(tokenString)
	}
	return &jwt.Token{Valid: true}, jwt.MapClaims{"jti": "mock-jti", "exp": float64(time.Now().Add(time.Hour).Unix())}, nil
}

func (m *MockTokenService) DecryptAccessTokenPayload(tokenString string) (*auth.EncryptedPayload, error) {
	if m.DecryptAccessTokenPayloadFunc != nil {
		return m.DecryptAccessTokenPayloadFunc(tokenString)
	}
	return nil, nil
}

func (m *MockTokenService) StoreRefreshToken(token auth.RefreshToken) error {
	if m.StoreRefreshTokenFunc != nil {
		return m.StoreRefreshTokenFunc(token)
	}
	return nil
}

func (m *MockTokenService) ValidateRefreshToken(token string) (auth.RefreshToken, error) {
	if m.ValidateRefreshTokenFunc != nil {
		return m.ValidateRefreshTokenFunc(token)
	}
	return auth.RefreshToken{Token: token, UserUUID: uuid.New().String()}, nil
}

func (m *MockTokenService) RevokeRefreshToken(token string) error {
	if m.RevokeRefreshTokenFunc != nil {
		return m.RevokeRefreshTokenFunc(token)
	}
	return nil
}

func (m *MockTokenService) RotateRefreshToken(oldToken string) (string, string, error) {
	if m.RotateRefreshTokenFunc != nil {
		return m.RotateRefreshTokenFunc(oldToken)
	}
	return uuid.New().String(), "mock-new-refresh-token", nil
}

func (m *MockTokenService) RevokeTokenFamily(familyID string) error {
	if m.RevokeTokenFamilyFunc != nil {
		return m.RevokeTokenFamilyFunc(familyID)
	}
	return nil
}

func (m *MockTokenService) GetTokenFamilyID(rawToken string) (string, error) {
	if m.GetTokenFamilyIDFunc != nil {
		return m.GetTokenFamilyIDFunc(rawToken)
	}
	return "mock-family-id", nil
}

// MockEmailProviderService
type MockEmailProviderService struct {
	SendEmailFunc func(ctx context.Context, email *mail.Email) error
}

func (m *MockEmailProviderService) SendEmail(ctx context.Context, email *mail.Email) error {
	if m.SendEmailFunc != nil {
		return m.SendEmailFunc(ctx, email)
	}
	return nil
}

// MockRedisCacheService
type MockRedisCacheService struct {
	SetFunc    func(key string, value any, ttl time.Duration) error
	SetNXFunc  func(key string, value any, ttl time.Duration) (bool, error)
	GetFunc    func(key string, dest any) error
	ClearFunc  func(pattern string) error
	ExistsFunc func(key string) (bool, error)
	DelFunc    func(key string) error
	IncrFunc   func(key string) (int64, error)
	ExpireFunc func(key string, ttl time.Duration) error
	TTLFunc    func(key string) (time.Duration, error)
}

func (m *MockRedisCacheService) Set(key string, value any, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(key, value, ttl)
	}
	return nil
}

func (m *MockRedisCacheService) SetNX(key string, value any, ttl time.Duration) (bool, error) {
	if m.SetNXFunc != nil {
		return m.SetNXFunc(key, value, ttl)
	}
	return true, nil
}

func (m *MockRedisCacheService) Get(key string, dest any) error {
	if m.GetFunc != nil {
		return m.GetFunc(key, dest)
	}
	return nil
}

func (m *MockRedisCacheService) Clear(pattern string) error {
	if m.ClearFunc != nil {
		return m.ClearFunc(pattern)
	}
	return nil
}

func (m *MockRedisCacheService) Exists(key string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(key)
	}
	return false, nil
}

func (m *MockRedisCacheService) Del(key string) error {
	if m.DelFunc != nil {
		return m.DelFunc(key)
	}
	return nil
}

func (m *MockRedisCacheService) Incr(key string) (int64, error) {
	if m.IncrFunc != nil {
		return m.IncrFunc(key)
	}
	return 1, nil
}

func (m *MockRedisCacheService) Expire(key string, ttl time.Duration) error {
	if m.ExpireFunc != nil {
		return m.ExpireFunc(key, ttl)
	}
	return nil
}

func (m *MockRedisCacheService) TTL(key string) (time.Duration, error) {
	if m.TTLFunc != nil {
		return m.TTLFunc(key)
	}
	return 0, nil
}

// =========================================================================
// MockReviewService
// =========================================================================
type MockReviewService struct {
	CreateReviewFunc            func(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error)
	GetProductReviewsFunc       func(ctx context.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error)
	GetProductRatingSummaryFunc func(ctx context.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error)
	GetMyReviewsFunc            func(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error)
	UpdateReviewFunc            func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error)
	UpdateReviewStatusFunc      func(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error)
	DeleteReviewFunc            func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) error
	AdminGetAllReviewsFunc      func(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewDTO, int32, error)
	AdminReplyReviewFunc        func(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error)
	ToggleHelpfulVoteFunc       func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error)
	ReportReviewFunc            func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error
	AdminForceDeleteReviewFunc  func(ctx context.Context, reviewUUID uuid.UUID) error
	GetReviewReportsFunc        func(ctx context.Context, reviewUUID uuid.UUID) ([]v1dto.ReviewReportDTO, error)
	AdminDismissReportsFunc     func(ctx context.Context, reviewUUID uuid.UUID) error
}

func (m *MockReviewService) CreateReview(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error) {
	if m.CreateReviewFunc != nil {
		return m.CreateReviewFunc(ctx, userUUID, input, imageURLs, videoURL)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) GetProductReviews(ctx context.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error) {
	if m.GetProductReviewsFunc != nil {
		return m.GetProductReviewsFunc(ctx, productUUID, query)
	}
	return nil, 0, nil
}

func (m *MockReviewService) GetProductRatingSummary(ctx context.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error) {
	if m.GetProductRatingSummaryFunc != nil {
		return m.GetProductRatingSummaryFunc(ctx, productUUID)
	}
	return v1dto.RatingSummaryRow{}, nil
}

func (m *MockReviewService) GetMyReviews(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error) {
	if m.GetMyReviewsFunc != nil {
		return m.GetMyReviewsFunc(ctx, userUUID, query)
	}
	return nil, 0, nil
}

func (m *MockReviewService) UpdateReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error) {
	if m.UpdateReviewFunc != nil {
		return m.UpdateReviewFunc(ctx, userUUID, reviewUUID, input)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error) {
	if m.UpdateReviewStatusFunc != nil {
		return m.UpdateReviewStatusFunc(ctx, reviewUUID, input)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) DeleteReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) error {
	if m.DeleteReviewFunc != nil {
		return m.DeleteReviewFunc(ctx, userUUID, reviewUUID)
	}
	return nil
}

func (m *MockReviewService) AdminGetAllReviews(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewDTO, int32, error) {
	if m.AdminGetAllReviewsFunc != nil {
		return m.AdminGetAllReviewsFunc(ctx, query)
	}
	return nil, 0, nil
}

func (m *MockReviewService) AdminReplyReview(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error) {
	if m.AdminReplyReviewFunc != nil {
		return m.AdminReplyReviewFunc(ctx, reviewUUID, input)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) ToggleHelpfulVote(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error) {
	if m.ToggleHelpfulVoteFunc != nil {
		return m.ToggleHelpfulVoteFunc(ctx, userUUID, reviewUUID)
	}
	return v1dto.ToggleHelpfulResponseDTO{}, nil
}

func (m *MockReviewService) ReportReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error {
	if m.ReportReviewFunc != nil {
		return m.ReportReviewFunc(ctx, userUUID, reviewUUID, input)
	}
	return nil
}

func (m *MockReviewService) AdminForceDeleteReview(ctx context.Context, reviewUUID uuid.UUID) error {
	if m.AdminForceDeleteReviewFunc != nil {
		return m.AdminForceDeleteReviewFunc(ctx, reviewUUID)
	}
	return nil
}

func (m *MockReviewService) GetReviewReports(ctx context.Context, reviewUUID uuid.UUID) ([]v1dto.ReviewReportDTO, error) {
	if m.GetReviewReportsFunc != nil {
		return m.GetReviewReportsFunc(ctx, reviewUUID)
	}
	return nil, nil
}

func (m *MockReviewService) AdminDismissReports(ctx context.Context, reviewUUID uuid.UUID) error {
	if m.AdminDismissReportsFunc != nil {
		return m.AdminDismissReportsFunc(ctx, reviewUUID)
	}
	return nil
}

// =========================================================================
// MockBannerService
// =========================================================================

type MockBannerService struct {
	GetActiveBannersFunc        func(ctx context.Context, position string) ([]v1dto.BannerDTO, error)
	AdminGetBannersFunc         func(ctx context.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error)
	AdminGetBannerByUUIDFunc    func(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminCreateBannerFunc       func(ctx context.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBannerFunc       func(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBannerStatusFunc func(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminSoftDeleteBannerFunc   func(ctx context.Context, bannerUUID uuid.UUID) error
	AdminRestoreBannerFunc      func(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminTrashBannerFunc        func(ctx context.Context, bannerUUID uuid.UUID) error
}

func (m *MockBannerService) GetActiveBanners(ctx context.Context, position string) ([]v1dto.BannerDTO, error) {
	if m.GetActiveBannersFunc != nil {
		return m.GetActiveBannersFunc(ctx, position)
	}
	return nil, nil
}

func (m *MockBannerService) AdminGetBanners(ctx context.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error) {
	if m.AdminGetBannersFunc != nil {
		return m.AdminGetBannersFunc(ctx, query)
	}
	return nil, 0, nil
}

func (m *MockBannerService) AdminGetBannerByUUID(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	if m.AdminGetBannerByUUIDFunc != nil {
		return m.AdminGetBannerByUUIDFunc(ctx, bannerUUID)
	}
	return v1dto.AdminBannerDetailDTO{}, nil
}

func (m *MockBannerService) AdminCreateBanner(ctx context.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	if m.AdminCreateBannerFunc != nil {
		return m.AdminCreateBannerFunc(ctx, input)
	}
	return v1dto.AdminBannerDetailDTO{}, nil
}

func (m *MockBannerService) AdminUpdateBanner(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	if m.AdminUpdateBannerFunc != nil {
		return m.AdminUpdateBannerFunc(ctx, bannerUUID, input)
	}
	return v1dto.AdminBannerDetailDTO{}, nil
}

func (m *MockBannerService) AdminUpdateBannerStatus(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error) {
	if m.AdminUpdateBannerStatusFunc != nil {
		return m.AdminUpdateBannerStatusFunc(ctx, bannerUUID, input)
	}
	return v1dto.AdminBannerDetailDTO{}, nil
}

func (m *MockBannerService) AdminSoftDeleteBanner(ctx context.Context, bannerUUID uuid.UUID) error {
	if m.AdminSoftDeleteBannerFunc != nil {
		return m.AdminSoftDeleteBannerFunc(ctx, bannerUUID)
	}
	return nil
}

func (m *MockBannerService) AdminRestoreBanner(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	if m.AdminRestoreBannerFunc != nil {
		return m.AdminRestoreBannerFunc(ctx, bannerUUID)
	}
	return v1dto.AdminBannerDetailDTO{}, nil
}

func (m *MockBannerService) AdminTrashBanner(ctx context.Context, bannerUUID uuid.UUID) error {
	if m.AdminTrashBannerFunc != nil {
		return m.AdminTrashBannerFunc(ctx, bannerUUID)
	}
	return nil
}

// =========================================================================
// MockAddressService
// =========================================================================

type MockAddressService struct {
	CreateUserAddressFunc     func(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error)
	GetMyAddressesFunc        func(ctx context.Context, userUUID uuid.UUID) ([]v1dto.AddressDTO, error)
	GetAddressByUUIDFunc      func(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (v1dto.AddressDTO, error)
	UpdateUserAddressFunc     func(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error)
	SetDefaultUserAddressFunc func(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
	DeleteUserAddressFunc     func(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
}

func (m *MockAddressService) CreateUserAddress(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error) {
	if m.CreateUserAddressFunc != nil {
		return m.CreateUserAddressFunc(ctx, userUUID, input)
	}
	return v1dto.AddressDTO{}, nil
}

func (m *MockAddressService) GetMyAddresses(ctx context.Context, userUUID uuid.UUID) ([]v1dto.AddressDTO, error) {
	if m.GetMyAddressesFunc != nil {
		return m.GetMyAddressesFunc(ctx, userUUID)
	}
	return nil, nil
}

func (m *MockAddressService) GetAddressByUUID(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (v1dto.AddressDTO, error) {
	if m.GetAddressByUUIDFunc != nil {
		return m.GetAddressByUUIDFunc(ctx, userUUID, addressUUID)
	}
	return v1dto.AddressDTO{}, nil
}

func (m *MockAddressService) UpdateUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error) {
	if m.UpdateUserAddressFunc != nil {
		return m.UpdateUserAddressFunc(ctx, userUUID, addressUUID, input)
	}
	return v1dto.AddressDTO{}, nil
}

func (m *MockAddressService) SetDefaultUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	if m.SetDefaultUserAddressFunc != nil {
		return m.SetDefaultUserAddressFunc(ctx, userUUID, addressUUID)
	}
	return nil
}

func (m *MockAddressService) DeleteUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	if m.DeleteUserAddressFunc != nil {
		return m.DeleteUserAddressFunc(ctx, userUUID, addressUUID)
	}
	return nil
}

// =========================================================================
// MockPaymentService
// =========================================================================

type MockPaymentService struct {
	CreatePaymentIntentFunc      func(ctx context.Context, userUUID uuid.UUID, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error)
	HandleWebhookFunc            func(ctx context.Context, provider string, rawPayload []byte, headers map[string]string) error
	ProcessWebhookFunc           func(ctx context.Context, msg v1dto.PaymentWebhookMessage) error
	ProcessWebhookMessageFunc    func(headers amqp.Table, body []byte) error
	GetPaymentHistoryFunc        func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error)
	ConfirmCODPaymentFunc        func(ctx context.Context, orderUUID uuid.UUID) error
	RefundPaymentFunc            func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
	ReconcilePendingPaymentsFunc func(ctx context.Context) error
}

func (m *MockPaymentService) CreatePaymentIntent(ctx context.Context, userUUID uuid.UUID, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error) {
	if m.CreatePaymentIntentFunc != nil {
		return m.CreatePaymentIntentFunc(ctx, userUUID, clientIP, idempotencyKey, input)
	}
	return v1dto.PaymentIntentDTO{}, nil
}

func (m *MockPaymentService) HandleWebhook(ctx context.Context, provider string, rawPayload []byte, headers map[string]string) error {
	if m.HandleWebhookFunc != nil {
		return m.HandleWebhookFunc(ctx, provider, rawPayload, headers)
	}
	return nil
}

func (m *MockPaymentService) ProcessWebhook(ctx context.Context, msg v1dto.PaymentWebhookMessage) error {
	if m.ProcessWebhookFunc != nil {
		return m.ProcessWebhookFunc(ctx, msg)
	}
	return nil
}

func (m *MockPaymentService) ProcessWebhookMessage(headers amqp.Table, body []byte) error {
	if m.ProcessWebhookMessageFunc != nil {
		return m.ProcessWebhookMessageFunc(headers, body)
	}
	return nil
}

func (m *MockPaymentService) GetPaymentHistory(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error) {
	if m.GetPaymentHistoryFunc != nil {
		return m.GetPaymentHistoryFunc(ctx, userUUID, orderUUID)
	}
	return nil, nil
}

func (m *MockPaymentService) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	if m.ConfirmCODPaymentFunc != nil {
		return m.ConfirmCODPaymentFunc(ctx, orderUUID)
	}
	return nil
}

func (m *MockPaymentService) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if m.RefundPaymentFunc != nil {
		return m.RefundPaymentFunc(ctx, idempotencyKey, input)
	}
	return v1dto.PaymentTransactionDTO{}, nil
}

func (m *MockPaymentService) ReconcilePendingPayments(ctx context.Context) error {
	if m.ReconcilePendingPaymentsFunc != nil {
		return m.ReconcilePendingPaymentsFunc(ctx)
	}
	return nil
}

// MockPaymentRefundService
type MockPaymentRefundService struct {
	RefundPaymentFunc func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
}

func (m *MockPaymentRefundService) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if m.RefundPaymentFunc != nil {
		return m.RefundPaymentFunc(ctx, idempotencyKey, input)
	}
	return v1dto.PaymentTransactionDTO{}, nil
}

type MockOrderService struct {
	CreateOrderFunc              func(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetMyOrdersFunc              func(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUIDFunc           func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetAllOrdersFunc             func(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	UpdateOrderStatusFunc        func(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	CancelOrderFunc              func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error)
	UpdateOrderShippingInfoFunc  func(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	UpdateOrderPaymentStatusFunc func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetOrderInvoicePDFFunc       func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
	BulkUpdateStatusFunc         func(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	BulkCancelFunc               func(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

func (m *MockOrderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.CreateOrderFunc != nil {
		return m.CreateOrderFunc(ctx, userUUID, input)
	}
	return sqlc.Order{}, nil, nil
}

func (m *MockOrderService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if m.GetMyOrdersFunc != nil {
		return m.GetMyOrdersFunc(ctx, userUUID, input)
	}
	return nil, 0, nil
}

func (m *MockOrderService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.GetOrderByUUIDFunc != nil {
		return m.GetOrderByUUIDFunc(ctx, userUUID, orderUUID)
	}
	return sqlc.GetOrderByUUIDRow{}, nil, nil
}

func (m *MockOrderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if m.GetAllOrdersFunc != nil {
		return m.GetAllOrdersFunc(ctx, input)
	}
	return nil, 0, nil
}

func (m *MockOrderService) UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.UpdateOrderStatusFunc != nil {
		return m.UpdateOrderStatusFunc(ctx, input)
	}
	return sqlc.Order{}, nil, nil
}

func (m *MockOrderService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error) {
	if m.CancelOrderFunc != nil {
		return m.CancelOrderFunc(ctx, userUUID, orderUUID, reason)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.UpdateOrderShippingInfoFunc != nil {
		return m.UpdateOrderShippingInfoFunc(ctx, userUUID, input)
	}
	return sqlc.Order{}, nil, nil
}

func (m *MockOrderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.UpdateOrderPaymentStatusFunc != nil {
		return m.UpdateOrderPaymentStatusFunc(ctx, userUUID, orderUUID, input)
	}
	return sqlc.Order{}, nil, nil
}

func (m *MockOrderService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	if m.GetOrderInvoicePDFFunc != nil {
		return m.GetOrderInvoicePDFFunc(ctx, userUUID, orderUUID)
	}
	return nil, "", nil
}

func (m *MockOrderService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.BulkUpdateStatusFunc != nil {
		return m.BulkUpdateStatusFunc(ctx, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

func (m *MockOrderService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.BulkCancelFunc != nil {
		return m.BulkCancelFunc(ctx, userUUID, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

type MockOrderInvoiceService struct {
	GetOrderInvoicePDFFunc func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
}

func (m *MockOrderInvoiceService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	if m.GetOrderInvoicePDFFunc != nil {
		return m.GetOrderInvoicePDFFunc(ctx, userUUID, orderUUID)
	}
	return nil, "", nil
}

type MockOrderAdminService struct {
	BulkUpdateStatusFunc func(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	BulkCancelFunc       func(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

func (m *MockOrderAdminService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.BulkUpdateStatusFunc != nil {
		return m.BulkUpdateStatusFunc(ctx, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

func (m *MockOrderAdminService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	if m.BulkCancelFunc != nil {
		return m.BulkCancelFunc(ctx, userUUID, input)
	}
	return &v1dto.BulkOperationResultDTO{Total: len(input.OrderUUIDs), Succeeded: len(input.OrderUUIDs)}, nil
}

type MockOrderShippingService struct {
	CreateShippingFunc                func(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error)
	ProcessShippingOrderFunc          func(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error
	ProcessShippingOrderMessageFunc   func(headers amqp.Table, body []byte) error
	GetOrderTimelineFunc              func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error)
	HandleShippingWebhookFunc         func(ctx context.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error
	CalculateShippingFeeFunc          func(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error)
	CancelShippingFunc                func(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error)
	ConfirmOrderDeliveryFunc          func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error)
	GetPrintBillFunc                  func(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error)
	GetProvincesFunc                  func(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	GetDistrictsFunc                  func(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	GetWardsFunc                      func(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)
	SyncOrderTrackingFunc             func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error)
	SyncActiveShippingOrdersFunc      func(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error)
	AdminSyncActiveShippingOrdersFunc func(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error)
}

func (m *MockOrderShippingService) CreateShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error) {
	if m.CreateShippingFunc != nil {
		return m.CreateShippingFunc(ctx, orderUUID, input)
	}
	return v1dto.AsyncShippingResponseDTO{}, nil
}

func (m *MockOrderShippingService) ProcessShippingOrder(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error {
	if m.ProcessShippingOrderFunc != nil {
		return m.ProcessShippingOrderFunc(ctx, msg)
	}
	return nil
}

func (m *MockOrderShippingService) ProcessShippingOrderMessage(headers amqp.Table, body []byte) error {
	if m.ProcessShippingOrderMessageFunc != nil {
		return m.ProcessShippingOrderMessageFunc(headers, body)
	}
	return nil
}

func (m *MockOrderShippingService) GetOrderTimeline(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error) {
	if m.GetOrderTimelineFunc != nil {
		return m.GetOrderTimelineFunc(ctx, userUUID, orderUUID)
	}
	return v1dto.OrderTimelineResponseDTO{}, nil
}

func (m *MockOrderShippingService) HandleShippingWebhook(ctx context.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error {
	if m.HandleShippingWebhookFunc != nil {
		return m.HandleShippingWebhookFunc(ctx, rawBody, payload)
	}
	return nil
}

func (m *MockOrderShippingService) CalculateShippingFee(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error) {
	if m.CalculateShippingFeeFunc != nil {
		return m.CalculateShippingFeeFunc(ctx, input)
	}
	return v1dto.ShippingFeeResponseDTO{}, nil
}

func (m *MockOrderShippingService) CancelShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error) {
	if m.CancelShippingFunc != nil {
		return m.CancelShippingFunc(ctx, orderUUID, input)
	}
	return v1dto.CancelShippingResponseDTO{}, nil
}

func (m *MockOrderShippingService) ConfirmOrderDelivery(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error) {
	if m.ConfirmOrderDeliveryFunc != nil {
		return m.ConfirmOrderDeliveryFunc(ctx, userUUID, orderUUID)
	}
	return v1dto.ConfirmDeliveryResponseDTO{}, nil
}

func (m *MockOrderShippingService) GetPrintBill(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error) {
	if m.GetPrintBillFunc != nil {
		return m.GetPrintBillFunc(ctx, orderUUID)
	}
	return v1dto.PrintBillResponseDTO{}, nil
}

func (m *MockOrderShippingService) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	if m.GetProvincesFunc != nil {
		return m.GetProvincesFunc(ctx)
	}
	return nil, nil
}

func (m *MockOrderShippingService) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	if m.GetDistrictsFunc != nil {
		return m.GetDistrictsFunc(ctx, provinceID)
	}
	return nil, nil
}

func (m *MockOrderShippingService) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	if m.GetWardsFunc != nil {
		return m.GetWardsFunc(ctx, districtID)
	}
	return nil, nil
}

func (m *MockOrderShippingService) SyncOrderTracking(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error) {
	if m.SyncOrderTrackingFunc != nil {
		return m.SyncOrderTrackingFunc(ctx, userUUID, orderUUID)
	}
	return v1dto.SyncTrackingResponseDTO{}, nil
}

func (m *MockOrderShippingService) SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	if m.SyncActiveShippingOrdersFunc != nil {
		return m.SyncActiveShippingOrdersFunc(ctx)
	}
	return v1dto.SyncActiveShippingStatsDTO{}, nil
}

func (m *MockOrderShippingService) AdminSyncActiveShippingOrders(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error) {
	if m.AdminSyncActiveShippingOrdersFunc != nil {
		return m.AdminSyncActiveShippingOrdersFunc(ctx, userUUID)
	}
	return v1dto.SyncActiveShippingStatsDTO{}, nil
}

type MockOrderReturnService struct {
	RequestOrderReturnFunc       func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error)
	GetOrderReturnDetailsFunc    func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error)
	GetMyOrderReturnsFunc        func(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error)
	AdminGetReturnsFunc          func(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error)
	AdminGetReturnDetailFunc     func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error)
	AdminUpdateReturnStatusFunc  func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error)
	CancelOrderReturnFunc        func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error)
	SubmitReturnShippingFunc     func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error)
	UpdateReturnBankAccountFunc  func(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error)
	AdminProcessReturnRefundFunc func(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error)
}

func (m *MockOrderReturnService) RequestOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error) {
	if m.RequestOrderReturnFunc != nil {
		return m.RequestOrderReturnFunc(ctx, userUUID, orderUUID, input, imageURLs, videoURL)
	}
	return v1dto.OrderReturnDTO{}, nil
}

func (m *MockOrderReturnService) GetOrderReturnDetails(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error) {
	if m.GetOrderReturnDetailsFunc != nil {
		return m.GetOrderReturnDetailsFunc(ctx, userUUID, orderUUID)
	}
	return v1dto.OrderReturnDetailDTO{}, nil
}

func (m *MockOrderReturnService) GetMyOrderReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error) {
	if m.GetMyOrderReturnsFunc != nil {
		return m.GetMyOrderReturnsFunc(ctx, userUUID, query)
	}
	return nil, 0, nil
}

func (m *MockOrderReturnService) AdminGetReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error) {
	if m.AdminGetReturnsFunc != nil {
		return m.AdminGetReturnsFunc(ctx, userUUID, query)
	}
	return nil, 0, nil
}

func (m *MockOrderReturnService) AdminGetReturnDetail(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error) {
	if m.AdminGetReturnDetailFunc != nil {
		return m.AdminGetReturnDetailFunc(ctx, userUUID, returnUUID)
	}
	return v1dto.AdminReturnDetailDTO{}, nil
}

func (m *MockOrderReturnService) AdminUpdateReturnStatus(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error) {
	if m.AdminUpdateReturnStatusFunc != nil {
		return m.AdminUpdateReturnStatusFunc(ctx, userUUID, returnUUID, input)
	}
	return v1dto.AdminReturnSummaryDTO{}, nil
}

func (m *MockOrderReturnService) CancelOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error) {
	if m.CancelOrderReturnFunc != nil {
		return m.CancelOrderReturnFunc(ctx, userUUID, orderUUID, input)
	}
	return v1dto.CancelOrderReturnResponseDTO{}, nil
}

func (m *MockOrderReturnService) SubmitReturnShipping(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error) {
	if m.SubmitReturnShippingFunc != nil {
		return m.SubmitReturnShippingFunc(ctx, userUUID, orderUUID, input)
	}
	return v1dto.SubmitReturnShippingResponseDTO{}, nil
}

func (m *MockOrderReturnService) UpdateReturnBankAccount(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error) {
	if m.UpdateReturnBankAccountFunc != nil {
		return m.UpdateReturnBankAccountFunc(ctx, userUUID, orderUUID, input)
	}
	return v1dto.UpdateReturnBankAccountResponseDTO{}, nil
}

func (m *MockOrderReturnService) AdminProcessReturnRefund(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error) {
	if m.AdminProcessReturnRefundFunc != nil {
		return m.AdminProcessReturnRefundFunc(ctx, userUUID, returnUUID, input)
	}
	return v1dto.AdminReturnRefundResponseDTO{}, nil
}

type MockLocationService struct {
	GetProvincesFunc func(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	GetDistrictsFunc func(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	GetWardsFunc     func(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)
}

func (m *MockLocationService) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	if m.GetProvincesFunc != nil {
		return m.GetProvincesFunc(ctx)
	}
	return nil, nil
}

func (m *MockLocationService) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	if m.GetDistrictsFunc != nil {
		return m.GetDistrictsFunc(ctx, provinceID)
	}
	return nil, nil
}

func (m *MockLocationService) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	if m.GetWardsFunc != nil {
		return m.GetWardsFunc(ctx, districtID)
	}
	return nil, nil
}

type MockProductService struct {
	CreateProductFunc     func(ctx context.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error)
	GetAllProductsFunc    func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error)
	GetProductByUUIDFunc  func(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	GetProductBySlugFunc  func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error)
	SearchProductsFunc    func(ctx context.Context, q, categoryUUID string, minPrice, maxPrice, minRating float64, sort string, page, limit int32) ([]v1dto.ProductDTO, int64, error)
	SuggestProductsFunc   func(ctx context.Context, q string, limit int32) ([]v1dto.ProductSuggestionDTO, error)
	UpdateProductFunc     func(ctx context.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error)
	SoftDeleteProductFunc func(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error)
	RestoreProductFunc    func(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error)
	TrashProductFunc      func(ctx context.Context, productUUID uuid.UUID) error
	ImportProductsCSVFunc func(ctx context.Context, file io.Reader) (*v1dto.ProductCSVImportResponse, error)
}

func (m *MockProductService) CreateProduct(ctx context.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error) {
	if m.CreateProductFunc != nil {
		return m.CreateProductFunc(ctx, input)
	}
	return sqlc.CreateProductRow{}, nil
}

func (m *MockProductService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error) {
	if m.GetAllProductsFunc != nil {
		return m.GetAllProductsFunc(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, page, limit, deleted)
	}
	return nil, 0, nil
}

func (m *MockProductService) SearchProducts(ctx context.Context, q, categoryUUID string, minPrice, maxPrice, minRating float64, sort string, page, limit int32) ([]v1dto.ProductDTO, int64, error) {
	if m.SearchProductsFunc != nil {
		return m.SearchProductsFunc(ctx, q, categoryUUID, minPrice, maxPrice, minRating, sort, page, limit)
	}
	return []v1dto.ProductDTO{}, 0, nil
}

func (m *MockProductService) SuggestProducts(ctx context.Context, q string, limit int32) ([]v1dto.ProductSuggestionDTO, error) {
	if m.SuggestProductsFunc != nil {
		return m.SuggestProductsFunc(ctx, q, limit)
	}
	return []v1dto.ProductSuggestionDTO{}, nil
}

func (m *MockProductService) GetProductByUUID(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	if m.GetProductByUUIDFunc != nil {
		return m.GetProductByUUIDFunc(ctx, productUUID)
	}
	return sqlc.GetProductByUUIDRow{}, nil
}

func (m *MockProductService) GetProductBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
	if m.GetProductBySlugFunc != nil {
		return m.GetProductBySlugFunc(ctx, slug)
	}
	return sqlc.GetProductBySlugRow{}, nil
}

func (m *MockProductService) UpdateProduct(ctx context.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error) {
	if m.UpdateProductFunc != nil {
		return m.UpdateProductFunc(ctx, input)
	}
	return sqlc.UpdateProductRow{}, nil
}

func (m *MockProductService) SoftDeleteProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	if m.SoftDeleteProductFunc != nil {
		return m.SoftDeleteProductFunc(ctx, productUUID)
	}
	return sqlc.Product{}, nil
}

func (m *MockProductService) RestoreProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	if m.RestoreProductFunc != nil {
		return m.RestoreProductFunc(ctx, productUUID)
	}
	return sqlc.Product{}, nil
}

func (m *MockProductService) TrashProduct(ctx context.Context, productUUID uuid.UUID) error {
	if m.TrashProductFunc != nil {
		return m.TrashProductFunc(ctx, productUUID)
	}
	return nil
}

func (m *MockProductService) ImportProductsCSV(ctx context.Context, file io.Reader) (*v1dto.ProductCSVImportResponse, error) {
	if m.ImportProductsCSVFunc != nil {
		return m.ImportProductsCSVFunc(ctx, file)
	}
	return &v1dto.ProductCSVImportResponse{}, nil
}


