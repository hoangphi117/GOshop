package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type couponService struct {
	repo     repository.CouponRepository
	userRepo repository.UserRepository
	cache    cache.RedisCacheService
}

func NewCouponService(repo repository.CouponRepository, userRepo repository.UserRepository, redisClient *redis.Client) CouponService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &couponService{
		repo:     repo,
		userRepo: userRepo,
		cache:    cacheService,
	}
}

func validateCouponDates(startDateStr, endDateStr string) (time.Time, time.Time, error) {
	startDate, err := time.Parse(time.RFC3339, startDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid start date format. Expected RFC3339 (e.g. 2026-08-28T10:00:00Z)")
	}
	endDate, err := time.Parse(time.RFC3339, endDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid end date format. Expected RFC3339 (e.g. 2026-09-28T10:00:00Z)")
	}
	now := time.Now()
	if !endDate.After(now) {
		return time.Time{}, time.Time{}, utils.NewError(utils.ErrCodeBadRequest, "End date must be in the future")
	}
	if !endDate.After(startDate) {
		return time.Time{}, time.Time{}, utils.NewError(utils.ErrCodeBadRequest, "End date must be strictly after start date")
	}
	return startDate, endDate, nil
}

func validateEndDate(endDateStr *string, startDate time.Time) (*time.Time, error) {
	if endDateStr == nil {
		return nil, nil
	}
	parsedEnd, err := time.Parse(time.RFC3339, *endDateStr)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid end date format")
	}
	if !parsedEnd.After(startDate) {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "End date must be after start date")
	}
	return &parsedEnd, nil
}

func validateDiscountConfig(couponType int32, discountValue float64, maxDiscountAmount *float64) error {
	switch couponType {
	case v1dto.CouponTypePercentage:
		if discountValue < 1 || discountValue > 100 {
			return utils.NewError(utils.ErrCodeBadRequest, "Percentage discount value must be between 1 and 100")
		}
	case v1dto.CouponTypeFixedAmount:
		if discountValue <= 0 {
			return utils.NewError(utils.ErrCodeBadRequest, "Fixed amount discount value must be greater than 0")
		}
		if maxDiscountAmount != nil {
			return utils.NewError(utils.ErrCodeBadRequest, "Max discount amount is not applicable for Fixed Amount coupons")
		}
	default:
		return utils.NewError(utils.ErrCodeBadRequest, "Invalid coupon type")
	}
	return nil
}

func validateCouponEligibility(coupon sqlc.Coupon, subtotal float64, userUsageCount int64, now time.Time) error {
	if !coupon.CouponIsActive {
		return utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
	}
	if now.Before(coupon.CouponStartDate) {
		return utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet valid")
	}
	if now.After(coupon.CouponEndDate) {
		return utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
	}
	if coupon.CouponUsageLimit != nil && coupon.CouponUsedCount >= *coupon.CouponUsageLimit {
		return utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
	}

	minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)
	if subtotal < minOrderAmount {
		return utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Subtotal amount must be at least %.2f to apply this coupon", minOrderAmount))
	}

	if userUsageCount >= int64(coupon.CouponUserUsageLimit) {
		return utils.NewError(utils.ErrCodeBadRequest, "You have reached your usage limit for this coupon")
	}

	return nil
}

func calculateDiscount(coupon sqlc.Coupon, orderAmount float64) float64 {
	var discount float64

	switch coupon.CouponType {
	case v1dto.CouponTypePercentage:
		discount = utils.RoundFloat(orderAmount*utils.NumericToFloat(coupon.CouponDiscountValue)/100, 2)
		if coupon.CouponMaxDiscountAmount.Valid {
			maxDiscount := utils.NumericToFloat(coupon.CouponMaxDiscountAmount)
			if discount > maxDiscount {
				discount = maxDiscount
			}
		}
	case v1dto.CouponTypeFixedAmount:
		discount = utils.NumericToFloat(coupon.CouponDiscountValue)
		if discount > orderAmount {
			discount = orderAmount
		}
	}

	return discount
}

func (cs *couponService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := cs.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}
	return user, nil
}

func (cs *couponService) invalidateCouponCache() {
	if cs.cache != nil {
		if err := cs.cache.Clear("coupons:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
		}
	}
}

// -------------------------------------------------------------------------
// CouponService Implementation
// -------------------------------------------------------------------------

func (cs *couponService) CreateCoupon(ctx context.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error) {
	normalizedCode := strings.ToUpper(strings.TrimSpace(input.Code))

	_, err := cs.repo.GetByCode(ctx, normalizedCode)
	if err == nil {
		return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeConflict, "Coupon code already exists")
	}

	startDate, endDate, err := validateCouponDates(input.StartDate, input.EndDate)
	if err != nil {
		return v1dto.CouponDTO{}, err
	}

	if err := validateDiscountConfig(input.Type, input.DiscountValue, input.MaxDiscountAmount); err != nil {
		return v1dto.CouponDTO{}, err
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	userUsageLimit := int32(1)
	if input.UserUsageLimit >= 1 {
		userUsageLimit = input.UserUsageLimit
	}

	params := sqlc.CreateCouponParams{
		CouponCode:              normalizedCode,
		CouponName:              strings.TrimSpace(input.Name),
		CouponDescription:       utils.StringToPointer(input.Description),
		CouponType:              input.Type,
		CouponDiscountValue:     utils.FloatToNumeric(input.DiscountValue),
		CouponMaxDiscountAmount: utils.FloatPointerToNumeric(input.MaxDiscountAmount),
		CouponMinOrderAmount:    utils.FloatToNumeric(input.MinOrderAmount),
		CouponUsageLimit:        input.UsageLimit,
		CouponUserUsageLimit:    userUsageLimit,
		CouponStartDate:         startDate,
		CouponEndDate:           endDate,
		CouponIsActive:          isActive,
	}

	coupon, err := cs.repo.Create(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeConflict, "Coupon code already exists")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create coupon", err)
	}

	cs.invalidateCouponCache()

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) GetAllCoupons(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error) {
	if sort == "" {
		sort = "desc"
	}
	if orderBy == "" {
		orderBy = "coupon_created_at"
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limitInt := utils.GetIntEnv("DEFAULT_LIMIT", 10)
		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	cacheKey := fmt.Sprintf("coupons:list:p%d:l%d:s%s:t%v:a%v:d%v:ob%s:so%s",
		page, limit, search,
		utils.Int32Value(coupon_type), utils.BoolValue(isActive),
		deleted, orderBy, sort,
	)

	var cacheData struct {
		Coupons []v1dto.CouponDTO `json:"coupons"`
		Total   int32             `json:"total"`
	}

	if cs.cache != nil {
		if err := cs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Coupons != nil {
			return cacheData.Coupons, cacheData.Total, nil
		}
	}

	listCoupons, err := cs.repo.GetAll(ctx, search, orderBy, sort, coupon_type, isActive, limit, offset, deleted)
	if err != nil {
		return []v1dto.CouponDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch coupon list", err)
	}

	total, err := cs.repo.CountCoupons(ctx, search, coupon_type, isActive, deleted)
	if err != nil {
		return []v1dto.CouponDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count coupons", err)
	}

	couponsDTO := v1dto.MapToCouponDTOList(listCoupons)

	if cs.cache != nil {
		cacheData = struct {
			Coupons []v1dto.CouponDTO `json:"coupons"`
			Total   int32             `json:"total"`
		}{
			Coupons: couponsDTO,
			Total:   int32(total),
		}
		if err := cs.cache.Set(cacheKey, cacheData, 5*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to set coupon cache")
		}
	}

	return couponsDTO, int32(total), nil
}

func (cs *couponService) GetCouponByUUID(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	coupon, err := cs.repo.GetByUUID(ctx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
	}

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) UpdateCoupon(ctx context.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error) {
	existing, err := cs.repo.GetByUUID(ctx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
	}

	if existing.CouponType == v1dto.CouponTypeFixedAmount && input.MaxDiscountAmount != nil {
		return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Max discount amount is not applicable for Fixed Amount coupons")
	}

	endDate, err := validateEndDate(input.EndDate, existing.CouponStartDate)
	if err != nil {
		return v1dto.CouponDTO{}, err
	}

	if input.DiscountValue != nil {
		if err := validateDiscountConfig(existing.CouponType, *input.DiscountValue, input.MaxDiscountAmount); err != nil {
			return v1dto.CouponDTO{}, err
		}
	}

	if input.UsageLimit != nil && *input.UsageLimit < existing.CouponUsedCount {
		return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Usage limit cannot be less than the number of times the coupon has been used")
	}

	var couponEndDate pgtype.Timestamptz
	if endDate != nil {
		couponEndDate = pgtype.Timestamptz{Time: *endDate, Valid: true}
	}

	params := sqlc.UpdateCouponParams{
		CouponUuid:              couponUUID,
		CouponName:              input.Name,
		CouponDescription:       input.Description,
		CouponDiscountValue:     utils.FloatPointerToNumeric(input.DiscountValue),
		CouponMaxDiscountAmount: utils.FloatPointerToNumeric(input.MaxDiscountAmount),
		ClearMaxDiscount:        input.ClearMaxDiscountAmount,
		CouponMinOrderAmount:    utils.FloatPointerToNumeric(input.MinOrderAmount),
		CouponUsageLimit:        input.UsageLimit,
		ClearUsageLimit:         input.ClearUsageLimit,
		CouponUserUsageLimit:    input.UserUsageLimit,
		CouponEndDate:           couponEndDate,
		CouponIsActive:          input.IsActive,
	}

	updatedCoupon, err := cs.repo.Update(ctx, params)
	if err != nil {
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update coupon", err)
	}

	cs.invalidateCouponCache()

	return v1dto.MapToCouponDTO(updatedCoupon), nil
}

func (cs *couponService) SoftDeleteCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	coupon, err := cs.repo.SoftDelete(ctx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to soft delete coupon", err)
	}

	cs.invalidateCouponCache()

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) RestoreCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	coupon, err := cs.repo.Restore(ctx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found or not soft deleted for restoration")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to restore coupon", err)
	}

	cs.invalidateCouponCache()

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) TrashCoupon(ctx context.Context, couponUUID uuid.UUID) error {
	_, err := cs.repo.Trash(ctx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Coupon not found or not soft deleted for trashing")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to trash coupon", err)
	}

	cs.invalidateCouponCache()

	return nil
}

func (cs *couponService) PreviewCoupon(ctx context.Context, userUUID uuid.UUID, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error) {
	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.PreviewCouponDTO{}, err
	}

	normalizedCode := strings.ToUpper(strings.TrimSpace(input.Code))

	coupon, err := cs.repo.GetByCode(ctx, normalizedCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.PreviewCouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
	}

	userUsageCount, err := cs.repo.CountUserUsages(ctx, user.UserID, coupon.CouponID)
	if err != nil {
		return v1dto.PreviewCouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to count user coupon usages", err)
	}

	now := time.Now()
	if err := validateCouponEligibility(coupon, input.SubtotalAmount, userUsageCount, now); err != nil {
		return v1dto.PreviewCouponDTO{}, err
	}

	discountAmount := calculateDiscount(coupon, input.SubtotalAmount)
	finalTotalAmount := input.SubtotalAmount - discountAmount
	if finalTotalAmount < 0 {
		finalTotalAmount = 0
	}

	minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)

	return v1dto.PreviewCouponDTO{
		CouponCode:       coupon.CouponCode,
		CouponName:       coupon.CouponName,
		CouponType:       coupon.CouponType,
		DiscountValue:    utils.NumericToFloat(coupon.CouponDiscountValue),
		SubtotalAmount:   input.SubtotalAmount,
		DiscountAmount:   discountAmount,
		FinalTotalAmount: finalTotalAmount,
		MinOrderAmount:   minOrderAmount,
	}, nil
}

func (cs *couponService) LockAndApplyCoupon(ctx context.Context, couponID int32, userID int32, orderID int32) error {
	tx, _ := db.GetTx(ctx)

	// 1. Atomically increment used count (with limit enforcement)
	if err := cs.repo.IncrementUsedCount(ctx, tx, couponID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to increment coupon usage count", err)
	}

	// 2. Record user usage with unique constraint (coupon_id, user_id, order_id) protection
	_, err := cs.repo.RecordUserUsage(ctx, tx, sqlc.RecordCouponUsageParams{
		CouponUsageCouponID: couponID,
		CouponUsageUserID:   userID,
		CouponUsageOrderID:  &orderID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return utils.NewError(utils.ErrCodeConflict, "Coupon has already been used")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to record coupon usage", err)
	}

	return nil
}

