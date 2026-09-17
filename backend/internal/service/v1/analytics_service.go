package v1service

import (
	"context"
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

func (s *analyticsService) GetOverview(ctx context.Context) (v1dto.OverviewStatsDTO, error) {
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

func (s *analyticsService) GetRevenueChart(ctx context.Context, period string, days, months int32) ([]v1dto.RevenueChartPointDTO, error) {
	if period == "monthly" {
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

func (s *analyticsService) GetTopProducts(ctx context.Context, days, limit int32) ([]v1dto.TopProductDTO, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 {
		limit = 5
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
