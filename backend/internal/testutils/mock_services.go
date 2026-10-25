package testutils

import (
	"context"
	"time"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ v1service.ReviewService   = (*MockReviewService)(nil)
	_ rabbitmq.RabbitMQService  = (*MockRabbitMQService)(nil)
	_ shipping.ShippingProvider = (*MockShippingProvider)(nil)
	_ auth.TokenService         = (*MockTokenService)(nil)
	_ mail.EmailProviderService = (*MockEmailProviderService)(nil)
	_ cache.RedisCacheService   = (*MockRedisCacheService)(nil)
)

type MockOrderReader struct {
	GetOrderSummaryFunc func(ctx context.Context, orderUUID uuid.UUID) (*v1service.OrderSummary, error)
	VerifyPurchaseFunc  func(ctx context.Context, orderUUID uuid.UUID, userID int32, productID int32) (*v1service.PurchaseVerification, error)
}

func (m *MockOrderReader) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*v1service.OrderSummary, error) {
	if m.GetOrderSummaryFunc != nil {
		return m.GetOrderSummaryFunc(ctx, orderUUID)
	}
	return nil, nil
}

func (m *MockOrderReader) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID int32, productID int32) (*v1service.PurchaseVerification, error) {
	if m.VerifyPurchaseFunc != nil {
		return m.VerifyPurchaseFunc(ctx, orderUUID, userID, productID)
	}
	return nil, nil
}

// =========================================================================
// MockRabbitMQService
// =========================================================================
type MockRabbitMQService struct {
	PublishFunc             func(ctx context.Context, queue string, message any) error
	ConsumeFunc             func(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error
	PublishTopicFunc        func(ctx context.Context, routingKey string, message any) error
	PublishToExchangeFunc   func(ctx context.Context, exchange, routingKey string, message any) error
	SubscribeTopicFunc      func(ctx context.Context, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error
	SubscribeToExchangeFunc func(ctx context.Context, exchange, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error
	CloseFunc               func() error
	IsHealthyFunc           func() bool
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

func (m *MockRabbitMQService) PublishTopic(ctx context.Context, routingKey string, message any) error {
	if m.PublishTopicFunc != nil {
		return m.PublishTopicFunc(ctx, routingKey, message)
	}
	return nil
}

func (m *MockRabbitMQService) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	if m.PublishToExchangeFunc != nil {
		return m.PublishToExchangeFunc(ctx, exchange, routingKey, message)
	}
	return nil
}

func (m *MockRabbitMQService) SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error {
	if m.SubscribeTopicFunc != nil {
		return m.SubscribeTopicFunc(ctx, queueName, routingKeys, handler)
	}
	return nil
}

func (m *MockRabbitMQService) SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error {
	if m.SubscribeToExchangeFunc != nil {
		return m.SubscribeToExchangeFunc(ctx, exchange, queueName, routingKeys, handler)
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

// MockTokenService
type MockTokenService struct {
	GenerateAccessTokenFunc           func(user auth.UserTokenPayload) (string, error)
	GenerateAccessTokenWithFamilyFunc func(user auth.UserTokenPayload, familyID string) (string, error)
	GenerateRefreshTokenFunc          func(user auth.UserTokenPayload) (auth.RefreshToken, error)
	ParseTokenFunc                    func(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayloadFunc     func(tokenString string) (*auth.EncryptedPayload, error)
	StoreRefreshTokenFunc             func(token auth.RefreshToken) error
	ValidateRefreshTokenFunc          func(token string) (auth.RefreshToken, error)
	RevokeRefreshTokenFunc            func(token string) error
	RotateRefreshTokenFunc            func(oldToken string) (string, string, error)
	RevokeTokenFamilyFunc             func(familyID string) error
	GetTokenFamilyIDFunc              func(rawToken string) (string, error)
}

func (m *MockTokenService) GenerateAccessToken(user auth.UserTokenPayload) (string, error) {
	if m.GenerateAccessTokenFunc != nil {
		return m.GenerateAccessTokenFunc(user)
	}
	return "mock-access-token", nil
}

func (m *MockTokenService) GenerateAccessTokenWithFamily(user auth.UserTokenPayload, familyID string) (string, error) {
	if m.GenerateAccessTokenWithFamilyFunc != nil {
		return m.GenerateAccessTokenWithFamilyFunc(user, familyID)
	}
	if m.GenerateAccessTokenFunc != nil {
		return m.GenerateAccessTokenFunc(user)
	}
	return "mock-access-token", nil
}

func (m *MockTokenService) GenerateRefreshToken(user auth.UserTokenPayload) (auth.RefreshToken, error) {
	if m.GenerateRefreshTokenFunc != nil {
		return m.GenerateRefreshTokenFunc(user)
	}
	return auth.RefreshToken{Token: "mock-refresh-token", UserUUID: user.UserUUID}, nil
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

func (m *MockTokenService) GetJWKS() auth.JWKS {
	return auth.JWKS{}
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
	CreateReviewFunc            func(ctx context.Context, userID int32, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error)
	GetProductReviewsFunc       func(ctx context.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error)
	GetProductRatingSummaryFunc func(ctx context.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error)
	GetMyReviewsFunc            func(ctx context.Context, userID int32, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error)
	UpdateReviewFunc            func(ctx context.Context, userID int32, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error)
	UpdateReviewStatusFunc      func(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error)
	DeleteReviewFunc            func(ctx context.Context, userID int32, reviewUUID uuid.UUID) error
	AdminGetAllReviewsFunc      func(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewDTO, int32, error)
	AdminReplyReviewFunc        func(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error)
	ToggleHelpfulVoteFunc       func(ctx context.Context, userID int32, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error)
	ReportReviewFunc            func(ctx context.Context, userID int32, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error
	AdminForceDeleteReviewFunc  func(ctx context.Context, reviewUUID uuid.UUID) error
	GetReviewReportsFunc        func(ctx context.Context, reviewUUID uuid.UUID) ([]v1dto.ReviewReportDTO, error)
	AdminDismissReportsFunc     func(ctx context.Context, reviewUUID uuid.UUID) error
}

func (m *MockReviewService) CreateReview(ctx context.Context, userID int32, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error) {
	if m.CreateReviewFunc != nil {
		return m.CreateReviewFunc(ctx, userID, input, imageURLs, videoURL)
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

func (m *MockReviewService) GetMyReviews(ctx context.Context, userID int32, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error) {
	if m.GetMyReviewsFunc != nil {
		return m.GetMyReviewsFunc(ctx, userID, query)
	}
	return nil, 0, nil
}

func (m *MockReviewService) UpdateReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error) {
	if m.UpdateReviewFunc != nil {
		return m.UpdateReviewFunc(ctx, userID, reviewUUID, input)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error) {
	if m.UpdateReviewStatusFunc != nil {
		return m.UpdateReviewStatusFunc(ctx, reviewUUID, input)
	}
	return v1dto.ReviewDTO{}, nil
}

func (m *MockReviewService) DeleteReview(ctx context.Context, userID int32, reviewUUID uuid.UUID) error {
	if m.DeleteReviewFunc != nil {
		return m.DeleteReviewFunc(ctx, userID, reviewUUID)
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

func (m *MockReviewService) ToggleHelpfulVote(ctx context.Context, userID int32, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error) {
	if m.ToggleHelpfulVoteFunc != nil {
		return m.ToggleHelpfulVoteFunc(ctx, userID, reviewUUID)
	}
	return v1dto.ToggleHelpfulResponseDTO{}, nil
}

func (m *MockReviewService) ReportReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error {
	if m.ReportReviewFunc != nil {
		return m.ReportReviewFunc(ctx, userID, reviewUUID, input)
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
// MockAddressService
// =========================================================================
