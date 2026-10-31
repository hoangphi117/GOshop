package handler

import (
	"net/http"

	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/service"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	service service.AnalyticsService
}

func NewAnalyticsHandler(service service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

// GetOverview godoc
// @Summary      Get analytics overview statistics
// @Description  Get KPI cards (revenue, orders, new customers, pending orders) with growth comparison for 30 days or custom date range
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        days      query int    false "Time window in days (default: 30)" default(30)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=analyticsdto.OverviewStatsDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/overview [get]
func (h *AnalyticsHandler) GetOverview(ctx *gin.Context) {
	var query analyticsdto.OverviewStatsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	stats, err := h.service.GetOverview(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Analytics overview retrieved successfully", stats)
}

// GetRevenueChart godoc
// @Summary      Get revenue chart data
// @Description  Get historical revenue and order count points aggregated by daily or monthly
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        period    query string false "Aggregation period: daily or monthly" Enums(daily, monthly) default(daily)
// @Param        days      query int    false "Number of days (for daily)" default(30)
// @Param        months    query int    false "Number of months (for monthly)" default(12)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=[]analyticsdto.RevenueChartPointDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/revenue-chart [get]
func (h *AnalyticsHandler) GetRevenueChart(ctx *gin.Context) {
	var query analyticsdto.RevenueChartQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	data, err := h.service.GetRevenueChart(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Revenue chart data retrieved successfully", data)
}

// GetRecentOrders godoc
// @Summary      Get recent orders for dashboard
// @Description  Retrieve the latest N orders with basic customer info
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        limit query int false "Max orders to return (1-50)" default(5)
// @Success      200 {object} utils.APIResponse{data=[]analyticsdto.RecentOrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/recent-orders [get]
func (h *AnalyticsHandler) GetRecentOrders(ctx *gin.Context) {
	var query analyticsdto.RecentOrdersQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, err := h.service.GetRecentOrders(ctx.Request.Context(), query.Limit)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Recent orders retrieved successfully", orders)
}

// GetTopProducts godoc
// @Summary      Get top selling products
// @Description  Get the top best-selling products by quantity and revenue over the last N days or custom date range
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        days      query int    false "Time window in days" default(30)
// @Param        limit     query int    false "Max items to return (1-50)" default(5)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=[]analyticsdto.TopProductDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/top-products [get]
func (h *AnalyticsHandler) GetTopProducts(ctx *gin.Context) {
	var query analyticsdto.TopProductsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	products, err := h.service.GetTopProducts(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Top selling products retrieved successfully", products)
}

// GetOrderStatusDistribution godoc
// @Summary      Get order status distribution
// @Description  Get order counts, revenue, and percentages grouped by order status with optional date filtering
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        days      query int    false "Time window in days (default: 30)" default(30)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=analyticsdto.OrderStatusDistributionResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/order-status-distribution [get]
func (h *AnalyticsHandler) GetOrderStatusDistribution(ctx *gin.Context) {
	var query analyticsdto.OrderStatusDistributionQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetOrderStatusDistribution(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order status distribution retrieved successfully", res)
}

// GetLowStockProducts godoc
// @Summary      Get low stock product alerts
// @Description  Get list of products with inventory stock below or equal to the specified threshold
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        threshold query int false "Inventory threshold alert limit (default: 5)" default(5)
// @Param        limit     query int false "Max items to return (1-50, default: 10)" default(10)
// @Success      200 {object} utils.APIResponse{data=analyticsdto.LowStockProductResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/low-stock-alerts [get]
func (h *AnalyticsHandler) GetLowStockProducts(ctx *gin.Context) {
	var query analyticsdto.LowStockProductsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetLowStockProducts(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Low stock products retrieved successfully", res)
}

// GetPaymentMethodBreakdown godoc
// @Summary      Get payment method breakdown
// @Description  Get net revenue, order volume, and percentages categorized by payment method
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        days      query int    false "Time window in days (default: 30)" default(30)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=analyticsdto.PaymentMethodBreakdownResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/analytics/payment-methods [get]
func (h *AnalyticsHandler) GetPaymentMethodBreakdown(ctx *gin.Context) {
	var query analyticsdto.PaymentMethodBreakdownQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetPaymentMethodBreakdown(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment method breakdown retrieved successfully", res)
}
