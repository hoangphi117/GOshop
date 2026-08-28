package repository

import (
	"context"
	"goshop/internal/db/sqlc"
)

type SqlCouponRepository struct {
	db sqlc.Querier
}

func NewSqlCouponRepository(db sqlc.Querier) CouponRepository {
	return &SqlCouponRepository{
		db: db,
	}
}

func (r *SqlCouponRepository) Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
	return r.db.CreateCoupon(ctx, params)
}

func (r *SqlCouponRepository) GetByCode(ctx context.Context, code string) (sqlc.Coupon, error) {
	return r.db.GetCouponByCode(ctx, code)
}
