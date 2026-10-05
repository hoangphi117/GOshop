package v1service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCouponService_CreateCoupon(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	startDateStr := now.Add(-1 * time.Hour).Format(time.RFC3339)
	endDateStr := now.Add(24 * time.Hour).Format(time.RFC3339)

	t.Run("Case 1: Coupon Code Already Exists -> Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{CouponCode: "SAVE20"}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:          "save20",
			Name:          "Save 20%",
			Type:          v1dto.CouponTypePercentage,
			DiscountValue: 20,
			StartDate:     startDateStr,
			EndDate:       endDateStr,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 2: Invalid Date Range (End Date in Past) -> Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:          "EXPIRED",
			Name:          "Expired Coupon",
			Type:          v1dto.CouponTypePercentage,
			DiscountValue: 10,
			StartDate:     now.Add(-48 * time.Hour).Format(time.RFC3339),
			EndDate:       now.Add(-24 * time.Hour).Format(time.RFC3339),
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "future")
	})

	t.Run("Case 3: Invalid Percentage Discount Value -> Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:          "INVALIDPERCENT",
			Name:          "Invalid %",
			Type:          v1dto.CouponTypePercentage,
			DiscountValue: 150, // Invalid > 100
			StartDate:     startDateStr,
			EndDate:       endDateStr,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "between 1 and 100")
	})

	t.Run("Case 4: Fixed Amount with Max Discount Specified -> Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		maxDisc := 50.0
		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:              "FIXEDWITHMAX",
			Name:              "Fixed With Max",
			Type:              v1dto.CouponTypeFixedAmount,
			DiscountValue:     20,
			MaxDiscountAmount: &maxDisc,
			StartDate:         startDateStr,
			EndDate:           endDateStr,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "not applicable")
	})

	t.Run("Case 5: Database Unique Constraint Violation (23505) -> Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, &pgconn.PgError{Code: "23505"}
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:          "UNIQUEPG",
			Name:          "Unique PG",
			Type:          v1dto.CouponTypePercentage,
			DiscountValue: 20,
			StartDate:     startDateStr,
			EndDate:       endDateStr,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 6: Happy Path -> Creates and Returns CouponDTO", func(t *testing.T) {
		couponUUID := uuid.New()
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
				assert.Equal(t, "SUMMER20", params.CouponCode)
				assert.Equal(t, "Summer 20%", params.CouponName)
				return sqlc.Coupon{
					CouponID:            1,
					CouponUuid:          couponUUID,
					CouponCode:          params.CouponCode,
					CouponName:          params.CouponName,
					CouponType:          params.CouponType,
					CouponDiscountValue: params.CouponDiscountValue,
					CouponIsActive:      true,
				}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupon, err := svc.CreateCoupon(ctx, v1dto.CreateCouponInput{
			Code:          "summer20",
			Name:          "Summer 20%",
			Type:          v1dto.CouponTypePercentage,
			DiscountValue: 20,
			StartDate:     startDateStr,
			EndDate:       endDateStr,
		})

		require.NoError(t, err)
		assert.Equal(t, couponUUID.String(), coupon.UUID)
		assert.Equal(t, "SUMMER20", coupon.Code)
		assert.Equal(t, 20.0, coupon.DiscountValue)
	})
}

func TestCouponService_GetAllCoupons(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()

	t.Run("Case 1: Repo GetAll Error -> Returns 500 Internal Error", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error) {
				return nil, errors.New("db error")
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, _, err := svc.GetAllCoupons(ctx, "", "", "", nil, nil, 1, 10, false)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 2: Repo CountCoupons Error -> Returns 500 Internal Error", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error) {
				return []sqlc.Coupon{}, nil
			},
			CountCouponsFunc: func(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error) {
				return 0, errors.New("count db error")
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, _, err := svc.GetAllCoupons(ctx, "", "", "", nil, nil, 1, 10, false)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path -> Returns Coupons and Total Count", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error) {
				return []sqlc.Coupon{
					{
						CouponID:            10,
						CouponUuid:          couponUUID,
						CouponCode:          "WELCOME10",
						CouponName:          "Welcome 10%",
						CouponType:          1,
						CouponDiscountValue: utils.FloatToNumeric(10.0),
						CouponIsActive:      true,
					},
				}, nil
			},
			CountCouponsFunc: func(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupons, total, err := svc.GetAllCoupons(ctx, "", "", "", nil, nil, 1, 10, false)

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, coupons, 1)
		assert.Equal(t, "WELCOME10", coupons[0].Code)
	})
}

func TestCouponService_GetCouponByUUID(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()

	t.Run("Case 1: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.GetCouponByUUID(ctx, couponUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns CouponDTO", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:            5,
					CouponUuid:          couponUUID,
					CouponCode:          "SPECIAL50",
					CouponName:          "Special 50%",
					CouponType:          1,
					CouponDiscountValue: utils.FloatToNumeric(50.0),
					CouponIsActive:      true,
				}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupon, err := svc.GetCouponByUUID(ctx, couponUUID)

		require.NoError(t, err)
		assert.Equal(t, couponUUID.String(), coupon.UUID)
		assert.Equal(t, "SPECIAL50", coupon.Code)
		assert.Equal(t, 50.0, coupon.DiscountValue)
	})
}

func TestCouponService_UpdateCoupon(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()
	startDate := time.Now().Add(-24 * time.Hour)

	t.Run("Case 1: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.UpdateCoupon(ctx, couponUUID, v1dto.UpdateCouponInput{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Fixed Amount with Max Discount -> Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:        1,
					CouponUuid:      couponUUID,
					CouponType:      v1dto.CouponTypeFixedAmount,
					CouponStartDate: startDate,
				}, nil
			},
		}

		maxDisc := 10.0
		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.UpdateCoupon(ctx, couponUUID, v1dto.UpdateCouponInput{
			MaxDiscountAmount: &maxDisc,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "not applicable")
	})

	t.Run("Case 3: Usage Limit Less Than Used Count -> Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:        1,
					CouponUuid:      couponUUID,
					CouponType:      v1dto.CouponTypePercentage,
					CouponStartDate: startDate,
					CouponUsedCount: 15,
				}, nil
			},
		}

		newLimit := int32(10) // 10 < 15
		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.UpdateCoupon(ctx, couponUUID, v1dto.UpdateCouponInput{
			UsageLimit: &newLimit,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Usage limit cannot be less")
	})

	t.Run("Case 4: Happy Path -> Updates and Returns CouponDTO", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:        1,
					CouponUuid:      couponUUID,
					CouponCode:      "PROMO10",
					CouponName:      "Old Name",
					CouponType:      v1dto.CouponTypePercentage,
					CouponStartDate: startDate,
					CouponUsedCount: 5,
				}, nil
			},
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
				assert.Equal(t, couponUUID, params.CouponUuid)
				assert.Equal(t, "Updated Name", *params.CouponName)
				return sqlc.Coupon{
					CouponID:            1,
					CouponUuid:          couponUUID,
					CouponCode:          "PROMO10",
					CouponName:          "Updated Name",
					CouponType:          v1dto.CouponTypePercentage,
					CouponDiscountValue: utils.FloatToNumeric(15.0),
					CouponIsActive:      true,
				}, nil
			},
		}

		newName := "Updated Name"
		newDisc := 15.0
		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupon, err := svc.UpdateCoupon(ctx, couponUUID, v1dto.UpdateCouponInput{
			Name:          &newName,
			DiscountValue: &newDisc,
		})

		require.NoError(t, err)
		assert.Equal(t, "Updated Name", coupon.Name)
		assert.Equal(t, 15.0, coupon.DiscountValue)
	})
}

func TestCouponService_SoftDeleteCoupon(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()

	t.Run("Case 1: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.SoftDeleteCoupon(ctx, couponUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Soft Deletes and Returns CouponDTO", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:   1,
					CouponUuid: couponUUID,
					CouponCode: "DELETED",
				}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupon, err := svc.SoftDeleteCoupon(ctx, couponUUID)

		require.NoError(t, err)
		assert.Equal(t, couponUUID.String(), coupon.UUID)
	})
}

func TestCouponService_RestoreCoupon(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()

	t.Run("Case 1: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		_, err := svc.RestoreCoupon(ctx, couponUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Restores and Returns CouponDTO", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:   1,
					CouponUuid: couponUUID,
					CouponCode: "RESTORED",
				}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		coupon, err := svc.RestoreCoupon(ctx, couponUUID)

		require.NoError(t, err)
		assert.Equal(t, couponUUID.String(), coupon.UUID)
	})
}

func TestCouponService_TrashCoupon(t *testing.T) {
	ctx := context.Background()
	couponUUID := uuid.New()

	t.Run("Case 1: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		err := svc.TrashCoupon(ctx, couponUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Trashes Coupon Successfully", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:   1,
					CouponUuid: couponUUID,
				}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		err := svc.TrashCoupon(ctx, couponUUID)

		require.NoError(t, err)
	})
}

func TestCouponService_PreviewCoupon(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	now := time.Now()
	startDate := now.Add(-2 * time.Hour)
	endDate := now.Add(48 * time.Hour)

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(nil, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "SAVE10",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Coupon Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "NONEXISTENT",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Coupon Is Inactive -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:       1,
					CouponCode:     "INACTIVE",
					CouponIsActive: false,
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "INACTIVE",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "not active")
	})

	t.Run("Case 4: Coupon Expired -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:        1,
					CouponCode:      "EXPIRED",
					CouponIsActive:  true,
					CouponStartDate: now.Add(-48 * time.Hour),
					CouponEndDate:   now.Add(-2 * time.Hour), // Expired
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "EXPIRED",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Case 5: Subtotal Below Minimum Order Amount -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:             1,
					CouponCode:           "MINORDER",
					CouponIsActive:       true,
					CouponStartDate:      startDate,
					CouponEndDate:        endDate,
					CouponMinOrderAmount: utils.FloatToNumeric(200.0),
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "MINORDER",
			SubtotalAmount: 150, // 150 < 200
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Subtotal amount must be at least")
	})

	t.Run("Case 6: User Usage Limit Reached -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:             1,
					CouponCode:           "LIMIT1",
					CouponIsActive:       true,
					CouponStartDate:      startDate,
					CouponEndDate:        endDate,
					CouponUserUsageLimit: 1,
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 1, nil // User already used once
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		_, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "LIMIT1",
			SubtotalAmount: 100,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "reached your usage limit")
	})

	t.Run("Case 7: Happy Path Percentage with Max Discount Cap -> Calculates Correctly", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		maxDiscount := 30.0
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:                1,
					CouponCode:              "SAVE50MAX30",
					CouponName:              "Save 50% Up to $30",
					CouponType:              v1dto.CouponTypePercentage,
					CouponDiscountValue:     utils.FloatToNumeric(50.0),
					CouponMaxDiscountAmount: utils.FloatPointerToNumeric(&maxDiscount),
					CouponMinOrderAmount:    utils.FloatToNumeric(50.0),
					CouponUserUsageLimit:    5,
					CouponStartDate:         startDate,
					CouponEndDate:           endDate,
					CouponIsActive:          true,
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		preview, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "SAVE50MAX30",
			SubtotalAmount: 200, // 50% of 200 = 100 -> capped at 30
		})

		require.NoError(t, err)
		assert.Equal(t, "SAVE50MAX30", preview.CouponCode)
		assert.Equal(t, 200.0, preview.SubtotalAmount)
		assert.Equal(t, 30.0, preview.DiscountAmount)
		assert.Equal(t, 170.0, preview.FinalTotalAmount)
	})

	t.Run("Case 8: Happy Path Fixed Amount -> Calculates Correctly", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:             1,
					CouponCode:           "FLAT25",
					CouponName:           "Flat $25 Off",
					CouponType:           v1dto.CouponTypeFixedAmount,
					CouponDiscountValue:  utils.FloatToNumeric(25.0),
					CouponMinOrderAmount: utils.FloatToNumeric(50.0),
					CouponUserUsageLimit: 2,
					CouponStartDate:      startDate,
					CouponEndDate:        endDate,
					CouponIsActive:       true,
				}, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, mockUserRepo, nil)
		preview, err := svc.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
			Code:           "FLAT25",
			SubtotalAmount: 100,
		})

		require.NoError(t, err)
		assert.Equal(t, "FLAT25", preview.CouponCode)
		assert.Equal(t, 100.0, preview.SubtotalAmount)
		assert.Equal(t, 25.0, preview.DiscountAmount)
		assert.Equal(t, 75.0, preview.FinalTotalAmount)
	})
}

func TestCouponService_LockAndApplyCoupon(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Increments count and records user usage", func(t *testing.T) {
		incrementCalled := false
		recordUsageCalled := false

		mockRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				incrementCalled = true
				assert.Equal(t, int32(10), couponID)
				return nil
			},
			RecordUserUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				recordUsageCalled = true
				assert.Equal(t, int32(10), params.CouponUsageCouponID)
				assert.Equal(t, int32(1), params.CouponUsageUserID)
				assert.Equal(t, int32(100), *params.CouponUsageOrderID)
				return sqlc.CouponUsage{}, nil
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		err := svc.LockAndApplyCoupon(ctx, 10, 1, 100)

		require.NoError(t, err)
		assert.True(t, incrementCalled)
		assert.True(t, recordUsageCalled)
	})

	t.Run("Coupon limit reached: Increment returns ErrNoRows -> 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return pgx.ErrNoRows
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		err := svc.LockAndApplyCoupon(ctx, 10, 1, 100)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Coupon already used: RecordUserUsage returns 23505 unique violation -> 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return nil
			},
			RecordUserUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{}, &pgconn.PgError{Code: "23505"}
			},
		}

		svc := v1service.NewCouponService(mockRepo, nil, nil)
		err := svc.LockAndApplyCoupon(ctx, 10, 1, 100)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})
}

