package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	coupondto "goshop/services/coupon/internal/dto"
	couponservice "goshop/services/coupon/internal/service"
)

// OrderCancelledConsumer releases a coupon reservation when an order is
// cancelled. Coupon owns this state transition (branch F); ReleaseCoupon is
// idempotent per order, so redeliveries are safe.
type OrderCancelledConsumer struct {
	couponService couponservice.CouponService
}

func NewOrderCancelledConsumer(couponService couponservice.CouponService) *OrderCancelledConsumer {
	return &OrderCancelledConsumer{couponService: couponService}
}

// Apply releases the coupon reserved for the cancelled order, if any.
func (c *OrderCancelledConsumer) Apply(ctx context.Context, body []byte) error {
	var event struct {
		OrderID     int32   `json:"order_id"`
		OrderUUID   string  `json:"order_uuid"`
		OrderNumber string  `json:"order_number"`
		CouponID    *int32  `json:"coupon_id"`
		CouponUUID  *string `json:"coupon_uuid"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode order.cancelled payload: %w", err)
	}

	couponID := int32(0)
	if event.CouponID != nil {
		couponID = *event.CouponID
	}
	couponUUID := ""
	if event.CouponUUID != nil {
		couponUUID = *event.CouponUUID
	}

	// No coupon on the order: nothing to release.
	if couponID == 0 && couponUUID == "" && event.OrderUUID == "" {
		return nil
	}

	if err := c.couponService.ReleaseCoupon(ctx, coupondto.ReleaseCouponInput{
		CouponID:   couponID,
		CouponUUID: couponUUID,
		OrderID:    event.OrderID,
		OrderUUID:  event.OrderUUID,
	}); err != nil {
		return fmt.Errorf("release coupon for order %s: %w", event.OrderUUID, err)
	}
	return nil
}
