package v1handler

import (
	"net/http"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	service v1service.AnalyticsService
}

func NewAnalyticsHandler(service v1service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		service: service,
	}
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
// @Success      200 {object} utils.APIResponse{data=v1dto.OverviewStatsDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/overview [get]
func (h *AnalyticsHandler) GetOverview(ctx *gin.Context) {
	var query v1dto.OverviewStatsQuery
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
// @Success      200 {object} utils.APIResponse{data=[]v1dto.RevenueChartPointDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/revenue-chart [get]
func (h *AnalyticsHandler) GetRevenueChart(ctx *gin.Context) {
	var query v1dto.RevenueChartQuery
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
// @Success      200 {object} utils.APIResponse{data=[]v1dto.RecentOrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/recent-orders [get]
func (h *AnalyticsHandler) GetRecentOrders(ctx *gin.Context) {
	var query v1dto.RecentOrdersQuery
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
// @Success      200 {object} utils.APIResponse{data=[]v1dto.TopProductDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/top-products [get]
func (h *AnalyticsHandler) GetTopProducts(ctx *gin.Context) {
	var query v1dto.TopProductsQuery
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
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderStatusDistributionResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/order-status-distribution [get]
func (h *AnalyticsHandler) GetOrderStatusDistribution(ctx *gin.Context) {
	var query v1dto.OrderStatusDistributionQuery
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
// @Success      200 {object} utils.APIResponse{data=v1dto.LowStockProductResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/low-stock-alerts [get]
func (h *AnalyticsHandler) GetLowStockProducts(ctx *gin.Context) {
	var query v1dto.LowStockProductsQuery
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
// @Success      200 {object} utils.APIResponse{data=v1dto.PaymentMethodBreakdownResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/payment-methods [get]
func (h *AnalyticsHandler) GetPaymentMethodBreakdown(ctx *gin.Context) {
	var query v1dto.PaymentMethodBreakdownQuery
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

// GetCartFunnel godoc
// @Summary      Get cart abandonment and conversion funnel statistics
// @Description  Get active carts, order drop-offs, abandonment rate, and potential lost revenue
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=v1dto.CartFunnelDTO}
// @Router       /admin/analytics/cart-funnel [get]
func (h *AnalyticsHandler) GetCartFunnel(ctx *gin.Context) {
	var query v1dto.CartFunnelQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetCartFunnel(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart funnel statistics retrieved successfully", res)
}

// GetReturnAnalytics godoc
// @Summary      Get return and refund financial analytics
// @Description  Get return rates, total refunded amount, reason breakdown, and top returned products
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Param        days      query int    false "Time window in days (default: 30)" default(30)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=v1dto.ReturnAnalyticsResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/returns [get]
func (h *AnalyticsHandler) GetReturnAnalytics(ctx *gin.Context) {
	var query v1dto.ReturnAnalyticsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetReturnAnalytics(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return and refund analytics retrieved successfully", res)
}

// GetCategorySalesBreakdown godoc
// @Summary      Get category sales breakdown
// @Description  Get sales volume, gross revenue, and revenue contribution percentages by category
// @Tags         Admin - Analytics
// @Produce      json
// @Security     BearerAuth
// @Param        days      query int    false "Time window in days (default: 30)" default(30)
// @Param        from_date query string false "Start date (YYYY-MM-DD)"
// @Param        to_date   query string false "End date (YYYY-MM-DD)"
// @Success      200 {object} utils.APIResponse{data=v1dto.CategorySalesBreakdownResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/analytics/category-sales [get]
func (h *AnalyticsHandler) GetCategorySalesBreakdown(ctx *gin.Context) {
	var query v1dto.CategorySalesBreakdownQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.service.GetCategorySalesBreakdown(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Category sales breakdown retrieved successfully", res)
}


