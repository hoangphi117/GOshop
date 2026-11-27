package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analyticsdto "goshop/services/analytics/internal/dto"
	analyticshandler "goshop/services/analytics/internal/handler"
	analyticsservice "goshop/services/analytics/internal/service"
	"goshop/shared/pkg/utils"
)

type mockAnalyticsService struct {
	analyticsservice.AnalyticsService
	getOverviewFn     func(ctx context.Context, q analyticsdto.OverviewStatsQuery) (analyticsdto.OverviewStatsDTO, error)
	getRevenueChartFn func(ctx context.Context, q analyticsdto.RevenueChartQuery) ([]analyticsdto.RevenueChartPointDTO, error)
	getRecentOrdersFn func(ctx context.Context, limit int32) ([]analyticsdto.RecentOrderDTO, error)
	getTopProductsFn  func(ctx context.Context, q analyticsdto.TopProductsQuery) ([]analyticsdto.TopProductDTO, error)
	getOrderDistFn    func(ctx context.Context, q analyticsdto.OrderStatusDistributionQuery) (analyticsdto.OrderStatusDistributionResponse, error)
	getLowStockFn     func(ctx context.Context, q analyticsdto.LowStockProductsQuery) (analyticsdto.LowStockProductResponse, error)
	getPaymentBreakFn func(ctx context.Context, q analyticsdto.PaymentMethodBreakdownQuery) (analyticsdto.PaymentMethodBreakdownResponse, error)
}

func (m *mockAnalyticsService) GetOverview(ctx context.Context, q analyticsdto.OverviewStatsQuery) (analyticsdto.OverviewStatsDTO, error) {
	if m.getOverviewFn != nil {
		return m.getOverviewFn(ctx, q)
	}
	return analyticsdto.OverviewStatsDTO{TotalRevenue: 1000}, nil
}
func (m *mockAnalyticsService) GetRevenueChart(ctx context.Context, q analyticsdto.RevenueChartQuery) ([]analyticsdto.RevenueChartPointDTO, error) {
	if m.getRevenueChartFn != nil {
		return m.getRevenueChartFn(ctx, q)
	}
	return []analyticsdto.RevenueChartPointDTO{{Date: "2026-11-20", Revenue: 500}}, nil
}
func (m *mockAnalyticsService) GetRecentOrders(ctx context.Context, limit int32) ([]analyticsdto.RecentOrderDTO, error) {
	if m.getRecentOrdersFn != nil {
		return m.getRecentOrdersFn(ctx, limit)
	}
	return []analyticsdto.RecentOrderDTO{{OrderNumber: "ORD-001"}}, nil
}
func (m *mockAnalyticsService) GetTopProducts(ctx context.Context, q analyticsdto.TopProductsQuery) ([]analyticsdto.TopProductDTO, error) {
	if m.getTopProductsFn != nil {
		return m.getTopProductsFn(ctx, q)
	}
	return []analyticsdto.TopProductDTO{{Name: "Shoes", TotalUnitsSold: 20}}, nil
}
func (m *mockAnalyticsService) GetOrderStatusDistribution(ctx context.Context, q analyticsdto.OrderStatusDistributionQuery) (analyticsdto.OrderStatusDistributionResponse, error) {
	if m.getOrderDistFn != nil {
		return m.getOrderDistFn(ctx, q)
	}
	return analyticsdto.OrderStatusDistributionResponse{TotalOrders: 10}, nil
}
func (m *mockAnalyticsService) GetLowStockProducts(ctx context.Context, q analyticsdto.LowStockProductsQuery) (analyticsdto.LowStockProductResponse, error) {
	if m.getLowStockFn != nil {
		return m.getLowStockFn(ctx, q)
	}
	return analyticsdto.LowStockProductResponse{TotalLow: 1}, nil
}
func (m *mockAnalyticsService) GetPaymentMethodBreakdown(ctx context.Context, q analyticsdto.PaymentMethodBreakdownQuery) (analyticsdto.PaymentMethodBreakdownResponse, error) {
	if m.getPaymentBreakFn != nil {
		return m.getPaymentBreakFn(ctx, q)
	}
	return analyticsdto.PaymentMethodBreakdownResponse{}, nil
}

func setupAnalyticsRouter(h *analyticshandler.AnalyticsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/admin/analytics/overview", h.GetOverview)
	r.GET("/api/v1/admin/analytics/revenue-chart", h.GetRevenueChart)
	r.GET("/api/v1/admin/analytics/recent-orders", h.GetRecentOrders)
	r.GET("/api/v1/admin/analytics/top-products", h.GetTopProducts)
	r.GET("/api/v1/admin/analytics/order-status-distribution", h.GetOrderStatusDistribution)
	r.GET("/api/v1/admin/analytics/low-stock-products", h.GetLowStockProducts)
	r.GET("/api/v1/admin/analytics/payment-methods", h.GetPaymentMethodBreakdown)
	return r
}

func TestAnalyticsHandler_Endpoints(t *testing.T) {
	mockSvc := &mockAnalyticsService{}
	r := setupAnalyticsRouter(analyticshandler.NewAnalyticsHandler(mockSvc))

	t.Run("GetOverview returns 200 with metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/overview?days=30", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "1000")
	})

	t.Run("GetRevenueChart returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/revenue-chart?period=daily", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "2026-11-20")
	})

	t.Run("GetRecentOrders returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/recent-orders?limit=5", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ORD-001")
	})

	t.Run("GetTopProducts returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/top-products?days=14", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Shoes")
	})

	t.Run("GetOrderStatusDistribution returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/order-status-distribution", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetLowStockProducts returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/low-stock-products?threshold=5", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetPaymentMethodBreakdown returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/payment-methods", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetOverview handles service error with 500", func(t *testing.T) {
		failSvc := &mockAnalyticsService{
			getOverviewFn: func(context.Context, analyticsdto.OverviewStatsQuery) (analyticsdto.OverviewStatsDTO, error) {
				return analyticsdto.OverviewStatsDTO{}, utils.NewError(utils.ErrCodeInternal, "db failure")
			},
		}
		failR := setupAnalyticsRouter(analyticshandler.NewAnalyticsHandler(failSvc))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/overview", nil)
		w := httptest.NewRecorder()
		failR.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
