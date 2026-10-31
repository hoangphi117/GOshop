package service

import (
	"context"
	"time"

	"goshop/services/analytics/internal/db/sqlc"
	"goshop/services/analytics/internal/repository"
)

// repoUnused embeds the repository interface as a nil value: calling any
// method that a test did not override panics, which surfaces accidental
// SQL dependencies in composition code.
type repoUnused struct {
	repository.AnalyticsRepository
}

// overviewRepo serves only GetOverview for the KPI test.
type overviewRepo struct {
	repoUnused
	revenue, prevRevenue float64
	orders, prevOrders   int64
}

func (o overviewRepo) GetOverview(context.Context) (sqlc.GetOverviewStatsRow, error) {
	return sqlc.GetOverviewStatsRow{
		Revenue:        numeric(o.revenue),
		OrderCount:     o.orders,
		PrevRevenue:    numeric(o.prevRevenue),
		PrevOrderCount: o.prevOrders,
	}, nil
}

// counterFunc adapts a function to the CustomerCounter port.
type counterFunc func(ctx context.Context, from, to time.Time) (int64, error)

func (f counterFunc) CountCustomersCreatedBetween(ctx context.Context, from, to time.Time) (int64, error) {
	return f(ctx, from, to)
}
