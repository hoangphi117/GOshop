package repository

import (
	"context"
	"goshop/services/coupon/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CouponRepository interface {
	GetByCode(ctx context.Context, code string) (sqlc.Coupon, error)
	GetByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (sqlc.Coupon, error)
	GetByID(ctx context.Context, id int32) (sqlc.Coupon, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int32) (sqlc.Coupon, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	List(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error)
	Count(ctx context.Context, params sqlc.CountCouponsParams) (int64, error)
	Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	Delete(ctx context.Context, uuid uuid.UUID) error
	IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error
	DecrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error
	CountUserUsages(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error)
	UpsertUsage(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error)
	ReleaseUsage(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error)
}

type SqlCouponRepository struct {
	q *sqlc.Queries
}

func NewSqlCouponRepository(q *sqlc.Queries) *SqlCouponRepository {
	return &SqlCouponRepository{q: q}
}

func (r *SqlCouponRepository) querier(tx pgx.Tx) *sqlc.Queries {
	if tx != nil {
		return r.q.WithTx(tx)
	}
	return r.q
}

func (r *SqlCouponRepository) GetByCode(ctx context.Context, code string) (sqlc.Coupon, error) {
	return r.q.GetCouponByCode(ctx, code)
}

func (r *SqlCouponRepository) GetByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (sqlc.Coupon, error) {
	return r.querier(tx).GetCouponByCodeForUpdate(ctx, code)
}

func (r *SqlCouponRepository) GetByID(ctx context.Context, id int32) (sqlc.Coupon, error) {
	return r.q.GetCouponByIDForUpdate(ctx, id)
}

func (r *SqlCouponRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int32) (sqlc.Coupon, error) {
	return r.querier(tx).GetCouponByIDForUpdate(ctx, id)
}

func (r *SqlCouponRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.q.GetCouponByUUID(ctx, uuid)
}

func (r *SqlCouponRepository) List(ctx context.Context, params sqlc.ListCouponsParams) ([]sqlc.Coupon, error) {
	return r.q.ListCoupons(ctx, params)
}

func (r *SqlCouponRepository) Count(ctx context.Context, params sqlc.CountCouponsParams) (int64, error) {
	return r.q.CountCoupons(ctx, params)
}

func (r *SqlCouponRepository) Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
	return r.q.CreateCoupon(ctx, params)
}

func (r *SqlCouponRepository) Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
	return r.q.UpdateCoupon(ctx, params)
}

func (r *SqlCouponRepository) Delete(ctx context.Context, uuid uuid.UUID) error {
	return r.q.DeleteCoupon(ctx, uuid)
}

func (r *SqlCouponRepository) IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	_, err := r.querier(tx).IncrementCouponUsedCount(ctx, couponID)
	return err
}

func (r *SqlCouponRepository) DecrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	_, err := r.querier(tx).DecrementCouponUsedCount(ctx, couponID)
	return err
}

func (r *SqlCouponRepository) CountUserUsages(ctx context.Context, params sqlc.CountUserCouponUsagesParams) (int64, error) {
	return r.q.CountUserCouponUsages(ctx, params)
}

func (r *SqlCouponRepository) UpsertUsage(ctx context.Context, tx pgx.Tx, params sqlc.UpsertCouponUsageParams) (sqlc.UpsertCouponUsageRow, error) {
	return r.querier(tx).UpsertCouponUsage(ctx, params)
}

func (r *SqlCouponRepository) ReleaseUsage(ctx context.Context, tx pgx.Tx, params sqlc.ReleaseCouponUsageParams) (sqlc.CouponUsage, error) {
	return r.querier(tx).ReleaseCouponUsage(ctx, params)
}
