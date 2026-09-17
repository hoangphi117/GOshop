package repository

import (
	"context"
	"goshop/internal/db/sqlc"
)

type SqlAnalyticsRepository struct {
	db sqlc.Querier
}

func NewSqlAnalyticsRepository(db sqlc.Querier) AnalyticsRepository {
	return &SqlAnalyticsRepository{db: db}
}

func (r *SqlAnalyticsRepository) GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
	return r.db.GetOverviewStats(ctx)
}

func (r *SqlAnalyticsRepository) GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
	return r.db.GetRevenueDailyChart(ctx, days)
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
