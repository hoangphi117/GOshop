package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"goshop/internal/utils"
	"goshop/services/coupon/internal/db/sqlc"
	"goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CouponService interface {
	PreviewCoupon(ctx context.Context, input dto.PreviewCouponInput) (*dto.PreviewCouponDTO, error)
	ReserveCoupon(ctx context.Context, input dto.ReserveCouponInput) (string, error)
	ReleaseCoupon(ctx context.Context, input dto.ReleaseCouponInput) error

	GetByCode(ctx context.Context, code string) (*dto.CouponDTO, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (*dto.CouponDTO, error)
	List(ctx context.Context, search *string, couponType *int32, isActive *bool, limit, offset int32) ([]dto.CouponDTO, int64, error)
	Create(ctx context.Context, input dto.CreateCouponInput) (*dto.CouponDTO, error)
	Update(ctx context.Context, uuid uuid.UUID, input dto.UpdateCouponInput) (*dto.CouponDTO, error)
	Delete(ctx context.Context, uuid uuid.UUID) error
}

type couponService struct {
	repo   repository.CouponRepository
	dbPool *pgxpool.Pool
}

func NewCouponService(repo repository.CouponRepository, dbPool *pgxpool.Pool) CouponService {
	return &couponService{
		repo:   repo,
		dbPool: dbPool,
	}
}

func toPgUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func toPgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func (s *couponService) PreviewCoupon(ctx context.Context, input dto.PreviewCouponInput) (*dto.PreviewCouponDTO, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if code == "" {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon code is required")
	}

	coupon, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found: "+code)
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch coupon", err)
	}

	if !coupon.CouponIsActive {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
	}

	now := time.Now()
	if now.Before(coupon.CouponStartDate) {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet valid")
	}
	if now.After(coupon.CouponEndDate) {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
	}

	if coupon.CouponUsageLimit != nil && coupon.CouponUsedCount >= *coupon.CouponUsageLimit {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon usage limit reached")
	}

	if input.UserUUID != "" {
		userUUID, err := uuid.Parse(input.UserUUID)
		if err == nil {
			userUsageCount, err := s.repo.CountUserUsages(ctx, sqlc.CountUserCouponUsagesParams{
				CouponUsageCouponID: coupon.CouponID,
				CouponUsageUserID:   0,
				UserUuid:            toPgUUID(&userUUID),
			})
			if err == nil && int32(userUsageCount) >= coupon.CouponUserUsageLimit {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "You have reached the maximum usage limit for this coupon")
			}
		}
	}

	minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)
	if input.SubtotalAmount < minOrderAmount {
		return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Order amount must be at least %.2f to use this coupon", minOrderAmount))
	}

	discountValue := utils.NumericToFloat(coupon.CouponDiscountValue)
	var maxDiscount *float64
	if coupon.CouponMaxDiscountAmount.Valid {
		v := utils.NumericToFloat(coupon.CouponMaxDiscountAmount)
		maxDiscount = &v
	}

	var discountAmount float64
	switch coupon.CouponType {
	case dto.CouponTypePercentage:
		discountAmount = input.SubtotalAmount * (discountValue / 100.0)
		if maxDiscount != nil && discountAmount > *maxDiscount {
			discountAmount = *maxDiscount
		}
	case dto.CouponTypeFixedAmount:
		discountAmount = discountValue
		if discountAmount > input.SubtotalAmount {
			discountAmount = input.SubtotalAmount
		}
	default:
		return nil, utils.NewError(utils.ErrCodeInternal, "Unknown coupon type")
	}

	finalTotalAmount := math.Max(0, input.SubtotalAmount-discountAmount)

	return &dto.PreviewCouponDTO{
		CouponID:         coupon.CouponID,
		CouponUUID:       coupon.CouponUuid.String(),
		CouponCode:       coupon.CouponCode,
		CouponName:       coupon.CouponName,
		CouponType:       coupon.CouponType,
		DiscountValue:    discountValue,
		SubtotalAmount:   input.SubtotalAmount,
		DiscountAmount:   discountAmount,
		FinalTotalAmount: finalTotalAmount,
		MinOrderAmount:   minOrderAmount,
	}, nil
}

func (s *couponService) ReserveCoupon(ctx context.Context, input dto.ReserveCouponInput) (string, error) {
	if input.CouponID == 0 && input.CouponUUID != "" {
		parsed, err := uuid.Parse(input.CouponUUID)
		if err == nil {
			c, err := s.repo.GetByUUID(ctx, parsed)
			if err == nil {
				input.CouponID = c.CouponID
			}
		}
	}

	var tx pgx.Tx
	var err error
	if s.dbPool != nil {
		tx, err = s.dbPool.Begin(ctx)
		if err != nil {
			return "", utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
	}

	// 1. Atomic increment with limit check
	if err := s.repo.IncrementUsedCount(ctx, tx, input.CouponID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return "", utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
		}
		return "", utils.WrapError(utils.ErrCodeInternal, "Failed to increment coupon usage count", err)
	}

	// 2. Record usage with unique constraints
	var couponUUID *uuid.UUID
	if input.CouponUUID != "" {
		if parsed, err := uuid.Parse(input.CouponUUID); err == nil {
			couponUUID = &parsed
		}
	}
	var userUUID *uuid.UUID
	if input.UserUUID != "" {
		if parsed, err := uuid.Parse(input.UserUUID); err == nil {
			userUUID = &parsed
		}
	}
	var orderUUID *uuid.UUID
	if input.OrderUUID != "" {
		if parsed, err := uuid.Parse(input.OrderUUID); err == nil {
			orderUUID = &parsed
		}
	}
	var orderID *int32
	if input.OrderID != 0 {
		orderID = &input.OrderID
	}

	usage, err := s.repo.RecordUsage(ctx, tx, sqlc.RecordCouponUsageParams{
		CouponUsageCouponID:   input.CouponID,
		CouponUsageCouponUuid: toPgUUID(couponUUID),
		CouponUsageUserID:     input.UserID,
		CouponUsageUserUuid:   toPgUUID(userUUID),
		CouponUsageOrderID:    orderID,
		CouponUsageOrderUuid:  toPgUUID(orderUUID),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", utils.NewError(utils.ErrCodeConflict, "Coupon has already been used for this order")
		}
		return "", utils.WrapError(utils.ErrCodeInternal, "Failed to record coupon usage", err)
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return "", utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}
		tx = nil
	}

	return usage.CouponUsageUuid.String(), nil
}

func (s *couponService) ReleaseCoupon(ctx context.Context, input dto.ReleaseCouponInput) error {
	var tx pgx.Tx
	var err error
	if s.dbPool != nil {
		tx, err = s.dbPool.Begin(ctx)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
	}

	if input.CouponID != 0 {
		_ = s.repo.DecrementUsedCount(ctx, tx, input.CouponID)
	}

	var orderUUID *uuid.UUID
	if input.OrderUUID != "" {
		if parsed, err := uuid.Parse(input.OrderUUID); err == nil {
			orderUUID = &parsed
		}
	}
	var orderID *int32
	if input.OrderID != 0 {
		orderID = &input.OrderID
	}

	if err := s.repo.DeleteUsageByOrder(ctx, tx, sqlc.DeleteCouponUsageByOrderParams{
		OrderID:   orderID,
		OrderUuid: toPgUUID(orderUUID),
	}); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to delete coupon usage", err)
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}
		tx = nil
	}
	return nil
}

func (s *couponService) GetByCode(ctx context.Context, code string) (*dto.CouponDTO, error) {
	c, err := s.repo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return nil, err
	}
	return toCouponDTO(c), nil
}

func (s *couponService) GetByUUID(ctx context.Context, u uuid.UUID) (*dto.CouponDTO, error) {
	c, err := s.repo.GetByUUID(ctx, u)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return nil, err
	}
	return toCouponDTO(c), nil
}

func (s *couponService) List(ctx context.Context, search *string, couponType *int32, isActive *bool, limit, offset int32) ([]dto.CouponDTO, int64, error) {
	coupons, err := s.repo.List(ctx, sqlc.ListCouponsParams{
		Search:   search,
		Type:     couponType,
		IsActive: isActive,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx, sqlc.CountCouponsParams{
		Search:   search,
		Type:     couponType,
		IsActive: isActive,
	})
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.CouponDTO, len(coupons))
	for i, c := range coupons {
		dtos[i] = *toCouponDTO(c)
	}
	return dtos, total, nil
}

func (s *couponService) Create(ctx context.Context, input dto.CreateCouponInput) (*dto.CouponDTO, error) {
	c, err := s.repo.Create(ctx, sqlc.CreateCouponParams{
		CouponCode:              strings.ToUpper(strings.TrimSpace(input.Code)),
		CouponName:              input.Name,
		CouponDescription:       &input.Description,
		CouponType:              input.Type,
		CouponDiscountValue:     utils.FloatToNumeric(input.DiscountValue),
		CouponMaxDiscountAmount: utils.FloatPointerToNumeric(input.MaxDiscountAmount),
		CouponMinOrderAmount:    utils.FloatToNumeric(input.MinOrderAmount),
		CouponUsageLimit:        input.UsageLimit,
		CouponUserUsageLimit:    input.UserUsageLimit,
		CouponStartDate:         input.StartDate,
		CouponEndDate:           input.EndDate,
		CouponIsActive:          input.IsActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "Coupon code already exists")
		}
		return nil, err
	}
	return toCouponDTO(c), nil
}

func (s *couponService) Update(ctx context.Context, u uuid.UUID, input dto.UpdateCouponInput) (*dto.CouponDTO, error) {
	c, err := s.repo.Update(ctx, sqlc.UpdateCouponParams{
		CouponUuid:              u,
		CouponName:              input.Name,
		CouponDescription:       input.Description,
		CouponType:              input.Type,
		CouponDiscountValue:     utils.FloatPointerToNumeric(input.DiscountValue),
		CouponMaxDiscountAmount: utils.FloatPointerToNumeric(input.MaxDiscountAmount),
		CouponMinOrderAmount:    utils.FloatPointerToNumeric(input.MinOrderAmount),
		CouponUsageLimit:        input.UsageLimit,
		CouponUserUsageLimit:    input.UserUsageLimit,
		CouponStartDate:         toPgTimestamptz(input.StartDate),
		CouponEndDate:           toPgTimestamptz(input.EndDate),
		CouponIsActive:          input.IsActive,
	})
	if err != nil {
		return nil, err
	}
	return toCouponDTO(c), nil
}

func (s *couponService) Delete(ctx context.Context, u uuid.UUID) error {
	return s.repo.Delete(ctx, u)
}

func toCouponDTO(c sqlc.Coupon) *dto.CouponDTO {
	typeLabel := "Percentage"
	if c.CouponType == dto.CouponTypeFixedAmount {
		typeLabel = "Fixed Amount"
	}

	var maxDiscount *float64
	if c.CouponMaxDiscountAmount.Valid {
		val := utils.NumericToFloat(c.CouponMaxDiscountAmount)
		maxDiscount = &val
	}

	return &dto.CouponDTO{
		ID:                c.CouponID,
		UUID:              c.CouponUuid.String(),
		Code:              c.CouponCode,
		Name:              c.CouponName,
		Description:       utils.NilToString(c.CouponDescription),
		Type:              c.CouponType,
		TypeLabel:         typeLabel,
		DiscountValue:     utils.NumericToFloat(c.CouponDiscountValue),
		MaxDiscountAmount: maxDiscount,
		MinOrderAmount:    utils.NumericToFloat(c.CouponMinOrderAmount),
		UsageLimit:        c.CouponUsageLimit,
		UsedCount:         c.CouponUsedCount,
		UserUsageLimit:    c.CouponUserUsageLimit,
		StartDate:         c.CouponStartDate.Format(time.RFC3339),
		EndDate:           c.CouponEndDate.Format(time.RFC3339),
		IsActive:          c.CouponIsActive,
		IsExpired:         time.Now().After(c.CouponEndDate),
	}
}
