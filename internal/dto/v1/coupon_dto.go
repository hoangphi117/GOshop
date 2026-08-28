package v1dto

import (
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"time"
)

const (
	CouponTypePercentage  int32 = 1
	CouponTypeFixedAmount int32 = 2
)

type CouponDTO struct {
	UUID              string   `json:"uuid"`
	Code              string   `json:"code"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Type              int32    `json:"type"`
	TypeLabel         string   `json:"type_label"`
	DiscountValue     float64  `json:"discount_value"`
	MaxDiscountAmount *float64 `json:"max_discount_amount,omitempty"`
	MinOrderAmount    float64  `json:"min_order_amount"`
	UsageLimit        *int32   `json:"usage_limit,omitempty"`
	UsedCount         int32    `json:"used_count"`
	UserUsageLimit    int32    `json:"user_usage_limit"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
	IsActive          bool     `json:"is_active"`
	IsExpired         bool     `json:"is_expired"`
}

type CreateCouponInput struct {
	Code              string   `json:"code" binding:"required,min=2,max=50,alphanum"`
	Name              string   `json:"name" binding:"required,min=2,max=100"`
	Description       string   `json:"description" binding:"omitempty,max=500"`
	Type              int32    `json:"type" binding:"required,oneof=1 2"`
	DiscountValue     float64  `json:"discount_value" binding:"required,gt=0"`
	MaxDiscountAmount *float64 `json:"max_discount_amount" binding:"omitempty,gt=0"`
	MinOrderAmount    float64  `json:"min_order_amount" binding:"omitempty,gte=0"`
	UsageLimit        *int32   `json:"usage_limit" binding:"omitempty,gte=1"`
	UserUsageLimit    int32    `json:"user_usage_limit" binding:"omitempty,gte=1"`
	StartDate         string   `json:"start_date" binding:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndDate           string   `json:"end_date" binding:"required,datetime=2006-01-02T15:04:05Z07:00"`
	IsActive          *bool    `json:"is_active" binding:"omitempty"`
}

func MapToCouponDTO(coupon sqlc.Coupon) CouponDTO {
	typeLabel := "Unknown"
	switch coupon.CouponType {
	case CouponTypePercentage:
		typeLabel = "Percentage"
	case CouponTypeFixedAmount:
		typeLabel = "Fixed Amount"
	}

	// Pointer null-safe
	var maxDiscountAmount *float64
	if coupon.CouponMaxDiscountAmount.Valid {
		val := utils.NumericToFloat(coupon.CouponMaxDiscountAmount)
		maxDiscountAmount = &val
	}

	var description string
	if coupon.CouponDescription != nil {
		description = *coupon.CouponDescription
	}

	now := time.Now()
	isExpired := now.After(coupon.CouponEndDate)

	return CouponDTO{
		UUID:              coupon.CouponUuid.String(),
		Code:              coupon.CouponCode,
		Name:              coupon.CouponName,
		Description:       description,
		Type:              coupon.CouponType,
		TypeLabel:         typeLabel,
		DiscountValue:     utils.NumericToFloat(coupon.CouponDiscountValue),
		MaxDiscountAmount: maxDiscountAmount,
		MinOrderAmount:    utils.NumericToFloat(coupon.CouponMinOrderAmount),
		UsageLimit:        coupon.CouponUsageLimit,
		UsedCount:         coupon.CouponUsedCount,
		UserUsageLimit:    coupon.CouponUserUsageLimit,
		StartDate:         coupon.CouponStartDate.Format(time.RFC3339),
		EndDate:           coupon.CouponEndDate.Format(time.RFC3339),
		IsActive:          coupon.CouponIsActive,
		IsExpired:         isExpired,
	}
}

func MapToCouponDTOList(coupons []sqlc.Coupon) []CouponDTO {
	dtos := make([]CouponDTO, 0, len(coupons))
	for _, coupon := range coupons {
		dtos = append(dtos, MapToCouponDTO(coupon))
	}
	return dtos
}
