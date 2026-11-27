package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/services/coupon/internal/db/sqlc"
	"goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/service"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleCoupon(code string) sqlc.Coupon {
	now := time.Now()
	desc := "Sample coupon description"
	return sqlc.Coupon{
		CouponID:                10,
		CouponUuid:              uuid.New(),
		CouponCode:              code,
		CouponName:              "10% Off",
		CouponDescription:       &desc,
		CouponType:              dto.CouponTypePercentage,
		CouponDiscountValue:     utils.FloatToNumeric(10.0),
		CouponMaxDiscountAmount: utils.FloatToNumeric(50.0),
		CouponMinOrderAmount:    utils.FloatToNumeric(100.0),
		CouponUsageLimit:        nil,
		CouponUsedCount:         2,
		CouponUserUsageLimit:    0,
		CouponStartDate:         now.Add(-24 * time.Hour),
		CouponEndDate:           now.Add(24 * time.Hour),
		CouponIsActive:          true,
	}
}

func TestCouponService_CRUD(t *testing.T) {
	ctx := context.Background()

	t.Run("GetByCode success and not found", func(t *testing.T) {
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				if code == "SAVE10" {
					return sampleCoupon("SAVE10"), nil
				}
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}
		svc := service.NewCouponService(mock, nil)

		c, err := svc.GetByCode(ctx, "save10")
		require.NoError(t, err)
		assert.Equal(t, "SAVE10", c.Code)
		assert.Equal(t, "Percentage", c.TypeLabel)

		_, err = svc.GetByCode(ctx, "UNKNOWN")
		require.Error(t, err)
	})

	t.Run("GetByUUID success and not found", func(t *testing.T) {
		targetUUID := uuid.New()
		mock := &mockCouponRepo{
			getByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				if u == targetUUID {
					sc := sampleCoupon("TARGET")
					sc.CouponUuid = targetUUID
					return sc, nil
				}
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}
		svc := service.NewCouponService(mock, nil)

		c, err := svc.GetByUUID(ctx, targetUUID)
		require.NoError(t, err)
		assert.Equal(t, targetUUID.String(), c.UUID)

		_, err = svc.GetByUUID(ctx, uuid.New())
		require.Error(t, err)
	})

	t.Run("List returns items and total", func(t *testing.T) {
		mock := &mockCouponRepo{
			listFunc: func(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error) {
				return []sqlc.Coupon{sampleCoupon("C1"), sampleCoupon("C2")}, nil
			},
			countFunc: func(ctx context.Context, params sqlc.CountCouponsParams) (int64, error) {
				return 2, nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		items, total, err := svc.List(ctx, nil, nil, nil, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("Create success and duplicate code conflict", func(t *testing.T) {
		mock := &mockCouponRepo{
			createFunc: func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
				if params.CouponCode == "DUP" {
					return sqlc.Coupon{}, &pgconn.PgError{Code: "23505"}
				}
				sc := sampleCoupon(params.CouponCode)
				sc.CouponName = params.CouponName
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		input := dto.CreateCouponInput{
			Code:          "NEW10",
			Name:          "New Coupon",
			Type:          dto.CouponTypePercentage,
			DiscountValue: 10,
			StartDate:     time.Now(),
			EndDate:       time.Now().Add(48 * time.Hour),
			IsActive:      true,
		}
		created, err := svc.Create(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, "NEW10", created.Code)

		input.Code = "dup"
		_, err = svc.Create(ctx, input)
		require.Error(t, err)
		appErr, ok := err.(*utils.AppError)
		require.True(t, ok)
		assert.Equal(t, utils.ErrCodeConflict, appErr.Code)
	})

	t.Run("Update and Delete", func(t *testing.T) {
		mock := &mockCouponRepo{
			updateFunc: func(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
				sc := sampleCoupon("UPDATED")
				sc.CouponUuid = params.CouponUuid
				return sc, nil
			},
			deleteFunc: func(ctx context.Context, u uuid.UUID) error {
				return nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		targetUUID := uuid.New()
		name := "Updated Name"
		updated, err := svc.Update(ctx, targetUUID, dto.UpdateCouponInput{Name: &name})
		require.NoError(t, err)
		assert.Equal(t, targetUUID.String(), updated.UUID)

		err = svc.Delete(ctx, targetUUID)
		require.NoError(t, err)
	})
}

func TestCouponService_PreviewCoupon_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty code returns bad request", func(t *testing.T) {
		svc := service.NewCouponService(&mockCouponRepo{}, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "   "})
		require.Error(t, err)
		assert.Equal(t, utils.ErrCodeBadRequest, err.(*utils.AppError).Code)
	})

	t.Run("not found coupon returns 404", func(t *testing.T) {
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "NOTFOUND"})
		require.Error(t, err)
		assert.Equal(t, utils.ErrCodeNotFound, err.(*utils.AppError).Code)
	})

	t.Run("inactive coupon returns 400", func(t *testing.T) {
		sc := sampleCoupon("INACTIVE")
		sc.CouponIsActive = false
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "INACTIVE"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon is not active")
	})

	t.Run("expired coupon returns 400", func(t *testing.T) {
		sc := sampleCoupon("EXPIRED")
		sc.CouponStartDate = time.Now().Add(-48 * time.Hour)
		sc.CouponEndDate = time.Now().Add(-24 * time.Hour)
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "EXPIRED"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon has expired")
	})

	t.Run("not yet valid coupon returns 400", func(t *testing.T) {
		sc := sampleCoupon("FUTURE")
		sc.CouponStartDate = time.Now().Add(24 * time.Hour)
		sc.CouponEndDate = time.Now().Add(48 * time.Hour)
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "FUTURE"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon is not yet valid")
	})

	t.Run("usage limit reached returns 400", func(t *testing.T) {
		sc := sampleCoupon("MAXED")
		limit := int32(5)
		sc.CouponUsageLimit = &limit
		sc.CouponUsedCount = 5
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{Code: "MAXED"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon usage limit reached")
	})

	t.Run("user usage limit reached returns 400", func(t *testing.T) {
		sc := sampleCoupon("USERLIMIT")
		sc.CouponUserUsageLimit = 1
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
			countUserUsagesFunc: func(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error) {
				return 1, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{
			Code:     "USERLIMIT",
			UserUUID: uuid.New().String(),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "You have reached the maximum usage limit")
	})

	t.Run("order amount below min returns 400", func(t *testing.T) {
		sc := sampleCoupon("MIN100")
		sc.CouponMinOrderAmount = utils.FloatToNumeric(200.0)
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		_, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{
			Code:           "MIN100",
			SubtotalAmount: 150.0,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order amount must be at least")
	})

	t.Run("fixed amount discount exceeds order amount is capped", func(t *testing.T) {
		sc := sampleCoupon("FIXED50")
		sc.CouponType = dto.CouponTypeFixedAmount
		sc.CouponDiscountValue = utils.FloatToNumeric(50.0)
		sc.CouponMinOrderAmount = utils.FloatToNumeric(20.0)
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sc, nil
			},
		}
		svc := service.NewCouponService(mock, nil)
		prev, err := svc.PreviewCoupon(ctx, dto.PreviewCouponInput{
			Code:           "FIXED50",
			SubtotalAmount: 30.0,
		})
		require.NoError(t, err)
		assert.Equal(t, 30.0, prev.DiscountAmount, "Discount cannot exceed order total")
		assert.Equal(t, 0.0, prev.FinalTotalAmount)
	})
}
