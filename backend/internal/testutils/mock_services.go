package testutils

import (
	"context"
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

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ v1service.OrderPricingService = (*MockOrderPricingService)(nil)
	_ v1service.InventoryService    = (*MockInventoryService)(nil)
	_ v1service.NotificationService = (*MockNotificationService)(nil)
	_ v1service.CartService         = (*MockCartService)(nil)
	_ v1service.CouponService       = (*MockCouponService)(nil)
	_ v1service.ReviewService       = (*MockReviewService)(nil)
	_ rabbitmq.RabbitMQService      = (*MockRabbitMQService)(nil)
	_ shipping.ShippingProvider     = (*MockShippingProvider)(nil)
	_ payment.PaymentProvider       = (*MockPaymentProvider)(nil)
	_ auth.TokenService             = (*MockTokenService)(nil)
	_ mail.EmailProviderService     = (*MockEmailProviderService)(nil)
	_ cache.RedisCacheService       = (*MockRedisCacheService)(nil)
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
	GetMyCartFunc              func(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error)
	AddItemToCartFunc          func(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantityFunc func(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCartFunc     func(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCartFunc            func(ctx context.Context, userUUID uuid.UUID) error
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
	GenerateAccessTokenFunc       func(user sqlc.User) (string, error)
	GenerateRefreshTokenFunc      func(user sqlc.User) (auth.RefreshToken, error)
	ParseTokenFunc                func(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayloadFunc func(tokenString string) (*auth.EncryptedPayload, error)
	StoreRefreshTokenFunc         func(token auth.RefreshToken) error
	ValidateRefreshTokenFunc      func(token string) (auth.RefreshToken, error)
	RevokeRefreshTokenFunc        func(token string) error
}

func (m *MockTokenService) GenerateAccessToken(user sqlc.User) (string, error) {
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
	AdminGetAllReviewsFunc      func(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewRow, int32, error)
	AdminReplyReviewFunc        func(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error)
	ToggleHelpfulVoteFunc       func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error)
	ReportReviewFunc            func(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error
	AdminForceDeleteReviewFunc  func(ctx context.Context, reviewUUID uuid.UUID) error
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

func (m *MockReviewService) AdminGetAllReviews(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewRow, int32, error) {
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




