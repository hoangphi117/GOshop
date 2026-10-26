package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"goshop/pkg/utils"
	"goshop/services/coupon/internal/db/sqlc"
	"goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCouponRepo struct {
	getByCodeFunc          func(ctx context.Context, code string) (sqlc.Coupon, error)
	getByUUIDFunc          func(ctx context.Context, u uuid.UUID) (sqlc.Coupon, error)
	getByIDForUpdateFunc   func(ctx context.Context, tx pgx.Tx, id int32) (sqlc.Coupon, error)
	incrementUsedCountFunc func(ctx context.Context, tx pgx.Tx, couponID int32) error
	decrementUsedCountFunc func(ctx context.Context, tx pgx.Tx, couponID int32) error
	countUserUsagesFunc    func(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error)
	upsertUsageFunc        func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error)
	releaseUsageFunc       func(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error)
	listFunc               func(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error)
	countFunc              func(ctx context.Context, params sqlc.CountCouponsParams) (int64, error)
	createFunc             func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	updateFunc             func(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	deleteFunc             func(ctx context.Context, u uuid.UUID) error
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

func (m *mockCouponRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int32) (sqlc.Coupon, error) {
	if m.getByIDForUpdateFunc != nil {
		return m.getByIDForUpdateFunc(ctx, tx, id)
	}
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

func (m *mockCouponRepo) UpsertUsage(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
	if m.upsertUsageFunc != nil {
		return m.upsertUsageFunc(ctx, tx, params)
	}
	return sqlc.UpsertCouponUsageRow{
		CouponUsageUuid:     uuid.New(),
		CouponUsageCouponID: params.CouponID,
		CouponUsageStatus:   dto.CouponUsageStatusReserved,
		Inserted:            true,
	}, nil
}

func (m *mockCouponRepo) ReleaseUsage(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
	if m.releaseUsageFunc != nil {
		return m.releaseUsageFunc(ctx, tx, params)
	}
	return sqlc.CouponUsage{}, pgx.ErrNoRows
}

func TestCouponService_PreviewCoupon(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid Percentage Coupon", func(t *testing.T) {
		maxDiscount := 50.0
		mock := &mockCouponRepo{
			getByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponID:                1,
					CouponUuid:              uuid.New(),
					CouponCode:              "SAVE10",
					CouponName:              "10% Off",
					CouponType:              dto.CouponTypePercentage,
					CouponDiscountValue:     utils.FloatToNumeric(10), // 10%
					CouponMaxDiscountAmount: utils.FloatPointerToNumeric(&maxDiscount),
					CouponMinOrderAmount:    utils.FloatToNumeric(100),
					CouponIsActive:          true,
					CouponStartDate:         time.Now().Add(-1 * time.Hour),
					CouponEndDate:           time.Now().Add(24 * time.Hour),
					CouponUserUsageLimit:    1,
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
	orderUUID := uuid.New().String()

	t.Run("Success Reserve", func(t *testing.T) {
		mock := &mockCouponRepo{}
		svc := service.NewCouponService(mock, nil)

		usageUUID, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{
			CouponID:  1,
			UserID:    10,
			OrderID:   100,
			OrderUUID: orderUUID,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, usageUUID)
	})

	t.Run("Duplicate Reserve does not consume quota twice", func(t *testing.T) {
		increments := 0
		reserved := false
		existing := uuid.New()

		mock := &mockCouponRepo{
			incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				increments++
				return nil
			},
			upsertUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
				// Repository semantics: second call returns the existing row.
				if reserved {
					return sqlc.UpsertCouponUsageRow{
						CouponUsageUuid:   existing,
						CouponUsageStatus: dto.CouponUsageStatusReserved,
						Inserted:          false,
					}, nil
				}
				reserved = true
				return sqlc.UpsertCouponUsageRow{
					CouponUsageUuid:   existing,
					CouponUsageStatus: dto.CouponUsageStatusReserved,
					Inserted:          true,
				}, nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		input := dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderID: 100, OrderUUID: orderUUID}
		first, err := svc.ReserveCoupon(ctx, input)
		require.NoError(t, err)
		second, err := svc.ReserveCoupon(ctx, input)
		require.NoError(t, err)

		assert.Equal(t, first, second, "retry returns the same reservation identity")
		assert.Equal(t, 1, increments, "quota consumed exactly once")
	})

	t.Run("Reserve on released reservation is rejected", func(t *testing.T) {
		increments := 0
		mock := &mockCouponRepo{
			incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				increments++
				return nil
			},
			upsertUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
				return sqlc.UpsertCouponUsageRow{
					CouponUsageUuid:   uuid.New(),
					CouponUsageStatus: dto.CouponUsageStatusReleased,
					Inserted:          false,
				}, nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderUUID: orderUUID})
		require.Error(t, err)
		appErr, ok := err.(*utils.AppError)
		require.True(t, ok)
		assert.Equal(t, utils.ErrCodeConflict, appErr.Code)
		assert.Zero(t, increments)
	})

	t.Run("Usage limit reached rolls back reservation", func(t *testing.T) {
		mock := &mockCouponRepo{
			incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return pgx.ErrNoRows
			},
		}
		svc := service.NewCouponService(mock, nil)

		_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderUUID: orderUUID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum total usage limit")
	})

	t.Run("Missing order uuid is rejected", func(t *testing.T) {
		svc := service.NewCouponService(&mockCouponRepo{}, nil)

		_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderID: 100})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order uuid is required")
	})

	t.Run("Invalid order uuid is rejected", func(t *testing.T) {
		svc := service.NewCouponService(&mockCouponRepo{}, nil)

		_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderUUID: "not-a-uuid"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid order uuid")
	})
}

func TestCouponService_ReleaseCoupon(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New().String()

	t.Run("Release refunds quota once", func(t *testing.T) {
		decrements := 0
		released := false
		mock := &mockCouponRepo{
			releaseUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
				if released {
					return sqlc.CouponUsage{}, pgx.ErrNoRows
				}
				released = true
				return sqlc.CouponUsage{CouponUsageCouponID: 1}, nil
			},
			decrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				decrements++
				return nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
		require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
		assert.Equal(t, 1, decrements, "duplicate release refunds nothing")
	})

	t.Run("Release before reserve refunds nothing", func(t *testing.T) {
		decrements := 0
		mock := &mockCouponRepo{
			releaseUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{}, pgx.ErrNoRows
			},
			decrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				decrements++
				return nil
			},
		}
		svc := service.NewCouponService(mock, nil)

		require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
		assert.Zero(t, decrements)
	})

	t.Run("Decrement failure surfaces instead of being swallowed", func(t *testing.T) {
		mock := &mockCouponRepo{
			releaseUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{CouponUsageCouponID: 1}, nil
			},
			decrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
				return assert.AnError
			},
		}
		svc := service.NewCouponService(mock, nil)

		err := svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to decrement coupon usage count")
	})

	t.Run("Missing order uuid is rejected", func(t *testing.T) {
		svc := service.NewCouponService(&mockCouponRepo{}, nil)

		err := svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order id or order uuid is required")
	})
}

// TestCouponService_ReserveReleaseInterleaving covers reversed delivery order:
// a release that lands before its reserve must not corrupt later quota.
func TestCouponService_ReserveReleaseInterleaving(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New().String()

	var (
		used      int32
		released  bool
		inserted  bool
		reserveIn int
	)

	mock := &mockCouponRepo{
		incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
			used++
			return nil
		},
		decrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
			used--
			return nil
		},
		upsertUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
			reserveIn++
			if !inserted {
				inserted = true
				return sqlc.UpsertCouponUsageRow{CouponUsageUuid: uuid.New(), CouponUsageStatus: dto.CouponUsageStatusReserved, Inserted: true}, nil
			}
			status := dto.CouponUsageStatusReserved
			if released {
				status = dto.CouponUsageStatusReleased
			}
			return sqlc.UpsertCouponUsageRow{CouponUsageUuid: uuid.New(), CouponUsageStatus: status, Inserted: false}, nil
		},
		releaseUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
			if !inserted || released {
				return sqlc.CouponUsage{}, pgx.ErrNoRows
			}
			released = true
			return sqlc.CouponUsage{CouponUsageCouponID: 1}, nil
		},
	}
	svc := service.NewCouponService(mock, nil)

	// Release arrives first: no reservation exists, nothing to refund.
	require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
	assert.Equal(t, int32(0), used, "release before reserve must not decrement")

	// Reserve now consumes quota exactly once.
	_, err := svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderUUID: orderUUID})
	require.NoError(t, err)
	assert.Equal(t, int32(1), used)

	// Redelivered release refunds once, then stops.
	require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
	require.NoError(t, svc.ReleaseCoupon(ctx, dto.ReleaseCouponInput{CouponID: 1, OrderUUID: orderUUID}))
	assert.Equal(t, int32(0), used)

	// Reserve after a completed release is a conflict, not a second quota hit.
	_, err = svc.ReserveCoupon(ctx, dto.ReserveCouponInput{CouponID: 1, UserID: 10, OrderUUID: orderUUID})
	require.Error(t, err)
	appErr, ok := err.(*utils.AppError)
	require.True(t, ok)
	assert.Equal(t, utils.ErrCodeConflict, appErr.Code)
	assert.Equal(t, int32(0), used)
}

// TestCouponService_ConcurrentReserve runs duplicate reserves in parallel.
// The repository is serialised the way the DB row lock serialises writers.
func TestCouponService_ConcurrentReserve(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New().String()

	var (
		mu         sync.Mutex
		increments int
		existing   uuid.UUID
		reserved   bool
	)

	mock := &mockCouponRepo{
		getByIDForUpdateFunc: func(ctx context.Context, tx pgx.Tx, id int32) (sqlc.Coupon, error) {
			return sqlc.Coupon{CouponID: id}, nil
		},
		incrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, couponID int32) error {
			mu.Lock()
			defer mu.Unlock()
			increments++
			return nil
		},
		upsertUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
			mu.Lock()
			defer mu.Unlock()
			if reserved {
				return sqlc.UpsertCouponUsageRow{CouponUsageUuid: existing, CouponUsageStatus: dto.CouponUsageStatusReserved}, nil
			}
			reserved = true
			existing = uuid.New()
			return sqlc.UpsertCouponUsageRow{CouponUsageUuid: existing, CouponUsageStatus: dto.CouponUsageStatusReserved, Inserted: true}, nil
		},
	}
	svc := service.NewCouponService(mock, nil)

	const workers = 16
	results := make([]string, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.ReserveCoupon(ctx, dto.ReserveCouponInput{
				CouponID:  1,
				UserID:    10,
				OrderUUID: orderUUID,
			})
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		assert.Equal(t, results[0], results[i])
	}
	assert.Equal(t, 1, increments, "concurrent duplicates consume quota once")
}

