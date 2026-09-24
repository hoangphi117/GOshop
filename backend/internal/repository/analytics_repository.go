package repository

import (
	"context"
	"goshop/internal/db/sqlc"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
		FromDate:   pgtype.Timestamptz{Time: fromDate, Valid: true},
		ToDate:     pgtype.Timestamptz{Time: toDate, Valid: true},
		LimitCount: limit,
	})
}

func (r *SqlAnalyticsRepository) GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
	return r.db.GetOrderStatusDistribution(ctx, sqlc.GetOrderStatusDistributionParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetLowStockProducts(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error) {
	return r.db.GetLowStockProducts(ctx, sqlc.GetLowStockProductsParams{
		Threshold: threshold,
		Limit:     limit,
	})
}
func (r *SqlAnalyticsRepository) GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
	return r.db.GetPaymentMethodBreakdown(ctx, sqlc.GetPaymentMethodBreakdownParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetCartAbandonmentAndFunnelStats(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error) {
	return r.db.GetCartAbandonmentAndFunnelStats(ctx, sqlc.GetCartAbandonmentAndFunnelStatsParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetReturnAnalyticsSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error) {
	return r.db.GetReturnAnalyticsSummary(ctx, sqlc.GetReturnAnalyticsSummaryParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetReturnReasonsBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetReturnReasonsBreakdownRow, error) {
	return r.db.GetReturnReasonsBreakdown(ctx, sqlc.GetReturnReasonsBreakdownParams{
		FromDate:   fromDate,
		ToDate:     toDate,
		LimitCount: limit,
	})
}

func (r *SqlAnalyticsRepository) GetTopReturnedProducts(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopReturnedProductsRow, error) {
	return r.db.GetTopReturnedProducts(ctx, sqlc.GetTopReturnedProductsParams{
		FromDate:   fromDate,
		ToDate:     toDate,
		LimitCount: limit,
	})
}

func (r *SqlAnalyticsRepository) GetCategorySalesBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error) {
	return r.db.GetCategorySalesBreakdown(ctx, sqlc.GetCategorySalesBreakdownParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetCouponPerformanceSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCouponPerformanceSummaryRow, error) {
	return r.db.GetCouponPerformanceSummary(ctx, sqlc.GetCouponPerformanceSummaryParams{
		FromDate: fromDate,
		ToDate:   toDate,
	})
}

func (r *SqlAnalyticsRepository) GetCouponCampaignBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error) {
	return r.db.GetCouponCampaignBreakdown(ctx, sqlc.GetCouponCampaignBreakdownParams{
		FromDate:   fromDate,
		ToDate:     toDate,
		LimitCount: limit,
	})
}



