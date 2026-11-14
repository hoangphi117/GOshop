package testutils

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/mail"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/shipping"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ messaging.Broker          = (*MockBrokerService)(nil)
	_ shipping.ShippingProvider = (*MockShippingProvider)(nil)
	_ auth.TokenService         = (*MockTokenService)(nil)
	_ mail.EmailProviderService = (*MockEmailProviderService)(nil)
	_ cache.RedisCacheService   = (*MockRedisCacheService)(nil)
)

// =========================================================================
// MockBrokerService is the in-memory messaging.Broker used by unit tests.
// =========================================================================
type MockBrokerService struct {
	PublishFunc             func(ctx context.Context, queue string, message any) error
	ConsumeFunc             func(ctx context.Context, queue string, handler messaging.Handler) error
	PublishTopicFunc        func(ctx context.Context, routingKey string, message any) error
	PublishToExchangeFunc   func(ctx context.Context, exchange, routingKey string, message any) error
	SubscribeTopicFunc      func(ctx context.Context, queueName string, routingKeys []string, handler messaging.Handler) error
	SubscribeToExchangeFunc func(ctx context.Context, exchange, queueName string, routingKeys []string, handler messaging.Handler) error
	CloseFunc               func() error
	IsHealthyFunc           func() bool
}

func (m *MockBrokerService) Publish(ctx context.Context, queue string, message any) error {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, queue, message)
	}
	return nil
}

func (m *MockBrokerService) Consume(ctx context.Context, queue string, handler messaging.Handler) error {
	if m.ConsumeFunc != nil {
		return m.ConsumeFunc(ctx, queue, handler)
	}
	return nil
}

func (m *MockBrokerService) PublishTopic(ctx context.Context, routingKey string, message any) error {
	if m.PublishTopicFunc != nil {
		return m.PublishTopicFunc(ctx, routingKey, message)
	}
	return nil
}

func (m *MockBrokerService) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	if m.PublishToExchangeFunc != nil {
		return m.PublishToExchangeFunc(ctx, exchange, routingKey, message)
	}
	return nil
}

func (m *MockBrokerService) SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler messaging.Handler) error {
	if m.SubscribeTopicFunc != nil {
		return m.SubscribeTopicFunc(ctx, queueName, routingKeys, handler)
	}
	return nil
}

func (m *MockBrokerService) SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler messaging.Handler) error {
	if m.SubscribeToExchangeFunc != nil {
		return m.SubscribeToExchangeFunc(ctx, exchange, queueName, routingKeys, handler)
	}
	return nil
}

func (m *MockBrokerService) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockBrokerService) IsHealthy() bool {
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
	RevokeUserSessionsFunc            func(userUUID string) error
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

func (m *MockTokenService) RevokeUserSessions(userUUID string) error {
	if m.RevokeUserSessionsFunc != nil {
		return m.RevokeUserSessionsFunc(userUUID)
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
// =========================================================================
