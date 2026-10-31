// Package servicetest holds test doubles for the review service layer.
// ponytail: local copies of the monolith's mocks (internal/testutils) so the
// service does not depend on monolith-internal packages in tests.
package servicetest

import (
	"context"
	"errors"
	"time"

	catalogcontract "goshop/services/catalog/contract"
	identitycontract "goshop/services/identity/contract"
	ordercontract "goshop/services/order/contract"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewdto "goshop/services/review/internal/dto"
	reviewrepo "goshop/services/review/internal/repository"
	reviewsvc "goshop/services/review/internal/service"
	"goshop/shared/pkg/cache"

	"github.com/google/uuid"
)

var (
	_ reviewrepo.ReviewRepository = (*MockReviewRepository)(nil)
	_ reviewsvc.ProductCatalog    = (*MockProductCatalog)(nil)
	_ reviewsvc.PurchaseVerifier  = (*MockPurchaseVerifier)(nil)
	_ reviewsvc.UserReader        = (*MockUserReader)(nil)
	_ cache.RedisCacheService     = (*MockRedisCacheService)(nil)
)

type MockReviewRepository struct {
	CreateFunc                  func(ctx context.Context, params reviewsqlc.CreateReviewParams) (reviewsqlc.ProductReview, error)
	CheckExistsFunc             func(ctx context.Context, userID, productID int32, orderUUID uuid.UUID) (bool, error)
	GetByProductFunc            func(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error)
	CountByProductFunc          func(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error)
	GetRatingSummaryFunc        func(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingSummaryRow, error)
	GetRatingAggregateFunc      func(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingAggregateRow, error)
	GetByUserIDFunc             func(ctx context.Context, userID int32, limit, offset int32) ([]reviewsqlc.GetReviewsByUserIDRow, error)
	CountByUserIDFunc           func(ctx context.Context, userID int32) (int64, error)
	GetByUUIDFunc               func(ctx context.Context, reviewUUID uuid.UUID) (reviewsqlc.ProductReview, error)
	UpdateFunc                  func(ctx context.Context, params reviewsqlc.UpdateReviewParams) (reviewsqlc.ProductReview, error)
	DeleteFunc                  func(ctx context.Context, reviewID int32) error
	HasReportedFunc             func(ctx context.Context, userID, reviewID int32) (bool, error)
	CreateReportFunc            func(ctx context.Context, params reviewsqlc.InsertReviewReportParams) error
	CountReportsFunc            func(ctx context.Context, reviewID int32) (int64, error)
	GetReportsByReviewIDFunc    func(ctx context.Context, reviewID int32) ([]reviewsqlc.GetReviewReportsByReviewIDRow, error)
	DeleteReportsByReviewIDFunc func(ctx context.Context, reviewID int32) error
	HasHelpfulVoteFunc          func(ctx context.Context, userID, reviewID int32) (bool, error)
	AddHelpfulVoteFunc          func(ctx context.Context, userID, reviewID int32) (int32, error)
	RemoveHelpfulVoteFunc       func(ctx context.Context, userID, reviewID int32) (int32, error)
	AdminGetAllFunc             func(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string, orderBy, sort string, limit, offset int32) ([]reviewdto.AdminReviewRow, error)
	AdminCountFunc              func(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error)
	UpdateStatusFunc            func(ctx context.Context, reviewID int32, status int16) (reviewsqlc.ProductReview, error)
	ReplyFunc                   func(ctx context.Context, reviewID int32, reply string) (reviewsqlc.ProductReview, error)
}

func (m *MockReviewRepository) Create(ctx context.Context, params reviewsqlc.CreateReviewParams) (reviewsqlc.ProductReview, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return reviewsqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) CheckExists(ctx context.Context, userID, productID int32, orderUUID uuid.UUID) (bool, error) {
	if m.CheckExistsFunc != nil {
		return m.CheckExistsFunc(ctx, userID, productID, orderUUID)
	}
	return false, nil
}

func (m *MockReviewRepository) GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error) {
	if m.GetByProductFunc != nil {
		return m.GetByProductFunc(ctx, productID, rating, hasMedia, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error) {
	if m.CountByProductFunc != nil {
		return m.CountByProductFunc(ctx, productID, rating, hasMedia)
	}
	return 0, nil
}

func (m *MockReviewRepository) GetRatingSummary(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingSummaryRow, error) {
	if m.GetRatingSummaryFunc != nil {
		return m.GetRatingSummaryFunc(ctx, productID)
	}
	return reviewsqlc.GetProductRatingSummaryRow{}, nil
}

func (m *MockReviewRepository) GetRatingAggregate(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
	if m.GetRatingAggregateFunc != nil {
		return m.GetRatingAggregateFunc(ctx, productID)
	}
	return reviewsqlc.GetProductRatingAggregateRow{}, nil
}

func (m *MockReviewRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]reviewsqlc.GetReviewsByUserIDRow, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	if m.CountByUserIDFunc != nil {
		return m.CountByUserIDFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockReviewRepository) GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (reviewsqlc.ProductReview, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, reviewUUID)
	}
	return reviewsqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) Update(ctx context.Context, params reviewsqlc.UpdateReviewParams) (reviewsqlc.ProductReview, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return reviewsqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) Delete(ctx context.Context, reviewID int32) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, reviewID)
	}
	return nil
}

func (m *MockReviewRepository) HasReported(ctx context.Context, userID, reviewID int32) (bool, error) {
	if m.HasReportedFunc != nil {
		return m.HasReportedFunc(ctx, userID, reviewID)
	}
	return false, nil
}

func (m *MockReviewRepository) CreateReport(ctx context.Context, params reviewsqlc.InsertReviewReportParams) error {
	if m.CreateReportFunc != nil {
		return m.CreateReportFunc(ctx, params)
	}
	return nil
}

func (m *MockReviewRepository) CountReports(ctx context.Context, reviewID int32) (int64, error) {
	if m.CountReportsFunc != nil {
		return m.CountReportsFunc(ctx, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) GetReportsByReviewID(ctx context.Context, reviewID int32) ([]reviewsqlc.GetReviewReportsByReviewIDRow, error) {
	if m.GetReportsByReviewIDFunc != nil {
		return m.GetReportsByReviewIDFunc(ctx, reviewID)
	}
	return nil, nil
}

func (m *MockReviewRepository) DeleteReportsByReviewID(ctx context.Context, reviewID int32) error {
	if m.DeleteReportsByReviewIDFunc != nil {
		return m.DeleteReportsByReviewIDFunc(ctx, reviewID)
	}
	return nil
}

func (m *MockReviewRepository) HasHelpfulVote(ctx context.Context, userID, reviewID int32) (bool, error) {
	if m.HasHelpfulVoteFunc != nil {
		return m.HasHelpfulVoteFunc(ctx, userID, reviewID)
	}
	return false, nil
}

func (m *MockReviewRepository) AddHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error) {
	if m.AddHelpfulVoteFunc != nil {
		return m.AddHelpfulVoteFunc(ctx, userID, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) RemoveHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error) {
	if m.RemoveHelpfulVoteFunc != nil {
		return m.RemoveHelpfulVoteFunc(ctx, userID, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string, orderBy, sort string, limit, offset int32) ([]reviewdto.AdminReviewRow, error) {
	if m.AdminGetAllFunc != nil {
		return m.AdminGetAllFunc(ctx, status, rating, productUUID, search, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) AdminCount(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error) {
	if m.AdminCountFunc != nil {
		return m.AdminCountFunc(ctx, status, rating, productUUID, search)
	}
	return 0, nil
}

func (m *MockReviewRepository) UpdateStatus(ctx context.Context, reviewID int32, status int16) (reviewsqlc.ProductReview, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, reviewID, status)
	}
	return reviewsqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) Reply(ctx context.Context, reviewID int32, reply string) (reviewsqlc.ProductReview, error) {
	if m.ReplyFunc != nil {
		return m.ReplyFunc(ctx, reviewID, reply)
	}
	return reviewsqlc.ProductReview{}, nil
}

// MockProductCatalog stubs the catalog gRPC port.
type MockProductCatalog struct {
	GetProductByUUIDFunc func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error)
	GetProductsBatchFunc func(ctx context.Context, uuids []string, ids []int32) ([]catalogcontract.ProductItemInfo, error)
}

func (m *MockProductCatalog) GetProductByUUID(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
	if m.GetProductByUUIDFunc != nil {
		return m.GetProductByUUIDFunc(ctx, u)
	}
	return nil, nil
}

func (m *MockProductCatalog) GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]catalogcontract.ProductItemInfo, error) {
	if m.GetProductsBatchFunc != nil {
		return m.GetProductsBatchFunc(ctx, uuids, ids)
	}
	return nil, nil
}

// MockPurchaseVerifier stubs the order-service gRPC port.
type MockPurchaseVerifier struct {
	VerifyPurchaseFunc func(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*ordercontract.PurchaseVerification, error)
}

func (m *MockPurchaseVerifier) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*ordercontract.PurchaseVerification, error) {
	if m.VerifyPurchaseFunc != nil {
		res, err := m.VerifyPurchaseFunc(ctx, orderUUID, userID, productID, userUUID, productUUID)
		if err != nil || res == nil {
			return nil, err
		}
		return &ordercontract.PurchaseVerification{
			OrderID:       res.OrderID,
			OrderUUID:     res.OrderUUID,
			IsPurchased:   res.IsPurchased,
			CanReview:     res.CanReview,
			CustomerName:  res.CustomerName,
			CustomerEmail: res.CustomerEmail,
		}, nil
	}
	return &ordercontract.PurchaseVerification{}, nil
}

// MockUserReader stubs the identity gRPC port.
type MockUserReader struct {
	GetUserByIDFunc func(ctx context.Context, userID int32) (*identitycontract.InternalUserDTO, error)
}

func (m *MockUserReader) GetUserByID(ctx context.Context, userID int32) (*identitycontract.InternalUserDTO, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, userID)
	}
	return nil, nil
}

// MockRedisCacheService is an in-memory no-op cache (always misses).
type MockRedisCacheService struct {
	GetFunc   func(key string, dest any) error
	SetFunc   func(key string, value any, ttl time.Duration) error
	DelFunc   func(key string) error
	ClearFunc func(pattern string) error
}

func (m *MockRedisCacheService) Set(key string, value any, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(key, value, ttl)
	}
	return nil
}

func (m *MockRedisCacheService) SetNX(key string, value any, ttl time.Duration) (bool, error) {
	return false, nil
}

func (m *MockRedisCacheService) Get(key string, dest any) error {
	if m.GetFunc != nil {
		return m.GetFunc(key, dest)
	}
	return errors.New("cache miss")
}

func (m *MockRedisCacheService) Clear(pattern string) error {
	if m.ClearFunc != nil {
		return m.ClearFunc(pattern)
	}
	return nil
}

func (m *MockRedisCacheService) Exists(key string) (bool, error) { return false, nil }

func (m *MockRedisCacheService) Del(key string) error {
	if m.DelFunc != nil {
		return m.DelFunc(key)
	}
	return nil
}

func (m *MockRedisCacheService) Incr(key string) (int64, error) { return 0, nil }

func (m *MockRedisCacheService) Expire(key string, ttl time.Duration) error { return nil }

func (m *MockRedisCacheService) TTL(key string) (time.Duration, error) { return 0, nil }
