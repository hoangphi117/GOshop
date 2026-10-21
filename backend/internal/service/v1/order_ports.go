package v1service

import (
	"context"
	"goshop/internal/utils"
	couponcontract "goshop/services/coupon/contract"
	inventorycontract "goshop/services/inventory/contract"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeductItem struct {
	ProductUUID string
	Quantity    int32
}

type RestoreItem struct {
	ProductID   int32
	ProductUUID string
	Quantity    int32
}

type OrderCartUUIDPort interface {
	RemovePurchasedItemsByUUID(ctx context.Context, userUUID uuid.UUID, productUUIDs []uuid.UUID) error
}

type OrderStockPort interface {
	LockAndDeductStockWithRef(ctx context.Context, items []DeductItem, refID string, refType string, actorID *int32, note *string) error
	RestoreStockWithRef(ctx context.Context, items []RestoreItem, refID string, refType string, actorID *int32, note *string) error
}

type orderStockGRPCAdapter struct {
	client inventorycontract.InventoryClient
}

func NewOrderStockGRPCAdapter(client inventorycontract.InventoryClient) OrderStockPort {
	return &orderStockGRPCAdapter{client: client}
}

func (a *orderStockGRPCAdapter) LockAndDeductStockWithRef(
	ctx context.Context,
	items []DeductItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	stockItems := make([]inventorycontract.StockItem, len(items))
	for i, it := range items {
		stockItems[i] = inventorycontract.StockItem{
			ProductUUID: it.ProductUUID,
			Quantity:    it.Quantity,
		}
	}
	_, err := a.client.ReserveStock(ctx, inventorycontract.ReserveStockInput{
		Items:         stockItems,
		ReferenceID:   refID,
		ReferenceType: refType,
		ActorID:       actorID,
		Note:          note,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.ResourceExhausted:
				return utils.NewError(utils.ErrCodeConflict, "insufficient stock for requested items")
			case codes.FailedPrecondition:
				return utils.NewError(utils.ErrCodeConflict, st.Message())
			case codes.InvalidArgument:
				return utils.NewError(utils.ErrCodeBadRequest, st.Message())
			}
		}
		return err
	}
	return nil
}

func (a *orderStockGRPCAdapter) RestoreStockWithRef(
	ctx context.Context,
	items []RestoreItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	stockItems := make([]inventorycontract.StockItem, len(items))
	for i, it := range items {
		stockItems[i] = inventorycontract.StockItem{
			ProductID:   it.ProductID,
			ProductUUID: it.ProductUUID,
			Quantity:    it.Quantity,
		}
	}
	return a.client.ReleaseStock(ctx, inventorycontract.ReleaseStockInput{
		Items:         stockItems,
		ReferenceID:   refID,
		ReferenceType: refType,
		ActorID:       actorID,
		Note:          note,
	})
}

type CouponDiscountResult struct {
	CouponID         int32   `json:"coupon_id"`
	CouponCode       string  `json:"coupon_code"`
	DiscountAmount   float64 `json:"discount_amount"`
	FinalTotalAmount float64 `json:"final_total_amount"`
}

type OrderCouponPort interface {
	PreviewCoupon(ctx context.Context, code string, amount float64, userUUID uuid.UUID) (*CouponDiscountResult, error)
	LockAndApplyCoupon(ctx context.Context, couponID int32, userID int32, orderID int32, orderUUID *uuid.UUID) error
	ReleaseCoupon(ctx context.Context, couponID int32, orderID int32, orderUUID *uuid.UUID) error
}

type orderCouponGRPCAdapter struct {
	client couponcontract.CouponClient
}

func NewOrderCouponGRPCAdapter(client couponcontract.CouponClient) OrderCouponPort {
	return &orderCouponGRPCAdapter{client: client}
}

func (a *orderCouponGRPCAdapter) PreviewCoupon(
	ctx context.Context,
	code string,
	amount float64,
	userUUID uuid.UUID,
) (*CouponDiscountResult, error) {
	var uStr string
	if userUUID != uuid.Nil {
		uStr = userUUID.String()
	}
	preview, err := a.client.PreviewCoupon(ctx, code, amount, uStr)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument, codes.NotFound:
				return nil, utils.NewError(utils.ErrCodeBadRequest, st.Message())
			}
		}
		return nil, err
	}
	return &CouponDiscountResult{
		CouponID:         preview.CouponID,
		CouponCode:       preview.CouponCode,
		DiscountAmount:   preview.DiscountAmount,
		FinalTotalAmount: preview.FinalTotalAmount,
	}, nil
}

func (a *orderCouponGRPCAdapter) LockAndApplyCoupon(
	ctx context.Context,
	couponID int32,
	userID int32,
	orderID int32,
	orderUUID *uuid.UUID,
) error {
	var orderUUIDStr string
	if orderUUID != nil && *orderUUID != uuid.Nil {
		orderUUIDStr = orderUUID.String()
	}
	_, err := a.client.ReserveCoupon(ctx, couponcontract.ReserveCouponInput{
		CouponID:  couponID,
		UserID:    userID,
		OrderID:   orderID,
		OrderUUID: orderUUIDStr,
	})
	return err
}

func (a *orderCouponGRPCAdapter) ReleaseCoupon(
	ctx context.Context,
	couponID int32,
	orderID int32,
	orderUUID *uuid.UUID,
) error {
	var orderUUIDStr string
	if orderUUID != nil && *orderUUID != uuid.Nil {
		orderUUIDStr = orderUUID.String()
	}
	return a.client.ReleaseCoupon(ctx, couponcontract.ReleaseCouponInput{
		CouponID:  couponID,
		OrderID:   orderID,
		OrderUUID: orderUUIDStr,
	})
}
