package v1service

import (
	"database/sql"
	"errors"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type couponService struct {
	repo  repository.CouponRepository
	cache cache.RedisCacheService
}

func NewCouponService(repo repository.CouponRepository, redisClient *redis.Client) CouponService {
	return &couponService{
		repo:  repo,
		cache: cache.NewRedisCacheService(redisClient),
	}
}

func validateCouponDates(startDateStr, endDateStr string) (time.Time, time.Time, error) {
	// (RFC3339 / ISO8601: "2026-08-28T10:00:00Z")
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

func (cs *couponService) CreateCoupon(ctx *gin.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	normalizedCode := strings.ToUpper(strings.TrimSpace(input.Code))

	_, err := cs.repo.GetByCode(reqCtx, normalizedCode)
	if err == nil {
		return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeConflict, "Coupon code already exists")
	}

	startDate, endDate, err := validateCouponDates(input.StartDate, input.EndDate)
	if err != nil {
		return v1dto.CouponDTO{}, err
	}

	switch input.Type {
	case v1dto.CouponTypePercentage:
		if input.DiscountValue < 1 || input.DiscountValue > 100 {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Percentage discount value must be between 1 and 100")
		}
	case v1dto.CouponTypeFixedAmount:
		if input.DiscountValue <= 0 {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Fixed amount discount value must be greater than 0")
		}
		if input.MaxDiscountAmount != nil {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Max discount amount is not applicable for Fixed Amount coupons")
		}
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

	coupon, err := cs.repo.Create(reqCtx, params)
	if err != nil {
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create coupon: ", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) UpdateCoupon(ctx *gin.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	existing, err := cs.repo.GetByUUID(reqCtx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
	}

	// Cross validation: MaxDiscountAmount is only valid for Percentage coupons
	if existing.CouponType == v1dto.CouponTypeFixedAmount && input.MaxDiscountAmount != nil {
		return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Max discount amount is not applicable for Fixed Amount coupons")
	}

	var endDate *time.Time
	if input.EndDate != nil {
		parsedEnd, err := time.Parse(time.RFC3339, *input.EndDate)
		if err != nil {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid end date format")
		}
		if !parsedEnd.After(existing.CouponStartDate) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "End date must be after start date")
		}
		endDate = &parsedEnd
	}

	if input.DiscountValue != nil {
		switch existing.CouponType {
		case v1dto.CouponTypePercentage:
			if *input.DiscountValue < 1 || *input.DiscountValue > 100 {
				return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Percentage discount value must be between 1 and 100")
			}
		case v1dto.CouponTypeFixedAmount:
			if *input.DiscountValue <= 0 {
				return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Fixed amount discount value must be greater than 0")
			}
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

	updatedCoupon, err := cs.repo.Update(reqCtx, params)
	if err != nil {
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update coupon ", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(updatedCoupon), nil
}

func (cs *couponService) SoftDeleteCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	coupon, err := cs.repo.SoftDelete(reqCtx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to soft delete coupon", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(coupon), nil
}
