package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	return &couponService{
		repo:     repo,
		userRepo: userRepo,
		cache:    cache.NewRedisCacheService(redisClient),
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeConflict, "Coupon code already exists")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create coupon: ", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) GetAllCoupons(ctx *gin.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error) {
	reqCtx := ctx.Request.Context()

	if sort == "" {
		sort = "desc"
	}

	if orderBy == "" {
		orderBy = "user_created_at"
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

	if err := cs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Coupons != nil {
		return cacheData.Coupons, cacheData.Total, nil
	}

	listCoupons, err := cs.repo.GetAll(reqCtx, search, orderBy, sort, coupon_type, isActive, limit, offset, deleted)
	if err != nil {
		return []v1dto.CouponDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch coupon list", err)
	}

	total, err := cs.repo.CountCoupons(reqCtx, search, coupon_type, isActive, deleted)
	if err != nil {
		return []v1dto.CouponDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count coupons", err)
	}

	cacheData = struct {
		Coupons []v1dto.CouponDTO `json:"coupons"`
		Total   int32             `json:"total"`
	}{
		Coupons: v1dto.MapToCouponDTOList(listCoupons),
		Total:   int32(total),
	}

	if err := cs.cache.Set(cacheKey, cacheData, 5*time.Minute); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to set coupon cache")
	}

	return v1dto.MapToCouponDTOList(listCoupons), int32(total), nil
}

func (cs *couponService) GetCouponByUUID(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	coupon, err := cs.repo.GetByUUID(reqCtx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
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

func (cs *couponService) RestoreCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	coupon, err := cs.repo.Restore(reqCtx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found or not soft deleted for restoration")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to restore coupon", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) TrashCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error) {
	reqCtx := ctx.Request.Context()

	coupon, err := cs.repo.Trash(reqCtx, couponUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found or not soft deleted for trashing")
		}
		return v1dto.CouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to trash coupon", err)
	}

	if err := cs.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}

	return v1dto.MapToCouponDTO(coupon), nil
}

func (cs *couponService) CountUserCouponUsages(ctx context.Context, userID, couponID int32) (int64, error) {
	count, err := cs.repo.CountUserUsages(ctx, userID, couponID)
	if err != nil {
		return 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count user coupon usages", err)
	}
	return count, nil
}

func (cs *couponService) IncrementCouponUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	err := cs.repo.IncrementUsedCount(ctx, tx, couponID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeConflict, "Coupon usage limit reached, please try another coupon")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to increment coupon used count", err)
	}
	return nil
}

func (cs *couponService) RecordUserCouponUsage(ctx context.Context, tx pgx.Tx, couponID, userID, orderID int32) (sqlc.CouponUsage, error) {
	params := sqlc.RecordCouponUsageParams{
		CouponUsageCouponID: couponID,
		CouponUsageUserID:   userID,
		CouponUsageOrderID:  utils.ConvertToInt32Pointer(orderID),
	}

	couponUsage, err := cs.repo.RecordUserUsage(ctx, tx, params)
	if err != nil {
		return sqlc.CouponUsage{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record user coupon usage", err)
	}

	return couponUsage, nil
}

func (cs *couponService) PreviewCoupon(ctx *gin.Context, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := GetUserFromContext(ctx, cs.userRepo)
	if err != nil {
		return v1dto.PreviewCouponDTO{}, err
	}

	normalizedCode := strings.ToUpper(strings.TrimSpace(input.Code))

	coupon, err := cs.repo.GetByCode(reqCtx, normalizedCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		}
		return v1dto.PreviewCouponDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get coupon", err)
	}

	if !coupon.CouponIsActive {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
	}

	now := time.Now()
	if now.Before(coupon.CouponStartDate) {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet valid")
	}
	if now.After(coupon.CouponEndDate) {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
	}

	if coupon.CouponUsageLimit != nil && coupon.CouponUsedCount >= *coupon.CouponUsageLimit {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
	}

	minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)
	if input.SubtotalAmount < minOrderAmount {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Subtotal amount must be at least %.2f to apply this coupon", minOrderAmount))
	}

	userUsageCount, err := cs.CountUserCouponUsages(reqCtx, user.UserID, coupon.CouponID)
	if err != nil {
		return v1dto.PreviewCouponDTO{}, err
	}

	if userUsageCount >= int64(coupon.CouponUserUsageLimit) {
		return v1dto.PreviewCouponDTO{}, utils.NewError(utils.ErrCodeBadRequest, "You have reached your usage limit for this coupon")
	}

	discountAmount := CalculateDiscount(coupon, input.SubtotalAmount)
	finalTotalAmount := input.SubtotalAmount - discountAmount

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
