package repository

import (
	"context"
	"time"

	"goshop/services/analytics/internal/db/sqlc"
)

type AnalyticsRepository interface {
	GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error)
	GetOverviewCustom(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error)
	GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error)
	GetRevenueDailyCustom(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error)
	GetRevenueMonthly(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error)
	GetRecentOrders(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error)
	GetTopSellingProducts(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error)
	GetTopSellingProductsCustom(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error)
	GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error)
	GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error)
}

type SqlAnalyticsRepository struct {
	db sqlc.Querier
}

func NewSqlAnalyticsRepository(db sqlc.Querier) AnalyticsRepository {
	return &SqlAnalyticsRepository{db: db}
}

func (r *SqlAnalyticsRepository) GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
	return r.db.GetOverviewStats(ctx)
}

func (r *SqlAnalyticsRepository) GetOverviewCustom(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error) {
	return r.db.GetOverviewStatsCustom(ctx, sqlc.GetOverviewStatsCustomParams{
		FromDate:     fromDate,
		ToDate:       toDate,
		PrevFromDate: prevFromDate,
	})
}

func (r *SqlAnalyticsRepository) GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
	return r.db.GetRevenueDailyChart(ctx, days)
}

func (r *SqlAnalyticsRepository) GetRevenueDailyCustom(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error) {
	return r.db.GetRevenueDailyCustom(ctx, sqlc.GetRevenueDailyCustomParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetRevenueMonthly(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error) {
	return r.db.GetRevenueMonthlyChart(ctx, months)
}

func (r *SqlAnalyticsRepository) GetRecentOrders(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
	return r.db.GetRecentOrders(ctx, limit)
}

func (r *SqlAnalyticsRepository) GetTopSellingProducts(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
	return r.db.GetTopSellingProducts(ctx, sqlc.GetTopSellingProductsParams{
		Days:       days,
		LimitCount: limit,
	})
}

func (r *SqlAnalyticsRepository) GetTopSellingProductsCustom(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error) {
	return r.db.GetTopSellingProductsCustom(ctx, sqlc.GetTopSellingProductsCustomParams{
		FromDate:   fromDate,
		ToDate:     toDate,
		LimitCount: limit,
	})
}

func (r *SqlAnalyticsRepository) GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
	return r.db.GetOrderStatusDistribution(ctx, sqlc.GetOrderStatusDistributionParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
	return r.db.GetPaymentMethodBreakdown(ctx, sqlc.GetPaymentMethodBreakdownParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}
