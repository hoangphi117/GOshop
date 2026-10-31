package dto

import "time"

const (
	CouponTypePercentage  int32 = 1
	CouponTypeFixedAmount int32 = 2
)

// Coupon usage reservation states (coupon_usages.coupon_usage_status).
const (
	CouponUsageStatusReserved int32 = 1
	CouponUsageStatusReleased int32 = 2
)

type CouponDTO struct {
	ID                int32    `json:"id"`
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

type PreviewCouponInput struct {
	Code           string  `json:"code"`
	SubtotalAmount float64 `json:"subtotal_amount"`
	UserUUID       string  `json:"user_uuid,omitempty"`
}

type PreviewCouponDTO struct {
	CouponID         int32   `json:"coupon_id"`
	CouponUUID       string  `json:"coupon_uuid"`
	CouponCode       string  `json:"coupon_code"`
	CouponName       string  `json:"coupon_name"`
	CouponType       int32   `json:"coupon_type"`
	DiscountValue    float64 `json:"discount_value"`
	SubtotalAmount   float64 `json:"subtotal_amount"`
	DiscountAmount   float64 `json:"discount_amount"`
	FinalTotalAmount float64 `json:"final_total_amount"`
	MinOrderAmount   float64 `json:"min_order_amount"`
}

type ReserveCouponInput struct {
	CouponID   int32  `json:"coupon_id"`
	CouponUUID string `json:"coupon_uuid"`
	UserID     int32  `json:"user_id"`
	UserUUID   string `json:"user_uuid"`
	OrderID    int32  `json:"order_id"`
	OrderUUID  string `json:"order_uuid"`
}

type ReleaseCouponInput struct {
	CouponID   int32  `json:"coupon_id"`
	CouponUUID string `json:"coupon_uuid"`
	OrderID    int32  `json:"order_id"`
	OrderUUID  string `json:"order_uuid"`
}

type CreateCouponInput struct {
	Code              string    `json:"code" binding:"required,min=2,max=50,alphanum"`
	Name              string    `json:"name" binding:"required,min=2,max=100"`
	Description       string    `json:"description" binding:"omitempty,max=500"`
	Type              int32     `json:"type" binding:"required,oneof=1 2"`
	DiscountValue     float64   `json:"discount_value" binding:"required,gt=0"`
	MaxDiscountAmount *float64  `json:"max_discount_amount" binding:"omitempty,gt=0"`
	MinOrderAmount    float64   `json:"min_order_amount" binding:"omitempty,gte=0"`
	UsageLimit        *int32    `json:"usage_limit" binding:"omitempty,gte=1"`
	UserUsageLimit    int32     `json:"user_usage_limit" binding:"omitempty,gte=1"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	IsActive          bool      `json:"is_active"`
}

type UpdateCouponInput struct {
	Name              *string    `json:"name"`
	Description       *string    `json:"description"`
	Type              *int32     `json:"type"`
	DiscountValue     *float64   `json:"discount_value"`
	MaxDiscountAmount *float64   `json:"max_discount_amount"`
	MinOrderAmount    *float64   `json:"min_order_amount"`
	UsageLimit        *int32     `json:"usage_limit"`
	UserUsageLimit    *int32     `json:"user_usage_limit"`
	StartDate         *time.Time `json:"start_date"`
	EndDate           *time.Time `json:"end_date"`
	IsActive          *bool      `json:"is_active"`
}
