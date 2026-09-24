package v1service

import (
	"context"
	"errors"
	"math"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
)

type analyticsService struct {
	repo repository.AnalyticsRepository
}

func NewAnalyticsService(repo repository.AnalyticsRepository) AnalyticsService {
	return &analyticsService{
		repo: repo,
	}
}

func (s *analyticsService) GetOverview(ctx context.Context, query v1dto.OverviewStatsQuery) (v1dto.OverviewStatsDTO, error) {
	if query.FromDate != "" || query.ToDate != "" {
		fromDate, toDate, err := resolveDateRange(query.Days, query.FromDate, query.ToDate)
		if err != nil {
			return v1dto.OverviewStatsDTO{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
		}
		duration := toDate.Sub(fromDate)
		prevFromDate := fromDate.Add(-duration)

		row, err := s.repo.GetOverviewCustom(ctx, fromDate, toDate, prevFromDate)
		if err != nil {
			return v1dto.OverviewStatsDTO{}, err
		}

		currRevenue := utils.NumericToFloat(row.Revenue)
		prevRevenue := utils.NumericToFloat(row.PrevRevenue)

		return v1dto.OverviewStatsDTO{
			TotalRevenue:   currRevenue,
			TotalOrders:    row.OrderCount,
			NewCustomers:   row.NewCustomers,
			PendingOrders:  row.PendingCount,
			RevenueGrowth:  calculateGrowth(currRevenue, prevRevenue),
			OrderGrowth:    calculateGrowth(float64(row.OrderCount), float64(row.PrevOrderCount)),
			CustomerGrowth: calculateGrowth(float64(row.NewCustomers), float64(row.PrevNewCustomers)),
		}, nil
	}

	row, err := s.repo.GetOverview(ctx)
	if err != nil {
		return v1dto.OverviewStatsDTO{}, err
	}

	currRevenue := utils.NumericToFloat(row.Revenue)
	prevRevenue := utils.NumericToFloat(row.PrevRevenue)

	return v1dto.OverviewStatsDTO{
		TotalRevenue:   currRevenue,
		TotalOrders:    row.OrderCount,
		NewCustomers:   row.NewCustomers,
		PendingOrders:  row.PendingCount,
		RevenueGrowth:  calculateGrowth(currRevenue, prevRevenue),
		OrderGrowth:    calculateGrowth(float64(row.OrderCount), float64(row.PrevOrderCount)),
		CustomerGrowth: calculateGrowth(float64(row.NewCustomers), float64(row.PrevNewCustomers)),
	}, nil
}

func (s *analyticsService) GetRevenueChart(ctx context.Context, query v1dto.RevenueChartQuery) ([]v1dto.RevenueChartPointDTO, error) {
	if query.Period == "monthly" {
		months := query.Months
		if months <= 0 {
			months = 12
		}
		rows, err := s.repo.GetRevenueMonthly(ctx, months)
		if err != nil {
			return nil, err
		}
		points := make([]v1dto.RevenueChartPointDTO, len(rows))
		for i, r := range rows {
			points[i] = v1dto.RevenueChartPointDTO{
				Date:       r.Month.Format("2006-01"),
				Revenue:    utils.NumericToFloat(r.TotalRevenue),
				OrderCount: r.TotalOrders,
			}
		}
		return points, nil
	}

	if query.FromDate != "" || query.ToDate != "" {
		fromDate, toDate, err := resolveDateRange(query.Days, query.FromDate, query.ToDate)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
		}
		rows, err := s.repo.GetRevenueDailyCustom(ctx, fromDate, toDate)
		if err != nil {
			return nil, err
		}
		points := make([]v1dto.RevenueChartPointDTO, len(rows))
		for i, r := range rows {
			points[i] = v1dto.RevenueChartPointDTO{
				Date:       r.Date.Format("2006-01-02"),
				Revenue:    utils.NumericToFloat(r.TotalRevenue),
				OrderCount: r.TotalOrders,
			}
		}
		return points, nil
	}

	days := query.Days
	if days <= 0 {
		days = 30
	}
	rows, err := s.repo.GetRevenueDaily(ctx, days)
	if err != nil {
		return nil, err
	}
	points := make([]v1dto.RevenueChartPointDTO, len(rows))
	for i, r := range rows {
		points[i] = v1dto.RevenueChartPointDTO{
			Date:       r.Date.Format("2006-01-02"),
			Revenue:    utils.NumericToFloat(r.TotalRevenue),
			OrderCount: r.TotalOrders,
		}
	}
	return points, nil
}

func (s *analyticsService) GetRecentOrders(ctx context.Context, limit int32) ([]v1dto.RecentOrderDTO, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.repo.GetRecentOrders(ctx, limit)
	if err != nil {
		return nil, err
	}
	orders := make([]v1dto.RecentOrderDTO, len(rows))
	for i, r := range rows {
		orders[i] = v1dto.RecentOrderDTO{
			UUID:          r.OrderUuid.String(),
			OrderNumber:   r.OrderNumber,
			CustomerName:  r.UserFullname,
			CustomerEmail: r.UserEmail,
			TotalAmount:   utils.NumericToFloat(r.OrderTotalAmount),
			PaymentMethod: r.OrderPaymentMethod,
			PaymentStatus: r.OrderPaymentStatus,
			Status:        r.OrderStatus,
			CreatedAt:     r.OrderCreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return orders, nil
}

func (s *analyticsService) GetTopProducts(ctx context.Context, query v1dto.TopProductsQuery) ([]v1dto.TopProductDTO, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}

	if query.FromDate != "" || query.ToDate != "" {
		fromDate, toDate, err := resolveDateRange(query.Days, query.FromDate, query.ToDate)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
		}
		rows, err := s.repo.GetTopSellingProductsCustom(ctx, fromDate, toDate, limit)
		if err != nil {
			return nil, err
		}
		products := make([]v1dto.TopProductDTO, len(rows))
		for i, r := range rows {
			products[i] = v1dto.TopProductDTO{
				UUID:           r.ProductUuid.String(),
				Name:           r.ProductName,
				Slug:           r.ProductSlug,
				Price:          utils.NumericToFloat(r.ProductPrice),
				ImageURL:       r.ProductImageUrl,
				CategoryName:   r.CategoryName,
				TotalUnitsSold: r.TotalUnitsSold,
				TotalRevenue:   utils.NumericToFloat(r.TotalSalesAmount),
			}
		}
		return products, nil
	}

	days := query.Days
	if days <= 0 {
		days = 30
	}
	rows, err := s.repo.GetTopSellingProducts(ctx, days, limit)
	if err != nil {
		return nil, err
	}
	products := make([]v1dto.TopProductDTO, len(rows))
	for i, r := range rows {
		products[i] = v1dto.TopProductDTO{
			UUID:           r.ProductUuid.String(),
			Name:           r.ProductName,
			Slug:           r.ProductSlug,
			Price:          utils.NumericToFloat(r.ProductPrice),
			ImageURL:       r.ProductImageUrl,
			CategoryName:   r.CategoryName,
			TotalUnitsSold: r.TotalUnitsSold,
			TotalRevenue:   utils.NumericToFloat(r.TotalSalesAmount),
		}
	}
	return products, nil
}

func calculateGrowth(current, previous float64) float64 {
	if previous <= 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return utils.RoundFloat(((current-previous)/previous)*100.0, 2)
}

func (s *analyticsService) GetOrderStatusDistribution(ctx context.Context, params v1dto.OrderStatusDistributionQuery) (v1dto.OrderStatusDistributionResponse, error) {
	fromDate, toDate, err := resolveDateRange(params.Days, params.FromDate, params.ToDate)
	if err != nil {
		return v1dto.OrderStatusDistributionResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	rows, err := s.repo.GetOrderStatusDistribution(ctx, fromDate, toDate)
	if err != nil {
		return v1dto.OrderStatusDistributionResponse{}, err
	}

	var totalOrders int64
	for _, r := range rows {
		totalOrders += r.Count
	}

	items := make([]v1dto.OrderStatusDistributionItemDTO, len(rows))
	for i, r := range rows {
		var percentage float64
		if totalOrders > 0 {
			percentage = utils.RoundFloat((float64(r.Count)/float64(totalOrders))*100.0, 2)
		}
		items[i] = v1dto.OrderStatusDistributionItemDTO{
			Status:      r.OrderStatus,
			StatusName:  v1dto.MapOrderStatusToString(r.OrderStatus),
			Count:       r.Count,
			TotalAmount: utils.NumericToFloat(r.TotalAmount),
			Percentage:  percentage,
		}
	}
	return v1dto.OrderStatusDistributionResponse{
		TotalOrders: totalOrders,
		Items:       items,
	}, nil
}

func (s *analyticsService) GetLowStockProducts(ctx context.Context, params v1dto.LowStockProductsQuery) (v1dto.LowStockProductResponse, error) {
	threshold := params.Threshold
	if threshold <= 0 {
		threshold = 5
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.repo.GetLowStockProducts(ctx, threshold, limit)
	if err != nil {
		return v1dto.LowStockProductResponse{}, err
	}

	products := make([]v1dto.LowStockProductDTO, len(rows))
	for i, r := range rows {
		stock := int32(0)
		if r.ProductStock != nil {
			stock = *r.ProductStock
		}

		alertLevel := "warning"
		if stock <= 0 {
			alertLevel = "critical"
		}

		categoryName := ""
		if r.CategoryName != nil {
			categoryName = *r.CategoryName
		}

		products[i] = v1dto.LowStockProductDTO{
			UUID:         r.ProductUuid.String(),
			Name:         r.ProductName,
			Slug:         r.ProductSlug,
			Price:        utils.NumericToFloat(r.ProductPrice),
			Stock:        stock,
			ImageURL:     r.ProductImageUrl,
			CategoryName: categoryName,
			AlertLevel:   alertLevel,
		}
	}
	return v1dto.LowStockProductResponse{
		Threshold: threshold,
		TotalLow:  int64(len(products)),
		Items:     products,
	}, nil
}

func (s *analyticsService) GetPaymentMethodBreakdown(ctx context.Context, params v1dto.PaymentMethodBreakdownQuery) (v1dto.PaymentMethodBreakdownResponse, error) {
	fromDate, toDate, err := resolveDateRange(params.Days, params.FromDate, params.ToDate)
	if err != nil {
		return v1dto.PaymentMethodBreakdownResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	rows, err := s.repo.GetPaymentMethodBreakdown(ctx, fromDate, toDate)
	if err != nil {
		return v1dto.PaymentMethodBreakdownResponse{}, err
	}

	// Calculate total revenue and total orders for percentage calculations
	var totalRevenue float64
	var totalOrders int64
	for _, r := range rows {
		totalRevenue += utils.NumericToFloat(r.NetRevenue)
		totalOrders += r.OrderCount
	}

	items := make([]v1dto.PaymentMethodBreakdownItemDTO, len(rows))
	for i, r := range rows {
		netRevenue := utils.NumericToFloat(r.NetRevenue)

		var percentage float64
		if totalRevenue > 0 {
			percentage = utils.RoundFloat((netRevenue/totalRevenue)*100.0, 2)
		}

		items[i] = v1dto.PaymentMethodBreakdownItemDTO{
			PaymentMethod: r.OrderPaymentMethod,
			MethodName:    v1dto.MapPaymentMethodToString(r.OrderPaymentMethod),
			OrderCount:    r.OrderCount,
			NetRevenue:    netRevenue,
			Percentage:    percentage,
		}
	}

	return v1dto.PaymentMethodBreakdownResponse{
		TotalRevenue: utils.RoundFloat(totalRevenue, 2),
		TotalOrders:  totalOrders,
		Items:        items,
	}, nil
}

func resolveDateRange(days int32, fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now()

	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			return time.Time{}, time.Time{}, errors.New("both from_date and to_date must be provided")
		}
		from, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		to, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if from.After(to) {
			return time.Time{}, time.Time{}, errors.New("from_date must be before or equal to to_date")
		}
		// Set toDate to the end of the day (23:59:59.999999999)
		to = to.Add(24*time.Hour - time.Nanosecond)
		return from, to, nil
	}

	// Fallback
	if days <= 0 {
		days = 30
	}
	fromDate := now.AddDate(0, 0, -int(days))
	return fromDate, now, nil
}

func (s *analyticsService) GetCartFunnel(ctx context.Context, params v1dto.CartFunnelQuery) (*v1dto.CartFunnelDTO, error) {
	fromDate, toDate, err := resolveDateRange(int32(params.Days), params.FromDate, params.ToDate)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	row, err := s.repo.GetCartAbandonmentAndFunnelStats(ctx, fromDate, toDate)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to fetch cart funnel stats: ", err)
	}

	dto := &v1dto.CartFunnelDTO{
		TotalCarts:           row.TotalCarts,
		OrdersCreated:        row.OrdersCreated,
		OrdersPaid:           row.OrdersPaid,
		AbandonedCarts:       row.AbandonedCarts,
		PotentialLostRevenue: utils.NumericToFloat(row.PotentialLostRevenue),
	}

	if dto.TotalCarts > 0 {
		dto.AbandonmentRate = math.Round((float64(dto.AbandonedCarts)/float64(dto.TotalCarts))*1000) / 10
		dto.CartToOrderRate = math.Round((float64(dto.OrdersCreated)/float64(dto.TotalCarts))*1000) / 10
	}
	if dto.OrdersCreated > 0 {
		dto.OrderToPaidRate = math.Round((float64(dto.OrdersPaid)/float64(dto.OrdersCreated))*1000) / 10
	}

	return dto, nil
}

func (s *analyticsService) GetReturnAnalytics(ctx context.Context, params v1dto.ReturnAnalyticsQuery) (*v1dto.ReturnAnalyticsResponse, error) {
	fromDate, toDate, err := resolveDateRange(params.Days, params.FromDate, params.ToDate)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	summary, err := s.repo.GetReturnAnalyticsSummary(ctx, fromDate, toDate)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get return summary: ", err)
	}

	reasons, err := s.repo.GetReturnReasonsBreakdown(ctx, fromDate, toDate, 5)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get return reasons: ", err)
	}

	products, err := s.repo.GetTopReturnedProducts(ctx, fromDate, toDate, 5)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get top returned products: ", err)
	}

	resp := &v1dto.ReturnAnalyticsResponse{
		TotalReturns:        summary.TotalReturns,
		ReturnedOrdersCount: summary.ReturnedOrdersCount,
		TotalPaidOrders:     summary.TotalPaidOrders,
		TotalRefundedAmount: utils.NumericToFloat(summary.TotalRefundedAmount),
		ReasonsBreakdown:    make([]v1dto.ReturnReasonBreakdownDTO, 0, len(reasons)),
		TopReturnedProducts: make([]v1dto.TopReturnedProductDTO, 0, len(products)),
	}

	if summary.TotalPaidOrders > 0 {
		resp.ReturnRate = math.Round((float64(summary.ReturnedOrdersCount)/float64(summary.TotalPaidOrders))*1000) / 10
	}

	for _, r := range reasons {
		pct := 0.0
		if summary.TotalReturns > 0 {
			pct = math.Round((float64(r.ReasonCount)/float64(summary.TotalReturns))*1000) / 10
		}
		resp.ReasonsBreakdown = append(resp.ReasonsBreakdown, v1dto.ReturnReasonBreakdownDTO{
			Reason:     r.ReturnReason,
			Count:      r.ReasonCount,
			Percentage: pct,
		})
	}

	for _, p := range products {
		resp.TopReturnedProducts = append(resp.TopReturnedProducts, v1dto.TopReturnedProductDTO{
			UUID:            p.ProductUuid.String(),
			Name:            p.ProductName,
			Slug:            p.ProductSlug,
			Price:           utils.NumericToFloat(p.ProductPrice),
			ImageURL:        p.ProductImageUrl,
			ReturnCaseCount: p.ReturnCaseCount,
			UnitsReturned:   p.ReturnedUnitsCount,
			ReturnedValue:   utils.NumericToFloat(p.ReturnedValue),
		})
	}

	return resp, nil
}

