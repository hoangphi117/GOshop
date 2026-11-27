package service

import (
	"context"
	"errors"
	"testing"
	"time"

	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/db/sqlc"
	"goshop/services/analytics/internal/repository"
	catalogcontract "goshop/services/catalog/contract"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/shared/pkg/messaging"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAnalyticsRepo struct {
	mock.Mock
}

func (m *mockAnalyticsRepo) GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
	args := m.Called(ctx)
	return args.Get(0).(sqlc.GetOverviewStatsRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetOverviewCustom(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error) {
	args := m.Called(ctx, fromDate, toDate, prevFromDate)
	return args.Get(0).(sqlc.GetOverviewStatsCustomRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
	args := m.Called(ctx, days)
	return args.Get(0).([]sqlc.GetRevenueDailyChartRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetRevenueDailyCustom(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error) {
	args := m.Called(ctx, fromDate, toDate)
	return args.Get(0).([]sqlc.GetRevenueDailyCustomRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetRevenueMonthly(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error) {
	args := m.Called(ctx, months)
	return args.Get(0).([]sqlc.GetRevenueMonthlyChartRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetRecentOrders(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]sqlc.GetRecentOrdersRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetTopSellingProducts(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
	args := m.Called(ctx, days, limit)
	return args.Get(0).([]sqlc.GetTopSellingProductsRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetTopSellingProductsCustom(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error) {
	args := m.Called(ctx, fromDate, toDate, limit)
	return args.Get(0).([]sqlc.GetTopSellingProductsCustomRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
	args := m.Called(ctx, fromDate, toDate)
	return args.Get(0).([]sqlc.GetOrderStatusDistributionRow), args.Error(1)
}

func (m *mockAnalyticsRepo) GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
	args := m.Called(ctx, fromDate, toDate)
	return args.Get(0).([]sqlc.GetPaymentMethodBreakdownRow), args.Error(1)
}

func TestAnalyticsService_Overview(t *testing.T) {
	t.Run("custom dates success", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetOverviewCustom", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			sqlc.GetOverviewStatsCustomRow{
				Revenue:        numeric(200),
				OrderCount:     10,
				PendingCount:   2,
				PrevRevenue:    numeric(100),
				PrevOrderCount: 5,
			}, nil,
		)

		res, err := svc.GetOverview(context.Background(), analyticsdto.OverviewStatsQuery{
			FromDate: "2026-01-01",
			ToDate:   "2026-01-31",
		})
		assert.NoError(t, err)
		assert.Equal(t, float64(200), res.TotalRevenue)
		assert.Equal(t, int64(10), res.TotalOrders)
		assert.Equal(t, int64(2), res.PendingOrders)
		assert.Equal(t, float64(100), res.RevenueGrowth)
		assert.Equal(t, float64(100), res.OrderGrowth)
	})

	t.Run("custom dates invalid range", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		_, err := svc.GetOverview(context.Background(), analyticsdto.OverviewStatsQuery{
			FromDate: "2026-02-01",
			ToDate:   "2026-01-01",
		})
		assert.Error(t, err)
	})

	t.Run("custom dates repo error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetOverviewCustom", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			sqlc.GetOverviewStatsCustomRow{}, errors.New("db error"),
		)

		_, err := svc.GetOverview(context.Background(), analyticsdto.OverviewStatsQuery{
			FromDate: "2026-01-01",
			ToDate:   "2026-01-31",
		})
		assert.Error(t, err)
	})

	t.Run("default overview repo error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetOverview", mock.Anything).Return(sqlc.GetOverviewStatsRow{}, errors.New("db error"))

		_, err := svc.GetOverview(context.Background(), analyticsdto.OverviewStatsQuery{})
		assert.Error(t, err)
	})
}

func TestAnalyticsService_RevenueChart(t *testing.T) {
	t.Run("monthly chart default and custom months", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		monthTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		repo.On("GetRevenueMonthly", mock.Anything, int32(12)).Return([]sqlc.GetRevenueMonthlyChartRow{
			{
				Month:        monthTime,
				TotalRevenue: numeric(500),
				TotalOrders:  25,
			},
		}, nil)

		points, err := svc.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{
			Period: "monthly",
		})
		assert.NoError(t, err)
		assert.Len(t, points, 1)
		assert.Equal(t, "2026-01", points[0].Date)
		assert.Equal(t, float64(500), points[0].Revenue)
		assert.Equal(t, int64(25), points[0].OrderCount)
	})

	t.Run("monthly chart error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetRevenueMonthly", mock.Anything, int32(6)).Return([]sqlc.GetRevenueMonthlyChartRow{}, errors.New("db fail"))

		_, err := svc.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{
			Period: "monthly",
			Months: 6,
		})
		assert.Error(t, err)
	})

	t.Run("daily custom dates success and validation error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		dateTime := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		repo.On("GetRevenueDailyCustom", mock.Anything, mock.Anything, mock.Anything).Return([]sqlc.GetRevenueDailyCustomRow{
			{
				Date:         dateTime,
				TotalRevenue: numeric(100),
				TotalOrders:  5,
			},
		}, nil)

		points, err := svc.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{
			FromDate: "2026-01-01",
			ToDate:   "2026-01-31",
		})
		assert.NoError(t, err)
		assert.Len(t, points, 1)
		assert.Equal(t, "2026-01-15", points[0].Date)

		// Invalid date
		_, err = svc.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{
			FromDate: "invalid-date",
			ToDate:   "2026-01-31",
		})
		assert.Error(t, err)
	})

	t.Run("daily default days and repo error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetRevenueDaily", mock.Anything, int32(30)).Return([]sqlc.GetRevenueDailyChartRow{}, nil)

		points, err := svc.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{})
		assert.NoError(t, err)
		assert.Empty(t, points)

		// repo error
		repo2 := new(mockAnalyticsRepo)
		svc2 := NewAnalyticsService(repo2, nil, nil, nil, nil)
		repo2.On("GetRevenueDaily", mock.Anything, int32(7)).Return([]sqlc.GetRevenueDailyChartRow{}, errors.New("err"))
		_, err = svc2.GetRevenueChart(context.Background(), analyticsdto.RevenueChartQuery{Days: 7})
		assert.Error(t, err)
	})
}

func TestAnalyticsService_RecentOrders(t *testing.T) {
	t.Run("recent orders success and default limit", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		orderUUID := uuid.New()
		now := time.Now()
		repo.On("GetRecentOrders", mock.Anything, int32(5)).Return([]sqlc.GetRecentOrdersRow{
			{
				OrderUuid:          orderUUID,
				OrderNumber:        "ORD-001",
				UserFullname:       "John Doe",
				UserEmail:          "john@example.com",
				OrderTotalAmount:   numeric(150),
				OrderPaymentMethod: 1,
				OrderPaymentStatus: 2,
				OrderStatus:        1,
				OrderCreatedAt:     now,
			},
		}, nil)

		orders, err := svc.GetRecentOrders(context.Background(), 0)
		assert.NoError(t, err)
		assert.Len(t, orders, 1)
		assert.Equal(t, "ORD-001", orders[0].OrderNumber)
		assert.Equal(t, "John Doe", orders[0].CustomerName)
		assert.Equal(t, float64(150), orders[0].TotalAmount)
	})

	t.Run("recent orders repo error", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetRecentOrders", mock.Anything, int32(10)).Return([]sqlc.GetRecentOrdersRow{}, errors.New("db error"))

		_, err := svc.GetRecentOrders(context.Background(), 10)
		assert.Error(t, err)
	})
}

func TestAnalyticsService_TopProducts(t *testing.T) {
	t.Run("default days and catalog enrichment with deleted and image", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		catalog := stubCatalog{products: []catalogcontract.ProductItemInfo{
			{
				ProductID:           1,
				ProductUUID:         "prod-1-uuid",
				ProductName:         "Keyboard",
				ProductSlug:         "keyboard",
				ProductPrice:        50.0,
				ProductCategoryName: "Peripherals",
				ProductImageURL:     "http://example.com/keyboard.jpg",
			},
			// ProductID 2 is missing from catalog (deleted)
		}}
		svc := NewAnalyticsService(repo, nil, catalog, nil, nil)

		repo.On("GetTopSellingProducts", mock.Anything, int32(30), int32(5)).Return([]sqlc.GetTopSellingProductsRow{
			{
				ProductID:        1,
				TotalUnitsSold:   10,
				TotalSalesAmount: numeric(500),
			},
			{
				ProductID:        2,
				TotalUnitsSold:   2,
				TotalSalesAmount: numeric(100),
			},
		}, nil)

		products, err := svc.GetTopProducts(context.Background(), analyticsdto.TopProductsQuery{})
		assert.NoError(t, err)
		assert.Len(t, products, 1) // Product 2 was deleted from catalog, so skipped
		assert.Equal(t, "Keyboard", products[0].Name)
		assert.NotNil(t, products[0].ImageURL)
		assert.Equal(t, "http://example.com/keyboard.jpg", *products[0].ImageURL)
	})

	t.Run("custom date range success", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		catalog := stubCatalog{products: []catalogcontract.ProductItemInfo{
			{
				ProductID:           5,
				ProductUUID:         "prod-5-uuid",
				ProductName:         "Mouse",
				ProductSlug:         "mouse",
				ProductPrice:        25.0,
				ProductCategoryName: "Peripherals",
			},
		}}
		svc := NewAnalyticsService(repo, nil, catalog, nil, nil)

		repo.On("GetTopSellingProductsCustom", mock.Anything, mock.Anything, mock.Anything, int32(5)).Return(
			[]sqlc.GetTopSellingProductsCustomRow{
				{
					ProductID:        5,
					TotalUnitsSold:   8,
					TotalSalesAmount: numeric(200),
				},
			}, nil,
		)

		products, err := svc.GetTopProducts(context.Background(), analyticsdto.TopProductsQuery{
			FromDate: "2026-01-01",
			ToDate:   "2026-01-31",
		})
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, "Mouse", products[0].Name)
		assert.Nil(t, products[0].ImageURL)
	})
}

func TestAnalyticsService_OrderStatusDistribution(t *testing.T) {
	t.Run("distribution success with percentages", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetOrderStatusDistribution", mock.Anything, mock.Anything, mock.Anything).Return([]sqlc.GetOrderStatusDistributionRow{
			{
				OrderStatus: 1, // Pending
				Count:       20,
				TotalAmount: numeric(1000),
			},
			{
				OrderStatus: 4, // Delivered
				Count:       80,
				TotalAmount: numeric(4000),
			},
		}, nil)

		res, err := svc.GetOrderStatusDistribution(context.Background(), analyticsdto.OrderStatusDistributionQuery{
			Days: 30,
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(100), res.TotalOrders)
		assert.Len(t, res.Items, 2)
		assert.Equal(t, 20.0, res.Items[0].Percentage)
		assert.Equal(t, 80.0, res.Items[1].Percentage)
	})

	t.Run("zero orders", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetOrderStatusDistribution", mock.Anything, mock.Anything, mock.Anything).Return([]sqlc.GetOrderStatusDistributionRow{}, nil)

		res, err := svc.GetOrderStatusDistribution(context.Background(), analyticsdto.OrderStatusDistributionQuery{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), res.TotalOrders)
		assert.Empty(t, res.Items)
	})
}

func TestAnalyticsService_PaymentMethodBreakdown(t *testing.T) {
	t.Run("breakdown success with percentages", func(t *testing.T) {
		repo := new(mockAnalyticsRepo)
		svc := NewAnalyticsService(repo, nil, nil, nil, nil)

		repo.On("GetPaymentMethodBreakdown", mock.Anything, mock.Anything, mock.Anything).Return([]sqlc.GetPaymentMethodBreakdownRow{
			{
				OrderPaymentMethod: 1, // COD
				OrderCount:         10,
				NetRevenue:         numeric(100),
			},
			{
				OrderPaymentMethod: 2, // VNPAY
				OrderCount:         30,
				NetRevenue:         numeric(300),
			},
		}, nil)

		res, err := svc.GetPaymentMethodBreakdown(context.Background(), analyticsdto.PaymentMethodBreakdownQuery{})
		assert.NoError(t, err)
		assert.Equal(t, float64(400), res.TotalRevenue)
		assert.Equal(t, int64(40), res.TotalOrders)
		assert.Len(t, res.Items, 2)
		assert.Equal(t, 25.0, res.Items[0].Percentage)
		assert.Equal(t, 75.0, res.Items[1].Percentage)
	})
}

func TestAnalyticsService_LowStockProducts_EdgeCases(t *testing.T) {
	t.Run("inventory nil", func(t *testing.T) {
		svc := NewAnalyticsService(nil, nil, nil, nil, nil)
		_, err := svc.GetLowStockProducts(context.Background(), analyticsdto.LowStockProductsQuery{})
		assert.Error(t, err)
	})

	t.Run("critical and warning alerts", func(t *testing.T) {
		catalog := stubCatalog{products: []catalogcontract.ProductItemInfo{
			{
				ProductID:           1,
				ProductUUID:         uuid.New().String(),
				ProductName:         "Critical Item",
				ProductSlug:         "critical-item",
				ProductPrice:        10.0,
				ProductCategoryName: "Category",
			},
			{
				ProductID:           2,
				ProductUUID:         uuid.New().String(),
				ProductName:         "Warning Item",
				ProductSlug:         "warning-item",
				ProductPrice:        20.0,
				ProductCategoryName: "Category",
			},
		}}
		inventory := stubInventory{
			items: []inventorycontract.StockItemInfo{
				{ProductID: 1, AvailableQuantity: 0},
				{ProductID: 2, AvailableQuantity: 3},
			},
			total: 2,
		}
		svc := NewAnalyticsService(nil, nil, catalog, inventory, nil)

		res, err := svc.GetLowStockProducts(context.Background(), analyticsdto.LowStockProductsQuery{})
		assert.NoError(t, err)
		assert.Len(t, res.Items, 2)
		assert.Equal(t, "critical", res.Items[0].AlertLevel)
		assert.Equal(t, "warning", res.Items[1].AlertLevel)
	})
}

func TestAnalyticsService_CalculateGrowth(t *testing.T) {
	assert.Equal(t, 100.0, calculateGrowth(50, 0))
	assert.Equal(t, 0.0, calculateGrowth(0, 0))
	assert.Equal(t, 50.0, calculateGrowth(150, 100))
	assert.Equal(t, -25.0, calculateGrowth(75, 100))
}

type mockEventRepo struct {
	mock.Mock
}

func (m *mockEventRepo) InsertEvents(ctx context.Context, events []repository.EngagementEvent) (int, error) {
	args := m.Called(ctx, events)
	return args.Get(0).(int), args.Error(1)
}

func (m *mockEventRepo) ListSearchHistory(ctx context.Context, subjectKind, subjectID string, limit int32) ([]repository.SearchHistoryEntry, error) {
	args := m.Called(ctx, subjectKind, subjectID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.SearchHistoryEntry), args.Error(1)
}

func (m *mockEventRepo) DismissSearch(ctx context.Context, subjectKind, subjectID, query string) error {
	args := m.Called(ctx, subjectKind, subjectID, query)
	return args.Error(0)
}

func (m *mockEventRepo) DismissAllSearches(ctx context.Context, subjectKind, subjectID string) error {
	args := m.Called(ctx, subjectKind, subjectID)
	return args.Error(0)
}

type mockBroker struct {
	messaging.Broker
	mock.Mock
}

func (m *mockBroker) PublishTopic(ctx context.Context, topic string, event any) error {
	args := m.Called(ctx, topic, event)
	return args.Error(0)
}

func TestEventService_Operations(t *testing.T) {
	uUUID := uuid.New().String()

	t.Run("Ingest success and telemetry fanout", func(t *testing.T) {
		repo := new(mockEventRepo)
		broker := new(mockBroker)
		svc := NewEventService(repo, broker)

		repo.On("InsertEvents", mock.Anything, mock.MatchedBy(func(evs []repository.EngagementEvent) bool {
			return len(evs) == 1 && evs[0].EventType == "search"
		})).Return(1, nil)
		broker.On("PublishTopic", mock.Anything, "search.event", mock.Anything).Return(nil)

		count, err := svc.Ingest(context.Background(), uUUID, 0, []analyticsdto.EngagementEventInput{
			buildInput(uuid.NewString()),
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
		repo.AssertExpectations(t)
		broker.AssertExpectations(t)
	})

	t.Run("Ingest empty and batch exceeded", func(t *testing.T) {
		svc := NewEventService(nil, nil)

		count, err := svc.Ingest(context.Background(), uUUID, 0, nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)

		batch := make([]analyticsdto.EngagementEventInput, MaxEventsPerBatch+1)
		_, err = svc.Ingest(context.Background(), uUUID, 0, batch)
		assert.Error(t, err)
	})

	t.Run("SearchHistory success and invalid subject", func(t *testing.T) {
		repo := new(mockEventRepo)
		svc := NewEventService(repo, nil)

		repo.On("ListSearchHistory", mock.Anything, "user", "42", int32(10)).Return([]repository.SearchHistoryEntry{
			{Query: "shoes", SearchCnt: 2, LastSeen: time.Now()},
		}, nil)

		hist, err := svc.SearchHistory(context.Background(), "", 42, 10)
		assert.NoError(t, err)
		assert.Len(t, hist, 1)

		_, err = svc.SearchHistory(context.Background(), "", 0, 10)
		assert.Error(t, err)
	})

	t.Run("DismissSearch and DismissAllSearches", func(t *testing.T) {
		repo := new(mockEventRepo)
		svc := NewEventService(repo, nil)

		repo.On("DismissSearch", mock.Anything, "user", "42", "shoes").Return(nil)
		repo.On("DismissAllSearches", mock.Anything, "user", "42").Return(nil)

		err := svc.DismissSearch(context.Background(), "", 42, " shoes ")
		assert.NoError(t, err)

		err = svc.DismissAllSearches(context.Background(), "", 42)
		assert.NoError(t, err)
	})
}
