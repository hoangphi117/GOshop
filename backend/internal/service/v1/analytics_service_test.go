package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
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
		stats, err := svc.GetOverview(ctx, v1dto.OverviewStatsQuery{})

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
		stats, err := svc.GetOverview(ctx, v1dto.OverviewStatsQuery{})

		require.NoError(t, err)
		assert.Equal(t, float64(500000), stats.TotalRevenue)
		assert.Equal(t, int64(5), stats.TotalOrders)
		assert.Equal(t, 100.0, stats.RevenueGrowth)
		assert.Equal(t, 100.0, stats.OrderGrowth)
		assert.Equal(t, 100.0, stats.CustomerGrowth)
	})

	t.Run("Success: Custom date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOverviewCustomFunc: func(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error) {
				assert.Equal(t, "2026-09-01", fromDate.Format("2006-01-02"))
				assert.Equal(t, "2026-09-15", toDate.Format("2006-01-02"))
				return sqlc.GetOverviewStatsCustomRow{
					Revenue:          utils.FloatToNumeric(8000000),
					OrderCount:       60,
					PrevRevenue:      utils.FloatToNumeric(4000000),
					PrevOrderCount:   30,
					PendingCount:     5,
					NewCustomers:     20,
					PrevNewCustomers: 10,
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		stats, err := svc.GetOverview(ctx, v1dto.OverviewStatsQuery{
			FromDate: "2026-09-01",
			ToDate:   "2026-09-15",
		})

		require.NoError(t, err)
		assert.Equal(t, float64(8000000), stats.TotalRevenue)
		assert.Equal(t, int64(60), stats.TotalOrders)
		assert.Equal(t, 100.0, stats.RevenueGrowth)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOverviewFunc: func(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
				return sqlc.GetOverviewStatsRow{}, errors.New("db connection timeout")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetOverview(ctx, v1dto.OverviewStatsQuery{})

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
		points, err := svc.GetRevenueChart(ctx, v1dto.RevenueChartQuery{Period: "daily", Days: 0, Months: 0}) // days=0 should fallback to 30

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
		points, err := svc.GetRevenueChart(ctx, v1dto.RevenueChartQuery{Period: "monthly", Months: 6})

		require.NoError(t, err)
		require.Len(t, points, 2)
		assert.Equal(t, "2026-08", points[0].Date)
		assert.Equal(t, float64(50000000), points[0].Revenue)
		assert.Equal(t, "2026-09", points[1].Date)
		assert.Equal(t, float64(65000000), points[1].Revenue)
	})

	t.Run("Success: Custom date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRevenueDailyCustomFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error) {
				assert.Equal(t, "2026-09-01", fromDate.Format("2006-01-02"))
				assert.Equal(t, "2026-09-05", toDate.Format("2006-01-02"))
				return []sqlc.GetRevenueDailyCustomRow{
					{
						Date:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
						TotalRevenue: utils.FloatToNumeric(1000000),
						TotalOrders:  5,
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		points, err := svc.GetRevenueChart(ctx, v1dto.RevenueChartQuery{
			FromDate: "2026-09-01",
			ToDate:   "2026-09-05",
		})

		require.NoError(t, err)
		require.Len(t, points, 1)
		assert.Equal(t, "2026-09-01", points[0].Date)
		assert.Equal(t, float64(1000000), points[0].Revenue)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRevenueDailyFunc: func(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
				return nil, errors.New("query failure")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetRevenueChart(ctx, v1dto.RevenueChartQuery{Period: "daily", Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "query failure")
	})
}

func TestAnalyticsService_GetRecentOrders(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()
	createdAt := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)

	t.Run("Success: Map recent orders correctly", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRecentOrdersFunc: func(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
				assert.Equal(t, int32(5), limit)
				return []sqlc.GetRecentOrdersRow{
					{
						OrderID:            1,
						OrderUuid:          orderUUID,
						OrderNumber:        "ORD-20260917-001",
						UserFullname:       "John Doe",
						UserEmail:          "john@example.com",
						OrderTotalAmount:   utils.FloatToNumeric(1250000),
						OrderPaymentMethod: 1,
						OrderPaymentStatus: 2,
						OrderStatus:        1,
						OrderCreatedAt:     createdAt,
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		orders, err := svc.GetRecentOrders(ctx, 0) // limit=0 should fallback to 5

		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, orderUUID.String(), orders[0].UUID)
		assert.Equal(t, "ORD-20260917-001", orders[0].OrderNumber)
		assert.Equal(t, "John Doe", orders[0].CustomerName)
		assert.Equal(t, "john@example.com", orders[0].CustomerEmail)
		assert.Equal(t, float64(1250000), orders[0].TotalAmount)
		assert.Equal(t, int32(1), orders[0].PaymentMethod)
		assert.Equal(t, int32(2), orders[0].PaymentStatus)
		assert.Equal(t, int32(1), orders[0].Status)
		assert.Equal(t, "2026-09-17 10:30:00", orders[0].CreatedAt)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetRecentOrdersFunc: func(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
				return nil, errors.New("query error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetRecentOrders(ctx, 5)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "query error")
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
		products, err := svc.GetTopProducts(ctx, v1dto.TopProductsQuery{Days: 0, Limit: 0}) // default 30 days, 5 limit

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

	t.Run("Success: Custom date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetTopSellingProductsCustomFunc: func(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error) {
				assert.Equal(t, "2026-09-01", fromDate.Format("2006-01-02"))
				assert.Equal(t, "2026-09-10", toDate.Format("2006-01-02"))
				return []sqlc.GetTopSellingProductsCustomRow{
					{
						ProductID:        1,
						ProductUuid:      productUUID,
						ProductName:      "Wireless Headphones Pro",
						ProductSlug:      "wireless-headphones-pro",
						ProductPrice:     utils.FloatToNumeric(2500000),
						ProductImageUrl:  &imgURL,
						CategoryName:     "Electronics",
						TotalUnitsSold:   30,
						TotalSalesAmount: utils.FloatToNumeric(75000000),
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		products, err := svc.GetTopProducts(ctx, v1dto.TopProductsQuery{
			FromDate: "2026-09-01",
			ToDate:   "2026-09-10",
			Limit:    5,
		})

		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Equal(t, int64(30), products[0].TotalUnitsSold)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetTopSellingProductsFunc: func(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
				return nil, errors.New("db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetTopProducts(ctx, v1dto.TopProductsQuery{Days: 30, Limit: 5})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestAnalyticsService_GetOrderStatusDistribution(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Map distribution with percentage and status name", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOrderStatusDistributionFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
				return []sqlc.GetOrderStatusDistributionRow{
					{
						OrderStatus: 1, // Pending
						Count:       20,
						TotalAmount: utils.FloatToNumeric(5000000),
					},
					{
						OrderStatus: 4, // Delivered
						Count:       80,
						TotalAmount: utils.FloatToNumeric(20000000),
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetOrderStatusDistribution(ctx, v1dto.OrderStatusDistributionQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(100), res.TotalOrders)
		require.Len(t, res.Items, 2)

		// Item 1: Pending
		assert.Equal(t, int32(1), res.Items[0].Status)
		assert.Equal(t, "Pending", res.Items[0].StatusName)
		assert.Equal(t, int64(20), res.Items[0].Count)
		assert.Equal(t, float64(5000000), res.Items[0].TotalAmount)
		assert.Equal(t, 20.0, res.Items[0].Percentage)

		// Item 2: Delivered
		assert.Equal(t, int32(4), res.Items[1].Status)
		assert.Equal(t, "Delivered", res.Items[1].StatusName)
		assert.Equal(t, int64(80), res.Items[1].Count)
		assert.Equal(t, float64(20000000), res.Items[1].TotalAmount)
		assert.Equal(t, 80.0, res.Items[1].Percentage)
	})

	t.Run("Success: Zero total orders returns 0 percentage safely", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOrderStatusDistributionFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
				return []sqlc.GetOrderStatusDistributionRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetOrderStatusDistribution(ctx, v1dto.OrderStatusDistributionQuery{Days: 7})

		require.NoError(t, err)
		assert.Equal(t, int64(0), res.TotalOrders)
		assert.Empty(t, res.Items)
	})

	t.Run("Success: Custom from_date and to_date", func(t *testing.T) {
		var capturedFrom, capturedTo time.Time
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOrderStatusDistributionFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
				capturedFrom = fromDate
				capturedTo = toDate
				return []sqlc.GetOrderStatusDistributionRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetOrderStatusDistribution(ctx, v1dto.OrderStatusDistributionQuery{
			FromDate: "2026-09-01",
			ToDate:   "2026-09-15",
		})

		require.NoError(t, err)
		assert.Equal(t, "2026-09-01", capturedFrom.Format("2006-01-02"))
		assert.Equal(t, "2026-09-15", capturedTo.Format("2006-01-02"))
	})

	t.Run("Failure: Invalid date range (from > to)", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{}
		svc := v1service.NewAnalyticsService(mockRepo)

		_, err := svc.GetOrderStatusDistribution(ctx, v1dto.OrderStatusDistributionQuery{
			FromDate: "2026-09-20",
			ToDate:   "2026-09-01",
		})

		require.Error(t, err)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetOrderStatusDistributionFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
				return nil, errors.New("db failure")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetOrderStatusDistribution(ctx, v1dto.OrderStatusDistributionQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db failure")
	})
}

func TestAnalyticsService_GetLowStockProducts(t *testing.T) {
	ctx := context.Background()
	p1UUID := uuid.New()
	p2UUID := uuid.New()
	catName := "Fashion"
	imgURL := "https://example.com/img.png"
	stock0 := int32(0)
	stock3 := int32(3)

	t.Run("Success: Map low stock products with alert levels", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetLowStockProductsFunc: func(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error) {
				assert.Equal(t, int32(5), threshold)
				assert.Equal(t, int32(10), limit)
				return []sqlc.GetLowStockProductsRow{
					{
						ProductID:       1,
						ProductUuid:     p1UUID,
						ProductName:     "Out of Stock T-Shirt",
						ProductSlug:     "out-of-stock-t-shirt",
						ProductPrice:    utils.FloatToNumeric(150000),
						ProductImageUrl: &imgURL,
						ProductStock:    &stock0,
						CategoryName:    &catName,
					},
					{
						ProductID:       2,
						ProductUuid:     p2UUID,
						ProductName:     "Low Stock Jacket",
						ProductSlug:     "low-stock-jacket",
						ProductPrice:    utils.FloatToNumeric(500000),
						ProductImageUrl: nil,
						ProductStock:    &stock3,
						CategoryName:    nil, // uncategorized
					},
				}, nil
			},
			GetLowStockProductsCountFunc: func(ctx context.Context, threshold int32) (int64, error) {
				assert.Equal(t, int32(5), threshold)
				return 7, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetLowStockProducts(ctx, v1dto.LowStockProductsQuery{}) // defaults: threshold 5, limit 10

		require.NoError(t, err)
		assert.Equal(t, int32(5), res.Threshold)
		assert.Equal(t, int64(7), res.TotalLow)
		require.Len(t, res.Items, 2)

		// Item 1: Stock 0 -> critical
		assert.Equal(t, p1UUID.String(), res.Items[0].UUID)
		assert.Equal(t, "Out of Stock T-Shirt", res.Items[0].Name)
		assert.Equal(t, int32(0), res.Items[0].Stock)
		assert.Equal(t, "critical", res.Items[0].AlertLevel)
		assert.Equal(t, "Fashion", res.Items[0].CategoryName)
		assert.Equal(t, &imgURL, res.Items[0].ImageURL)

		// Item 2: Stock 3 -> warning
		assert.Equal(t, p2UUID.String(), res.Items[1].UUID)
		assert.Equal(t, "Low Stock Jacket", res.Items[1].Name)
		assert.Equal(t, int32(3), res.Items[1].Stock)
		assert.Equal(t, "warning", res.Items[1].AlertLevel)
		assert.Equal(t, "", res.Items[1].CategoryName)
		assert.Nil(t, res.Items[1].ImageURL)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetLowStockProductsFunc: func(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error) {
				return nil, errors.New("inventory query error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetLowStockProducts(ctx, v1dto.LowStockProductsQuery{Threshold: 5, Limit: 10})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory query error")
	})
}

func TestAnalyticsService_GetPaymentMethodBreakdown(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Map payment methods breakdown with percentage", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetPaymentMethodBreakdownFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
				return []sqlc.GetPaymentMethodBreakdownRow{
					{
						OrderPaymentMethod: 1, // COD
						OrderCount:         60,
						NetRevenue:         utils.FloatToNumeric(30000000),
					},
					{
						OrderPaymentMethod: 2, // Online / Bank Transfer
						OrderCount:         40,
						NetRevenue:         utils.FloatToNumeric(20000000),
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetPaymentMethodBreakdown(ctx, v1dto.PaymentMethodBreakdownQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, float64(50000000), res.TotalRevenue)
		assert.Equal(t, int64(100), res.TotalOrders)
		require.Len(t, res.Items, 2)

		// Item 1: COD
		assert.Equal(t, int32(1), res.Items[0].PaymentMethod)
		assert.Equal(t, "COD", res.Items[0].MethodName)
		assert.Equal(t, int64(60), res.Items[0].OrderCount)
		assert.Equal(t, float64(30000000), res.Items[0].NetRevenue)
		assert.Equal(t, 60.0, res.Items[0].Percentage)

		// Item 2: Online
		assert.Equal(t, int32(2), res.Items[1].PaymentMethod)
		assert.Equal(t, "Bank Transfer", res.Items[1].MethodName)
		assert.Equal(t, int64(40), res.Items[1].OrderCount)
		assert.Equal(t, float64(20000000), res.Items[1].NetRevenue)
		assert.Equal(t, 40.0, res.Items[1].Percentage)
	})

	t.Run("Success: Zero total revenue handles percentage safely", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetPaymentMethodBreakdownFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
				return []sqlc.GetPaymentMethodBreakdownRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetPaymentMethodBreakdown(ctx, v1dto.PaymentMethodBreakdownQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, float64(0), res.TotalRevenue)
		assert.Equal(t, int64(0), res.TotalOrders)
		assert.Empty(t, res.Items)
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetPaymentMethodBreakdownFunc: func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
				return nil, errors.New("db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetPaymentMethodBreakdown(ctx, v1dto.PaymentMethodBreakdownQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestAnalyticsService_GetCartFunnel(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Normal conversion and abandonment calculation", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCartAbandonmentAndFunnelStatsFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error) {
				return sqlc.GetCartAbandonmentAndFunnelStatsRow{
					TotalCarts:           200,
					OrdersCreated:        80,
					OrdersPaid:           60,
					AbandonedCarts:       120,
					PotentialLostRevenue: utils.FloatToNumeric(45000000),
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCartFunnel(ctx, v1dto.CartFunnelQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(200), res.TotalCarts)
		assert.Equal(t, int64(80), res.OrdersCreated)
		assert.Equal(t, int64(60), res.OrdersPaid)
		assert.Equal(t, int64(120), res.AbandonedCarts)
		assert.Equal(t, float64(45000000), res.PotentialLostRevenue)
		assert.Equal(t, 60.0, res.AbandonmentRate) // 120 / 200 = 60.0%
		assert.Equal(t, 40.0, res.CartToOrderRate) // 80 / 200 = 40.0%
		assert.Equal(t, 75.0, res.OrderToPaidRate) // 60 / 80 = 75.0%
	})

	t.Run("Success: Zero carts (safe division by zero)", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCartAbandonmentAndFunnelStatsFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error) {
				return sqlc.GetCartAbandonmentAndFunnelStatsRow{
					TotalCarts:           0,
					OrdersCreated:        0,
					OrdersPaid:           0,
					AbandonedCarts:       0,
					PotentialLostRevenue: pgtype.Numeric{Valid: false},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCartFunnel(ctx, v1dto.CartFunnelQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(0), res.TotalCarts)
		assert.Equal(t, 0.0, res.AbandonmentRate)
		assert.Equal(t, 0.0, res.CartToOrderRate)
		assert.Equal(t, 0.0, res.OrderToPaidRate)
		assert.Equal(t, 0.0, res.PotentialLostRevenue)
	})

	t.Run("Failure: Invalid date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{}
		svc := v1service.NewAnalyticsService(mockRepo)

		_, err := svc.GetCartFunnel(ctx, v1dto.CartFunnelQuery{
			FromDate: "2026-09-30",
			ToDate:   "2026-09-01",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid date range")
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCartAbandonmentAndFunnelStatsFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error) {
				return sqlc.GetCartAbandonmentAndFunnelStatsRow{}, errors.New("db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetCartFunnel(ctx, v1dto.CartFunnelQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestAnalyticsService_GetReturnAnalytics(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Normal return rate and breakdown calculation", func(t *testing.T) {
		prodUUID := uuid.New()
		mockRepo := &testutils.MockAnalyticsRepository{
			GetReturnAnalyticsSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error) {
				return sqlc.GetReturnAnalyticsSummaryRow{
					TotalReturns:        25,
					ReturnedOrdersCount: 20,
					TotalPaidOrders:     500,
					TotalRefundedAmount: utils.FloatToNumeric(12500000),
				}, nil
			},
			GetReturnReasonsBreakdownFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetReturnReasonsBreakdownRow, error) {
				return []sqlc.GetReturnReasonsBreakdownRow{
					{ReturnReason: "Defective item", ReasonCount: 15},
					{ReturnReason: "Wrong size", ReasonCount: 10},
				}, nil
			},
			GetTopReturnedProductsFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetTopReturnedProductsRow, error) {
				return []sqlc.GetTopReturnedProductsRow{
					{
						ProductID:          1,
						ProductUuid:        prodUUID,
						ProductName:        "Mechanical Keyboard",
						ProductSlug:        "mechanical-keyboard",
						ProductPrice:       utils.FloatToNumeric(1500000),
						ReturnCaseCount:    8,
						ReturnedUnitsCount: 8,
						ReturnedValue:      utils.FloatToNumeric(12000000),
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetReturnAnalytics(ctx, v1dto.ReturnAnalyticsQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(25), res.TotalReturns)
		assert.Equal(t, int64(20), res.ReturnedOrdersCount)
		assert.Equal(t, int64(500), res.TotalPaidOrders)
		assert.Equal(t, 4.0, res.ReturnRate) // 20 / 500 = 4.0%
		assert.Equal(t, float64(12500000), res.TotalRefundedAmount)

		require.Len(t, res.ReasonsBreakdown, 2)
		assert.Equal(t, "Defective item", res.ReasonsBreakdown[0].Reason)
		assert.Equal(t, int64(15), res.ReasonsBreakdown[0].Count)
		assert.Equal(t, 60.0, res.ReasonsBreakdown[0].Percentage) // 15 / 25 = 60.0%

		require.Len(t, res.TopReturnedProducts, 1)
		assert.Equal(t, prodUUID.String(), res.TopReturnedProducts[0].UUID)
		assert.Equal(t, "Mechanical Keyboard", res.TopReturnedProducts[0].Name)
		assert.Equal(t, int64(8), res.TopReturnedProducts[0].ReturnCaseCount)
		assert.Equal(t, float64(12000000), res.TopReturnedProducts[0].ReturnedValue)
	})

	t.Run("Success: Zero returns and zero paid orders", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetReturnAnalyticsSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error) {
				return sqlc.GetReturnAnalyticsSummaryRow{
					TotalReturns:        0,
					ReturnedOrdersCount: 0,
					TotalPaidOrders:     0,
					TotalRefundedAmount: pgtype.Numeric{Valid: false},
				}, nil
			},
			GetReturnReasonsBreakdownFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetReturnReasonsBreakdownRow, error) {
				return []sqlc.GetReturnReasonsBreakdownRow{}, nil
			},
			GetTopReturnedProductsFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetTopReturnedProductsRow, error) {
				return []sqlc.GetTopReturnedProductsRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetReturnAnalytics(ctx, v1dto.ReturnAnalyticsQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(0), res.TotalReturns)
		assert.Equal(t, 0.0, res.ReturnRate)
		assert.Equal(t, float64(0), res.TotalRefundedAmount)
		assert.Empty(t, res.ReasonsBreakdown)
		assert.Empty(t, res.TopReturnedProducts)
	})

	t.Run("Failure: Invalid date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{}
		svc := v1service.NewAnalyticsService(mockRepo)

		_, err := svc.GetReturnAnalytics(ctx, v1dto.ReturnAnalyticsQuery{
			FromDate: "2026-09-30",
			ToDate:   "2026-09-01",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid date range")
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetReturnAnalyticsSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error) {
				return sqlc.GetReturnAnalyticsSummaryRow{}, errors.New("db query error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetReturnAnalytics(ctx, v1dto.ReturnAnalyticsQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db query error")
	})
}

func TestAnalyticsService_GetCategorySalesBreakdown(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Normal category sales with revenue share calculation", func(t *testing.T) {
		catUUID1 := uuid.New()
		catUUID2 := uuid.New()
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCategorySalesBreakdownFunc: func(ctx context.Context, from, to time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error) {
				return []sqlc.GetCategorySalesBreakdownRow{
					{
						CategoryID:     1,
						CategoryUuid:   catUUID1,
						CategoryName:   "Keyboards",
						CategorySlug:   "keyboards",
						TotalUnitsSold: 120,
						GrossRevenue:   utils.FloatToNumeric(60000000),
						OrderCount:     80,
					},
					{
						CategoryID:     2,
						CategoryUuid:   catUUID2,
						CategoryName:   "Mice",
						CategorySlug:   "mice",
						TotalUnitsSold: 80,
						GrossRevenue:   utils.FloatToNumeric(40000000),
						OrderCount:     50,
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCategorySalesBreakdown(ctx, v1dto.CategorySalesBreakdownQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, float64(100000000), res.TotalRevenue)
		assert.Equal(t, int64(200), res.TotalUnitsSold)
		assert.Equal(t, int64(130), res.TotalOrders)
		require.Len(t, res.Categories, 2)

		// Item 1
		assert.Equal(t, "Keyboards", res.Categories[0].CategoryName)
		assert.Equal(t, float64(60000000), res.Categories[0].GrossRevenue)
		assert.Equal(t, 60.0, res.Categories[0].Percentage) // 60M / 100M = 60.0%

		// Item 2
		assert.Equal(t, "Mice", res.Categories[1].CategoryName)
		assert.Equal(t, float64(40000000), res.Categories[1].GrossRevenue)
		assert.Equal(t, 40.0, res.Categories[1].Percentage) // 40M / 100M = 40.0%
	})

	t.Run("Success: Zero sales across categories", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCategorySalesBreakdownFunc: func(ctx context.Context, from, to time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error) {
				return []sqlc.GetCategorySalesBreakdownRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCategorySalesBreakdown(ctx, v1dto.CategorySalesBreakdownQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, float64(0), res.TotalRevenue)
		assert.Equal(t, int64(0), res.TotalUnitsSold)
		assert.Empty(t, res.Categories)
	})

	t.Run("Failure: Invalid date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{}
		svc := v1service.NewAnalyticsService(mockRepo)

		_, err := svc.GetCategorySalesBreakdown(ctx, v1dto.CategorySalesBreakdownQuery{
			FromDate: "2026-09-30",
			ToDate:   "2026-09-01",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid date range")
	})

	t.Run("Failure: Repository error returned", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCategorySalesBreakdownFunc: func(ctx context.Context, from, to time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error) {
				return nil, errors.New("database connection failed")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetCategorySalesBreakdown(ctx, v1dto.CategorySalesBreakdownQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database connection failed")
	})
}

func TestAnalyticsService_GetCouponPerformance(t *testing.T) {
	ctx := context.Background()

	t.Run("Success: Normal coupon performance with ROI multiplier calculation", func(t *testing.T) {
		cUUID1 := uuid.New()
		cUUID2 := uuid.New()
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCouponPerformanceSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCouponPerformanceSummaryRow, error) {
				return sqlc.GetCouponPerformanceSummaryRow{
					TotalOrdersWithCoupon: 150,
					TotalDiscountAmount:   utils.FloatToNumeric(10000000), // 10M
					TotalRevenueGenerated: utils.FloatToNumeric(50000000), // 50M -> ROI = 5.0
				}, nil
			},
			GetCouponCampaignBreakdownFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error) {
				assert.Equal(t, int32(20), limit)
				return []sqlc.GetCouponCampaignBreakdownRow{
					{
						CouponID:              1,
						CouponUuid:            cUUID1,
						CouponCode:            "SUMMER20",
						CouponName:            "Summer Sale 20%",
						CouponType:            1,
						CouponDiscountValue:   utils.FloatToNumeric(20),
						UsageCount:            100,
						TotalDiscountAmount:   utils.FloatToNumeric(8000000),  // 8M
						TotalRevenueGenerated: utils.FloatToNumeric(44000000), // 44M -> ROI = 5.5
					},
					{
						CouponID:              2,
						CouponUuid:            cUUID2,
						CouponCode:            "WELCOME50K",
						CouponName:            "Welcome Voucher",
						CouponType:            2,
						CouponDiscountValue:   utils.FloatToNumeric(50000),
						UsageCount:            50,
						TotalDiscountAmount:   utils.FloatToNumeric(2000000), // 2M
						TotalRevenueGenerated: utils.FloatToNumeric(6000000), // 6M -> ROI = 3.0
					},
				}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCouponPerformance(ctx, v1dto.CouponPerformanceQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(150), res.TotalCouponsUsed)
		assert.Equal(t, float64(10000000), res.TotalDiscountGiven)
		assert.Equal(t, float64(50000000), res.TotalRevenueGenerated)
		assert.Equal(t, 5.0, res.OverallROIMultiplier) // 50M / 10M = 5.0

		require.Len(t, res.Campaigns, 2)
		// Item 1
		assert.Equal(t, int32(1), res.Campaigns[0].CouponID)
		assert.Equal(t, cUUID1.String(), res.Campaigns[0].CouponUUID)
		assert.Equal(t, "SUMMER20", res.Campaigns[0].CouponCode)
		assert.Equal(t, int64(100), res.Campaigns[0].UsageCount)
		assert.Equal(t, float64(8000000), res.Campaigns[0].TotalDiscount)
		assert.Equal(t, float64(44000000), res.Campaigns[0].TotalRevenue)
		assert.Equal(t, 5.5, res.Campaigns[0].ROIMultiplier) // 44M / 8M = 5.5

		// Item 2
		assert.Equal(t, int32(2), res.Campaigns[1].CouponID)
		assert.Equal(t, cUUID2.String(), res.Campaigns[1].CouponUUID)
		assert.Equal(t, "WELCOME50K", res.Campaigns[1].CouponCode)
		assert.Equal(t, int64(50), res.Campaigns[1].UsageCount)
		assert.Equal(t, float64(2000000), res.Campaigns[1].TotalDiscount)
		assert.Equal(t, float64(6000000), res.Campaigns[1].TotalRevenue)
		assert.Equal(t, 3.0, res.Campaigns[1].ROIMultiplier) // 6M / 2M = 3.0
	})

	t.Run("Success: Zero discount given handles ROI safely", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCouponPerformanceSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCouponPerformanceSummaryRow, error) {
				return sqlc.GetCouponPerformanceSummaryRow{
					TotalOrdersWithCoupon: 0,
					TotalDiscountAmount:   utils.FloatToNumeric(0),
					TotalRevenueGenerated: utils.FloatToNumeric(0),
				}, nil
			},
			GetCouponCampaignBreakdownFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error) {
				return []sqlc.GetCouponCampaignBreakdownRow{}, nil
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		res, err := svc.GetCouponPerformance(ctx, v1dto.CouponPerformanceQuery{Days: 30})

		require.NoError(t, err)
		assert.Equal(t, int64(0), res.TotalCouponsUsed)
		assert.Equal(t, float64(0), res.TotalDiscountGiven)
		assert.Equal(t, float64(0), res.TotalRevenueGenerated)
		assert.Equal(t, 0.0, res.OverallROIMultiplier)
		assert.Empty(t, res.Campaigns)
	})

	t.Run("Failure: Invalid date range", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{}
		svc := v1service.NewAnalyticsService(mockRepo)

		_, err := svc.GetCouponPerformance(ctx, v1dto.CouponPerformanceQuery{
			FromDate: "2026-09-30",
			ToDate:   "2026-09-01",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid date range")
	})

	t.Run("Failure: Repository summary error", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCouponPerformanceSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCouponPerformanceSummaryRow, error) {
				return sqlc.GetCouponPerformanceSummaryRow{}, errors.New("summary db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetCouponPerformance(ctx, v1dto.CouponPerformanceQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "summary db error")
	})

	t.Run("Failure: Repository breakdown error", func(t *testing.T) {
		mockRepo := &testutils.MockAnalyticsRepository{
			GetCouponPerformanceSummaryFunc: func(ctx context.Context, from, to time.Time) (sqlc.GetCouponPerformanceSummaryRow, error) {
				return sqlc.GetCouponPerformanceSummaryRow{}, nil
			},
			GetCouponCampaignBreakdownFunc: func(ctx context.Context, from, to time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error) {
				return nil, errors.New("breakdown db error")
			},
		}

		svc := v1service.NewAnalyticsService(mockRepo)
		_, err := svc.GetCouponPerformance(ctx, v1dto.CouponPerformanceQuery{Days: 30})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "breakdown db error")
	})
}



