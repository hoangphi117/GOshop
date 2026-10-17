package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/internal/utils"
	"goshop/services/coupon/internal/db/sqlc"
	"goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCouponRepo struct {
	getByCodeFunc           func(ctx context.Context, code string) (sqlc.Coupon, error)
	getByUUIDFunc           func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error)
	incrementUsedCountFunc  func(ctx context.Context, tx pgx.Tx, couponID int32) error
	decrementUsedCountFunc  func(ctx context.Context, tx pgx.Tx, couponID int32) error
	countUserUsagesFunc     func(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error)
	recordUsageFunc         func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error)
	deleteUsageByOrderFunc  func(ctx context.Context, tx pgx.Tx, params sqlc.DeleteCouponUsageByOrderParams) error
	listFunc                func(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error)
	countFunc               func(ctx context.Context, params sqlc.CountCouponsParams) (int64, error)
	createFunc              func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	updateFunc              func(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	deleteFunc              func(ctx context.Context, u uuid.UUID) error
}

func (m *mockCouponRepo) GetByCode(ctx context.Context, code string) (sqlc.Coupon, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}
	return sqlc.Coupon{}, nil
}

func (m *mockCouponRepo) GetByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (sqlc.Coupon, error) {
	return m.GetByCode(ctx, code)
}

func (m *mockCouponRepo) GetByID(ctx context.Context, id int32) (sqlc.Coupon, error) {
	return sqlc.Coupon{CouponID: id}, nil
}

func (m *mockCouponRepo) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(ctx, u)
	}
	return sqlc.Coupon{CouponUuid: u}, nil
}

func (m *mockCouponRepo) List(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockCouponRepo) Count(ctx context.Context, params sqlc.CountCouponsParams) (int64, error) {
	if m.countFunc != nil {
		return m.countFunc(ctx, params)
	}
	return 0, nil
}

func (m *mockCouponRepo) Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, params)
	}
	return sqlc.Coupon{}, nil
}

func (m *mockCouponRepo) Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, params)
	}
	return sqlc.Coupon{}, nil
}

func (m *mockCouponRepo) Delete(ctx context.Context, u uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, u)
	}
	return nil
}

func (m *mockCouponRepo) IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	if m.incrementUsedCountFunc != nil {
		return m.incrementUsedCountFunc(ctx, tx, couponID)
	}
	return nil
}

func (m *mockCouponRepo) DecrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	if m.decrementUsedCountFunc != nil {
		return m.decrementUsedCountFunc(ctx, tx, couponID)
	}
	return nil
}

func (m *mockCouponRepo) CountUserUsages(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error) {
	if m.countUserUsagesFunc != nil {
		return m.countUserUsagesFunc(ctx, params)
	}
	return 0, nil
}

func (m *mockCouponRepo) RecordUsage(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
	if m.recordUsageFunc != nil {
		return m.recordUsageFunc(ctx, tx, params)
	}
	return sqlc.CouponUsage{CouponUsageUuid: uuid.New()}, nil
}

func (m *mockCouponRepo) DeleteUsageByOrder(ctx context.Context, tx pgx.Tx, params sqlc.DeleteCouponUsageByOrderParams) error {
	if m.deleteUsageByOrderFunc != nil {
		return m.deleteUsageByOrderFunc(ctx, tx, params)
	}
	return nil
}

func TestCouponService_PreviewCoupon(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid Percentage Coupon", func(t *testing.T) {
		maxDiscount := 50.0
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:                  1,
					CouponUuid:                uuid.New(),
					CouponCode:                "SAVE10",
					CouponName:                "10% Off",
					CouponType:                dto.CouponTypePercentage,
					CouponDiscountValue:       utils.FloatToNumeric(10), // 10%
					CouponMaxDiscountAmount:   utils.FloatPointerToNumeric(&maxDiscount),
					CouponMinOrderAmount:      utils.FloatToNumeric(100),
					CouponIsActive:            true,
					CouponStartDate:           time.Now().Add(-1 * time.Hour),
					CouponEndDate:             time.Now().Add(24 * time.Hour),
					CouponUserUsageLimit:      1,
				}, nil
			},
		}

		svc := service.NewCouponService(mock, nil)
		result, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{
			Code:           "save10",
			SubtotalAmount: 200,
		})

		require.NoError(t, err)
		assert.Equal(t, 20.0, result.DiscountAmount) // 10% of 200 = 20
		assert.Equal(t, 180.0, result.FinalTotalAmount)
	})

	t.Run("Below Min Order Amount", func(t *testing.T) {
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:             1,
					CouponCode:           "MIN500",
					CouponMinOrderAmount: utils.FloatToNumeric(500),
					CouponIsActive:       true,
					CouponStartDate:      time.Now().Add(-1 * time.Hour),
					CouponEndDate:        time.Now().Add(24 * time.Hour),
				}, nil
			},
		}

		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{
			Code:           "MIN500",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order amount must be at least 500.00")
	})
}

func TestCouponService_ReserveCoupon(t *testing.T) {
	ctx := context.Background()

	t.Run("Success Reserve", func(t *testing.T) {
		mock := &mockCouponRepo{
			incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return nil
			},
			recordUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{CouponUsageUuid: uuid.New()}, nil
			},
		}

		svc := service.NewCouponService(mock, nil)
		usageUUID, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{
			CouponID: 1,
			UserID:   10,
			OrderID:  100,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, usageUUID)
	})

	t.Run("Idempotency Conflict (23505)", func(t *testing.T) {
		mock := &mockCouponRepo{
			incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return nil
			},
			recordUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{}, &pgconn.PgError{Code: "23505"}
			},
		}

		svc := service.NewCouponService(mock, nil)
		_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{
			CouponID: 1,
			UserID:   10,
			OrderID:  100,
		})

		require.Error(t, err)
		appErr, ok := err.(*utils.AppError)
		require.True(t, ok)
		assert.Equal(t, utils.ErrCodeConflict, appErr.Code)
		assert.Contains(t, err.Error(), "already been used")
	})
}
