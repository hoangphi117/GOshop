package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsService_GetOverview(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Normal growth calculation", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOverviewFunc: func(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
				return sqlc.GetOverviewStatsRow{
					Revenue:          utils.FloatToNumeric(15000000),
					OrderCount:       120,
					PrevRevenue:      utils.FloatToNumeric(10000000),
					PrevOrderCount:   100,
					PendingCount:     15,
					NewCustomers:     45,
					PrevNewCustomers: 30,
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		stats, err := svc.GetOverview(ctx)

		require.NoError(t, err)
		assert.Equal(t, float64(15000000), stats.TotalRevenue)
		assert.Equal(t, int64(120), stats.TotalOrders)
		assert.Equal(t, int64(45), stats.NewCustomers)
		assert.Equal(t, int64(15), stats.PendingOrders)
		assert.Equal(t, 50.0, stats.RevenueGrowth)   // (15M - 10M) / 10M * 100 = 50%
		assert.Equal(t, 20.0, stats.OrderGrowth)     // (120 - 100) / 100 * 100 = 20%
		assert.Equal(t, 50.0, stats.CustomerGrowth)  // (45 - 30) / 30 * 100 = 50%
	})

	t.Run("Success: Zero previous metrics (no division by zero)", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOverviewFunc: func(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
				return sqlc.GetOverviewStatsRow{
					Revenue:          utils.FloatToNumeric(500000),
					OrderCount:       5,
					PrevRevenue:      pgtype.Numeric{Valid: false},
					PrevOrderCount:   0,
					PendingCount:     2,
					NewCustomers:     10,
					PrevNewCustomers: 0,
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		stats, err := svc.GetOverview(ctx)

		require.NoError(t, err)
		assert.Equal(t, float64(500000), stats.TotalRevenue)
		assert.Equal(t, int64(5), stats.TotalOrders)
		assert.Equal(t, 100.0, stats.RevenueGrowth)
		assert.Equal(t, 100.0, stats.OrderGrowth)
		assert.Equal(t, 100.0, stats.CustomerGrowth)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOverviewFunc: func(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
				return sqlc.GetOverviewStatsRow{}, errors.New("db connection timeout")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetOverview(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db connection timeout")
	})
}

func TestAnalyticsService_GetRevenueChart(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)

	t.Run("Success: Daily chart", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRevenueDailyFunc: func(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
				assert.Equal(t, int32(30), days)
				return []sqlc.GetRevenueDailyChartRow{
					{
						Date:         now.AddDate(0, 0, -1),
						TotalRevenue: utils.FloatToNumeric(2500000),
						TotalOrders:  10,
					},
					{
						Date:         now,
						TotalRevenue: utils.FloatToNumeric(3000000),
						TotalOrders:  14,
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		points, err := svc.GetRevenueChart(ctx, "daily", 0, 0) // days=0 should fallback to 30

		require.NoError(t, err)
		require.Len(t, points, 2)
		assert.Equal(t, "2026-09-16", points[0].Date)
		assert.Equal(t, float64(2500000), points[0].Revenue)
		assert.Equal(t, int64(10), points[0].OrderCount)
		assert.Equal(t, "2026-09-17", points[1].Date)
		assert.Equal(t, float64(3000000), points[1].Revenue)
		assert.Equal(t, int64(14), points[1].OrderCount)
	})

	t.Run("Success: Monthly chart", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRevenueMonthlyFunc: func(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error) {
				assert.Equal(t, int32(6), months)
				return []sqlc.GetRevenueMonthlyChartRow{
					{
						Month:        time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
						TotalRevenue: utils.FloatToNumeric(50000000),
						TotalOrders:  220,
					},
					{
						Month:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
						TotalRevenue: utils.FloatToNumeric(65000000),
						TotalOrders:  310,
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		points, err := svc.GetRevenueChart(ctx, "monthly", 0, 6)

		require.NoError(t, err)
		require.Len(t, points, 2)
		assert.Equal(t, "2026-08", points[0].Date)
		assert.Equal(t, float64(50000000), points[0].Revenue)
		assert.Equal(t, "2026-09", points[1].Date)
		assert.Equal(t, float64(65000000), points[1].Revenue)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRevenueDailyFunc: func(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
				return nil, errors.New("query failure")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetRevenueChart(ctx, "daily", 30, 0)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "query failure")
	})
}

func TestAnalyticsService_GetRecentOrders(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()
	userUUID := uuid.New()
	createdAt := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	t.Run("Success: Map recent orders correctly", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRecentOrdersFunc: func(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
				assert.Equal(t, int32(5), limit)
				return []sqlc.GetRecentOrdersRow{
					{
						OrderID:            101,
						OrderUuid:          orderUUID,
						OrderNumber:        "ORD-20260917-001",
						OrderShippingName:  "Alice Smith",
						OrderTotalAmount:   utils.FloatToNumeric(1200000),
						OrderPaymentMethod: 1,
						OrderPaymentStatus: 2,
						OrderStatus:        2,
						OrderCreatedAt:     createdAt,
						UserUuid:           userUUID,
						UserFullname:       "Alice Smith",
						UserEmail:          "alice@example.com",
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		orders, err := svc.GetRecentOrders(ctx, 0) // limit <= 0 -> fallback to 5

		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, orderUUID.String(), orders[0].UUID)
		assert.Equal(t, "ORD-20260917-001", orders[0].OrderNumber)
		assert.Equal(t, "Alice Smith", orders[0].CustomerName)
		assert.Equal(t, "alice@example.com", orders[0].CustomerEmail)
		assert.Equal(t, float64(1200000), orders[0].TotalAmount)
		assert.Equal(t, int32(2), orders[0].Status)
		assert.Equal(t, "2026-09-17 10:30:00", orders[0].CreatedAt)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRecentOrdersFunc: func(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
				return nil, errors.New("database error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetRecentOrders(ctx, 5)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
	})
}

func TestAnalyticsService_GetTopProducts(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	imgURL := "https://example.com/images/prod1.jpg"

	t.Run("Success: Map top products correctly", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetTopSellingProductsFunc: func(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
				assert.Equal(t, int32(30), days)
				assert.Equal(t, int32(5), limit)
				return []sqlc.GetTopSellingProductsRow{
					{
						ProductID:        1,
						ProductUuid:      productUUID,
						ProductName:      "Wireless Headphones Pro",
						ProductSlug:      "wireless-headphones-pro",
						ProductPrice:     utils.FloatToNumeric(2500000),
						ProductImageUrl:  &imgURL,
						CategoryName:     "Electronics",
						TotalUnitsSold:   85,
						TotalSalesAmount: utils.FloatToNumeric(212500000),
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		products, err := svc.GetTopProducts(ctx, 0, 0) // default 30 days, 5 limit

		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Equal(t, productUUID.String(), products[0].UUID)
		assert.Equal(t, "Wireless Headphones Pro", products[0].Name)
		assert.Equal(t, "wireless-headphones-pro", products[0].Slug)
		assert.Equal(t, float64(2500000), products[0].Price)
		assert.Equal(t, &imgURL, products[0].ImageURL)
		assert.Equal(t, "Electronics", products[0].CategoryName)
		assert.Equal(t, int64(85), products[0].TotalUnitsSold)
		assert.Equal(t, float64(212500000), products[0].TotalRevenue)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetTopSellingProductsFunc: func(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
				return nil, errors.New("db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetTopProducts(ctx, 30, 5)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}
