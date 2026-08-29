package repository

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"strings"

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

func (r *SqlCouponRepository) GetAll(ctx context.Context, search, orderBy, sort string, coupon_type *int32, is_active *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error) {
	var args []any
	paramIdx := 1

	whereClause := " WHERE 1=1"

	// Filter Deleted
	if deleted {
		whereClause += " AND coupon_deleted_at IS NOT NULL"
	} else {
		whereClause += " AND coupon_deleted_at IS NULL"
	}

	// Search unaccent
	if strings.TrimSpace(search) != "" {
		whereClause += fmt.Sprintf(" AND (unaccent(coupon_code) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(coupon_name) ILIKE '%%' || unaccent($%d) || '%%')", paramIdx, paramIdx)
		args = append(args, strings.TrimSpace(search))
		paramIdx++
	}

	if coupon_type != nil {
		whereClause += fmt.Sprintf(" AND coupon_type = $%d", paramIdx)
		args = append(args, *coupon_type)
		paramIdx++
	}

	sortOrder := "DESC"
	if strings.ToLower(strings.TrimSpace(sort)) == "asc" {
		sortOrder = "ASC"
	}

	orderByColumn := "coupon_created_at"
	switch orderBy {
	case "coupon_id", "coupon_code", "coupon_discount_value", "coupon_used_count", "coupon_end_date", "coupon_created_at":
		orderByColumn = orderBy
	}

	query := fmt.Sprintf("SELECT * FROM coupons %s ORDER BY %s %s, coupon_id ASC LIMIT $%d OFFSET $%d",
		whereClause, orderByColumn, sortOrder, paramIdx, paramIdx+1)

	args = append(args, limit, offset)

	rows, err := db.DBPool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coupons := []sqlc.Coupon{}
	for rows.Next() {
		var i sqlc.Coupon
		if err := rows.Scan(
			&i.CouponID,
			&i.CouponUuid,
			&i.CouponCode,
			&i.CouponName,
			&i.CouponDescription,
			&i.CouponType,
			&i.CouponDiscountValue,
			&i.CouponMaxDiscountAmount,
			&i.CouponMinOrderAmount,
			&i.CouponUsageLimit,
			&i.CouponUsedCount,
			&i.CouponUserUsageLimit,
			&i.CouponStartDate,
			&i.CouponEndDate,
			&i.CouponIsActive,
			&i.CouponDeletedAt,
			&i.CouponCreatedAt,
			&i.CouponUpdatedAt,
		); err != nil {
			return nil, err
		}
		coupons = append(coupons, i)
	}
	return coupons, nil
}

func (r *SqlCouponRepository) GetByCode(ctx context.Context, code string) (sqlc.Coupon, error) {
	return r.db.GetCouponByCode(ctx, code)
}
func (r *SqlCouponRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.GetCouponByUUID(ctx, uuid)
}

func (r *SqlCouponRepository) Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
	return r.db.UpdateCoupon(ctx, params)
}

func (r *SqlCouponRepository) CountCoupons(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error) {
	return r.db.CountCoupons(ctx, sqlc.CountCouponsParams{
		Search:   &search,
		Type:     coupon_type,
		IsActive: isActive,
		Deleted:  &deleted,
	})
}

func (r *SqlCouponRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.SoftDeleteCouponByUUID(ctx, uuid)
}

func (r *SqlCouponRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.RestoreCoupon(ctx, uuid)
}

func (r *SqlCouponRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	return r.db.TrashCoupon(ctx, uuid)
}
