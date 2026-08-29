package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
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

func (r *SqlCouponRepository) Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
	return r.db.UpdateCoupon(ctx, params)
}

func (r *SqlCouponRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.GetCouponByUUID(ctx, uuid)
}

func (r *SqlCouponRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.SoftDeleteCouponByUUID(ctx, uuid)
}
