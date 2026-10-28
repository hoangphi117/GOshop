package service

import (
	"context"
	"errors"
	"time"

	"goshop/pkg/utils"
	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/repository"
	catalogcontract "goshop/services/catalog/contract"
)

type AnalyticsService interface {
	GetOverview(ctx context.Context, query analyticsdto.OverviewStatsQuery) (analyticsdto.OverviewStatsDTO, error)
	GetRevenueChart(ctx context.Context, query analyticsdto.RevenueChartQuery) ([]analyticsdto.RevenueChartPointDTO, error)
	GetRecentOrders(ctx context.Context, limit int32) ([]analyticsdto.RecentOrderDTO, error)
	GetTopProducts(ctx context.Context, query analyticsdto.TopProductsQuery) ([]analyticsdto.TopProductDTO, error)
	GetOrderStatusDistribution(ctx context.Context, params analyticsdto.OrderStatusDistributionQuery) (analyticsdto.OrderStatusDistributionResponse, error)
	GetLowStockProducts(ctx context.Context, params analyticsdto.LowStockProductsQuery) (analyticsdto.LowStockProductResponse, error)
	GetPaymentMethodBreakdown(ctx context.Context, params analyticsdto.PaymentMethodBreakdownQuery) (analyticsdto.PaymentMethodBreakdownResponse, error)
}

type analyticsService struct {
	repo      repository.AnalyticsRepository
	customers CustomerCounter
	catalog   ProductCatalog
	inventory InventoryStockReader
}

func NewAnalyticsService(
	repo repository.AnalyticsRepository,
	customers CustomerCounter,
	catalog ProductCatalog,
	inventory InventoryStockReader,
) AnalyticsService {
	return &analyticsService{
		repo:      repo,
		customers: customers,
		catalog:   catalog,
		inventory: inventory,
	}
}

// ---------------------------------------------------------------------------
// Overview: projection aggregates + identity new-customer counts
// ---------------------------------------------------------------------------

func (s *analyticsService) GetOverview(ctx context.Context, query analyticsdto.OverviewStatsQuery) (analyticsdto.OverviewStatsDTO, error) {
	if query.FromDate != "" || query.ToDate != "" {
		fromDate, toDate, err := resolveDateRange(query.Days, query.FromDate, query.ToDate)
		if err != nil {
			return analyticsdto.OverviewStatsDTO{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
		}
		duration := toDate.Sub(fromDate)
		prevFromDate := fromDate.Add(-duration)

		row, err := s.repo.GetOverviewCustom(ctx, fromDate, toDate, prevFromDate)
		if err != nil {
			return analyticsdto.OverviewStatsDTO{}, err
		}

		currRevenue := utils.NumericToFloat(row.Revenue)
		prevRevenue := utils.NumericToFloat(row.PrevRevenue)
		newCustomers, prevCustomers := s.customerCounts(ctx, fromDate, toDate, prevFromDate, fromDate)

		return analyticsdto.OverviewStatsDTO{
			TotalRevenue:   currRevenue,
			TotalOrders:    row.OrderCount,
			NewCustomers:   newCustomers,
			PendingOrders:  row.PendingCount,
			RevenueGrowth:  calculateGrowth(currRevenue, prevRevenue),
			OrderGrowth:    calculateGrowth(float64(row.OrderCount), float64(row.PrevOrderCount)),
			CustomerGrowth: calculateGrowth(float64(newCustomers), float64(prevCustomers)),
		}, nil
	}

	row, err := s.repo.GetOverview(ctx)
	if err != nil {
		return analyticsdto.OverviewStatsDTO{}, err
	}

	currRevenue := utils.NumericToFloat(row.Revenue)
	prevRevenue := utils.NumericToFloat(row.PrevRevenue)
	now := time.Now()
	newCustomers, prevCustomers := s.customerCounts(ctx, now.AddDate(0, 0, -30), now, now.AddDate(0, 0, -60), now.AddDate(0, 0, -30))

	return analyticsdto.OverviewStatsDTO{
		TotalRevenue:   currRevenue,
		TotalOrders:    row.OrderCount,
		NewCustomers:   newCustomers,
		PendingOrders:  row.PendingCount,
		RevenueGrowth:  calculateGrowth(currRevenue, prevRevenue),
		OrderGrowth:    calculateGrowth(float64(row.OrderCount), float64(row.PrevOrderCount)),
		CustomerGrowth: calculateGrowth(float64(newCustomers), float64(prevCustomers)),
	}, nil
}

// customerCounts composes the identity service counts for the current and
// previous window. A failed identity call degrades to 0 rather than failing
// the whole overview.
func (s *analyticsService) customerCounts(ctx context.Context, from, to, prevFrom, prevTo time.Time) (int64, int64) {
	if s.customers == nil {
		return 0, 0
	}
	curr, err := s.customers.CountCustomersCreatedBetween(ctx, from, to)
	if err != nil {
		curr = 0
	}
	prev, err := s.customers.CountCustomersCreatedBetween(ctx, prevFrom, prevTo)
	if err != nil {
		prev = 0
	}
	return curr, prev
}

// ---------------------------------------------------------------------------
// Revenue charts (projection-only)
// ---------------------------------------------------------------------------

func (s *analyticsService) GetRevenueChart(ctx context.Context, query analyticsdto.RevenueChartQuery) ([]analyticsdto.RevenueChartPointDTO, error) {
	if query.Period == "monthly" {
		months := query.Months
		if months <= 0 {
			months = 12
		}
		rows, err := s.repo.GetRevenueMonthly(ctx, months)
		if err != nil {
			return nil, err
		}
		points := make([]analyticsdto.RevenueChartPointDTO, len(rows))
		for i, r := range rows {
			points[i] = analyticsdto.RevenueChartPointDTO{
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
		points := make([]analyticsdto.RevenueChartPointDTO, len(rows))
		for i, r := range rows {
			points[i] = analyticsdto.RevenueChartPointDTO{
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
	points := make([]analyticsdto.RevenueChartPointDTO, len(rows))
	for i, r := range rows {
		points[i] = analyticsdto.RevenueChartPointDTO{
			Date:       r.Date.Format("2006-01-02"),
			Revenue:    utils.NumericToFloat(r.TotalRevenue),
			OrderCount: r.TotalOrders,
		}
	}
	return points, nil
}

// ---------------------------------------------------------------------------
// Recent orders (projection-only)
// ---------------------------------------------------------------------------

func (s *analyticsService) GetRecentOrders(ctx context.Context, limit int32) ([]analyticsdto.RecentOrderDTO, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.repo.GetRecentOrders(ctx, limit)
	if err != nil {
		return nil, err
	}
	orders := make([]analyticsdto.RecentOrderDTO, len(rows))
	for i, r := range rows {
		orders[i] = analyticsdto.RecentOrderDTO{
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

// ---------------------------------------------------------------------------
// Top products: projection aggregates + catalog enrichment
// ---------------------------------------------------------------------------

func (s *analyticsService) GetTopProducts(ctx context.Context, query analyticsdto.TopProductsQuery) ([]analyticsdto.TopProductDTO, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}

	var rows []struct {
		ProductID        int32
		TotalUnitsSold   int64
		TotalSalesAmount float64
	}
	if query.FromDate != "" || query.ToDate != "" {
		fromDate, toDate, err := resolveDateRange(query.Days, query.FromDate, query.ToDate)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
		}
		raw, err := s.repo.GetTopSellingProductsCustom(ctx, fromDate, toDate, limit)
		if err != nil {
			return nil, err
		}
		rows = make([]struct {
			ProductID        int32
			TotalUnitsSold   int64
			TotalSalesAmount float64
		}, len(raw))
		for i, r := range raw {
			rows[i] = struct {
				ProductID        int32
				TotalUnitsSold   int64
				TotalSalesAmount float64
			}{r.ProductID, r.TotalUnitsSold, utils.NumericToFloat(r.TotalSalesAmount)}
		}
	} else {
		days := query.Days
		if days <= 0 {
			days = 30
		}
		raw, err := s.repo.GetTopSellingProducts(ctx, days, limit)
		if err != nil {
			return nil, err
		}
		rows = make([]struct {
			ProductID        int32
			TotalUnitsSold   int64
			TotalSalesAmount float64
		}, len(raw))
		for i, r := range raw {
			rows[i] = struct {
				ProductID        int32
				TotalUnitsSold   int64
				TotalSalesAmount float64
			}{r.ProductID, r.TotalUnitsSold, utils.NumericToFloat(r.TotalSalesAmount)}
		}
	}

	// Enrich with catalog dimensions for the ids in the result set.
	ids := make([]int32, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ProductID)
	}
	byID, err := s.catalogProducts(ctx, ids)
	if err != nil {
		return nil, err
	}

	products := make([]analyticsdto.TopProductDTO, 0, len(rows))
	for _, r := range rows {
		p, ok := byID[r.ProductID]
		if !ok {
			continue // product deleted in catalog since projection
		}
		img := p.ProductImageURL
		var imgPtr *string
		if img != "" {
			imgPtr = &img
		}
		products = append(products, analyticsdto.TopProductDTO{
			UUID:           p.ProductUUID,
			Name:           p.ProductName,
			Slug:           p.ProductSlug,
			Price:          p.ProductPrice,
			ImageURL:       imgPtr,
			CategoryName:   p.ProductCategoryName,
			TotalUnitsSold: r.TotalUnitsSold,
			TotalRevenue:   r.TotalSalesAmount,
		})
	}
	return products, nil
}

// catalogProducts resolves product dimensions by ids (or all products when
// ids is empty, which the batch endpoint does not support; then nil map).
func (s *analyticsService) catalogProducts(ctx context.Context, ids []int32) (map[int32]catalogcontract.ProductItemInfo, error) {
	if len(ids) == 0 || s.catalog == nil {
		return map[int32]catalogcontract.ProductItemInfo{}, nil
	}
	products, err := s.catalog.GetProductsBatch(ctx, nil, ids)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to resolve catalog products", err)
	}
	byID := make(map[int32]catalogcontract.ProductItemInfo, len(products))
	for _, p := range products {
		byID[p.ProductID] = p
	}
	return byID, nil
}

// ---------------------------------------------------------------------------
// Order status distribution / payment methods (projection-only)
// ---------------------------------------------------------------------------

func (s *analyticsService) GetOrderStatusDistribution(ctx context.Context, params analyticsdto.OrderStatusDistributionQuery) (analyticsdto.OrderStatusDistributionResponse, error) {
	fromDate, toDate, err := resolveDateRange(params.Days, params.FromDate, params.ToDate)
	if err != nil {
		return analyticsdto.OrderStatusDistributionResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	rows, err := s.repo.GetOrderStatusDistribution(ctx, fromDate, toDate)
	if err != nil {
		return analyticsdto.OrderStatusDistributionResponse{}, err
	}

	var totalOrders int64
	for _, r := range rows {
		totalOrders += r.Count
	}

	items := make([]analyticsdto.OrderStatusDistributionItemDTO, len(rows))
	for i, r := range rows {
		var percentage float64
		if totalOrders > 0 {
			percentage = utils.RoundFloat((float64(r.Count)/float64(totalOrders))*100.0, 2)
		}
		items[i] = analyticsdto.OrderStatusDistributionItemDTO{
			Status:      r.OrderStatus,
			StatusName:  analyticsdto.MapOrderStatusToString(r.OrderStatus),
			Count:       r.Count,
			TotalAmount: utils.NumericToFloat(r.TotalAmount),
			Percentage:  percentage,
		}
	}
	return analyticsdto.OrderStatusDistributionResponse{
		TotalOrders: totalOrders,
		Items:       items,
	}, nil
}

// ---------------------------------------------------------------------------
// Low stock alerts: inventory RPC + catalog enrichment
// ---------------------------------------------------------------------------

func (s *analyticsService) GetLowStockProducts(ctx context.Context, params analyticsdto.LowStockProductsQuery) (analyticsdto.LowStockProductResponse, error) {
	threshold := params.Threshold
	if threshold <= 0 {
		threshold = 5
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	if s.inventory == nil {
		return analyticsdto.LowStockProductResponse{}, utils.NewError(utils.ErrCodeInternal, "Inventory service unavailable")
	}
	stocks, totalLow, err := s.inventory.ListLowStockProducts(ctx, threshold, limit)
	if err != nil {
		return analyticsdto.LowStockProductResponse{}, utils.WrapError(utils.ErrCodeInternal, "Failed to list low stock products", err)
	}

	ids := make([]int32, 0, len(stocks))
	for _, st := range stocks {
		if st.ProductID > 0 {
			ids = append(ids, st.ProductID)
		}
	}
	byID, err := s.catalogProducts(ctx, ids)
	if err != nil {
		return analyticsdto.LowStockProductResponse{}, err
	}

	items := make([]analyticsdto.LowStockProductDTO, 0, len(stocks))
	for _, st := range stocks {
		p, ok := byID[st.ProductID]
		if !ok {
			continue // product missing in catalog; not nameable, skip
		}
		alertLevel := "warning"
		if st.AvailableQuantity <= 0 {
			alertLevel = "critical"
		}
		img := p.ProductImageURL
		var imgPtr *string
		if img != "" {
			imgPtr = &img
		}
		items = append(items, analyticsdto.LowStockProductDTO{
			UUID:         p.ProductUUID,
			Name:         p.ProductName,
			Slug:         p.ProductSlug,
			Price:        p.ProductPrice,
			Stock:        st.AvailableQuantity,
			ImageURL:     imgPtr,
			CategoryName: p.ProductCategoryName,
			AlertLevel:   alertLevel,
		})
	}

	return analyticsdto.LowStockProductResponse{
		Threshold: threshold,
		TotalLow:  totalLow,
		Items:     items,
	}, nil
}

func (s *analyticsService) GetPaymentMethodBreakdown(ctx context.Context, params analyticsdto.PaymentMethodBreakdownQuery) (analyticsdto.PaymentMethodBreakdownResponse, error) {
	fromDate, toDate, err := resolveDateRange(params.Days, params.FromDate, params.ToDate)
	if err != nil {
		return analyticsdto.PaymentMethodBreakdownResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "Invalid date range", err)
	}

	rows, err := s.repo.GetPaymentMethodBreakdown(ctx, fromDate, toDate)
	if err != nil {
		return analyticsdto.PaymentMethodBreakdownResponse{}, err
	}

	var totalRevenue float64
	var totalOrders int64
	for _, r := range rows {
		totalRevenue += utils.NumericToFloat(r.NetRevenue)
		totalOrders += r.OrderCount
	}

	items := make([]analyticsdto.PaymentMethodBreakdownItemDTO, len(rows))
	for i, r := range rows {
		netRevenue := utils.NumericToFloat(r.NetRevenue)
		var percentage float64
		if totalRevenue > 0 {
			percentage = utils.RoundFloat((netRevenue/totalRevenue)*100.0, 2)
		}
		items[i] = analyticsdto.PaymentMethodBreakdownItemDTO{
			PaymentMethod: r.OrderPaymentMethod,
			MethodName:    analyticsdto.MapPaymentMethodToString(r.OrderPaymentMethod),
			OrderCount:    r.OrderCount,
			NetRevenue:    netRevenue,
			Percentage:    percentage,
		}
	}

	return analyticsdto.PaymentMethodBreakdownResponse{
		TotalRevenue: utils.RoundFloat(totalRevenue, 2),
		TotalOrders:  totalOrders,
		Items:        items,
	}, nil
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
		to = to.Add(24*time.Hour - time.Nanosecond)
		return from, to, nil
	}

	if days <= 0 {
		days = 30
	}
	fromDate := now.AddDate(0, 0, -int(days))
	return fromDate, now, nil
}