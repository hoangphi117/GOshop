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

// withTx runs fn inside a transaction. When the service has no pool (unit
// tests with a fake repository) fn runs without a transaction.
func (s *couponService) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	if s.dbPool == nil {
		return fn(nil)
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer func() {
		// Rollback after a successful commit returns ErrTxClosed; nothing to do.
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}
	return nil
}

// reservationIDs normalises the identifiers the callers may or may not send.
// CouponID/OrderID are the legacy integer forms; UUIDs are canonical.
func reservationIDs(orderID int32, orderUUID string) (*int32, *uuid.UUID, error) {
	var idPtr *int32
	if orderID != 0 {
		id := orderID
		idPtr = &id
	}

	if strings.TrimSpace(orderUUID) == "" {
		if idPtr == nil {
			return nil, nil, utils.NewError(utils.ErrCodeBadRequest, "Order id or order uuid is required")
		}
		return idPtr, nil, nil
	}

	parsed, err := uuid.Parse(strings.TrimSpace(orderUUID))
	if err != nil {
		return nil, nil, utils.WrapError(utils.ErrCodeBadRequest, "Invalid order uuid", err)
	}
	return idPtr, &parsed, nil
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ReserveCoupon consumes quota for exactly one (coupon, order) reservation.
// Safe under retries and concurrent duplicates: the usage row is the
// reservation identity, so a second Reserve for the same order returns the
// existing usage instead of taking quota again.
func (s *couponService) ReserveCoupon(ctx context.Context, input dto.ReserveCouponInput) (string, error) {
	orderID, orderUUID, err := reservationIDs(input.OrderID, input.OrderUUID)
	if err != nil {
		return "", err
	}
	if orderUUID == nil {
		// Reservations are keyed by order uuid; an integer-only order id cannot
		// be made idempotent and would allow unlimited retries to burn quota.
		return "", utils.NewError(utils.ErrCodeBadRequest, "Order uuid is required to reserve a coupon")
	}

	userUUID, err := parseOptionalUUID(input.UserUUID)
	if err != nil {
		return "", utils.WrapError(utils.ErrCodeBadRequest, "Invalid user uuid", err)
	}
	couponUUID, err := parseOptionalUUID(input.CouponUUID)
	if err != nil {
		return "", utils.WrapError(utils.ErrCodeBadRequest, "Invalid coupon uuid", err)
	}
	if input.CouponID == 0 && couponUUID == nil {
		return "", utils.NewError(utils.ErrCodeBadRequest, "Coupon id or coupon uuid is required")
	}

	var usageUUID string
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		couponID := input.CouponID
		if couponID == 0 {
			c, lookupErr := s.repo.GetByUUID(ctx, *couponUUID)
			if lookupErr != nil {
				return mapCouponLookupError(lookupErr)
			}
			couponID = c.CouponID
		}

		// Lock the coupon row so quota checks and the increment are serialised.
		if _, lockErr := s.repo.GetByIDForUpdate(ctx, tx, couponID); lockErr != nil {
			return mapCouponLookupError(lockErr)
		}

		usage, upsertErr := s.repo.UpsertUsage(ctx, tx, sqlc.UpsertCouponUsageParams{
			CouponID:   couponID,
			OrderUuid:  toPgUUID(orderUUID),
			CouponUuid: toPgUUID(couponUUID),
			UserID:     input.UserID,
			UserUuid:   toPgUUID(userUUID),
			OrderID:    orderID,
		})
		if upsertErr != nil {
			var pgErr *pgconn.PgError
			if errors.As(upsertErr, &pgErr) && pgErr.Code == "23505" {
				// Same order claimed by another coupon: still a conflict, but a
				// different identity than the coupon+order reservation.
				return utils.NewError(utils.ErrCodeConflict, "Coupon has already been used for this order")
			}
			return utils.WrapError(utils.ErrCodeInternal, "Failed to record coupon usage", upsertErr)
		}

		if !usage.Inserted {
			if usage.CouponUsageStatus == dto.CouponUsageStatusReleased {
				return utils.NewError(utils.ErrCodeConflict, "Coupon reservation for this order was released and cannot be reused")
			}
			usageUUID = usage.CouponUsageUuid.String()
			return nil
		}

		if incErr := s.repo.IncrementUsedCount(ctx, tx, couponID); incErr != nil {
			if errors.Is(incErr, pgx.ErrNoRows) || errors.Is(incErr, sql.ErrNoRows) {
				return utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
			}
			return utils.WrapError(utils.ErrCodeInternal, "Failed to increment coupon usage count", incErr)
		}

		usageUUID = usage.CouponUsageUuid.String()
		return nil
	})
	if err != nil {
		return "", err
	}
	return usageUUID, nil
}

// ReleaseCoupon tombstones the reservation and refunds quota at most once.
// A duplicate Release, or a Release that arrives before its Reserve, flips no
// row and therefore refunds nothing.
func (s *couponService) ReleaseCoupon(ctx context.Context, input dto.ReleaseCouponInput) error {
	orderID, orderUUID, err := reservationIDs(input.OrderID, input.OrderUUID)
	if err != nil {
		return err
	}

	return s.withTx(ctx, func(tx pgx.Tx) error {
		usage, releaseErr := s.repo.ReleaseUsage(ctx, tx, sqlc.ReleaseCouponUsageParams{
			OrderID:   orderID,
			OrderUuid: toPgUUID(orderUUID),
		})
		if releaseErr != nil {
			if errors.Is(releaseErr, pgx.ErrNoRows) || errors.Is(releaseErr, sql.ErrNoRows) {
				// Already released, or never reserved. Nothing to refund.
				return nil
			}
			return utils.WrapError(utils.ErrCodeInternal, "Failed to release coupon usage", releaseErr)
		}

		// Only the reservation that still consumed quota refunds it.
		if decErr := s.repo.DecrementUsedCount(ctx, tx, usage.CouponUsageCouponID); decErr != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to decrement coupon usage count", decErr)
		}
		return nil
	})
}

func mapCouponLookupError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to fetch coupon", err)
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