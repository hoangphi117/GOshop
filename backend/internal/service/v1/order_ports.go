package v1service

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ─── Consumer-driven Inbound Ports (Use-Case Interfaces) ──────────────────────

type OrderCheckoutService interface {
	CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
}

type OrderQueryService interface {
	GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
}

type OrderLifecycleService interface {
	UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

// ─── Consumer-driven Outbound Ports (Narrow Dependency Ports) ─────────────────

// OrderCustomerReader resolves user profile information.
// Satisfied by repository.UserRepository.
type OrderCustomerReader interface {
	GetByUUID(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error)
}

// OrderStockPort manages stock reservation and restoration.
// Satisfied by InventoryService.
type OrderStockPort interface {
	LockAndDeductStockWithRef(ctx context.Context, items []DeductItem, refID string, refType string, actorID *int32, note *string) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStockWithRef(ctx context.Context, items []RestoreItem, refID string, refType string, actorID *int32, note *string) error
}

// OrderCouponPort locks and registers coupon usage upon checkout.
// Satisfied by CouponService.
type OrderCouponPort interface {
	LockAndApplyCoupon(ctx context.Context, couponID int32, userID int32, orderID int32) error
}

// OrderCartPort clears purchased items from cart.
// Satisfied by CartService.
type OrderCartPort interface {
	RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error
}

// OrderStorePort is the narrow persistence port for orders.
// Satisfied by repository.OrderRepository.
type OrderStorePort interface {
	Create(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error)
	GetByUUID(ctx context.Context, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, error)
	GetAll(ctx context.Context, userID *int32, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error)
	CountOrders(ctx context.Context, userID *int32, status *int32, search string) (int64, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error)
}

// OrderItemStorePort is the narrow persistence port for order line items.
// Satisfied by repository.OrderItemRepository.
type OrderItemStorePort interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error)
}

// OrderTrackingPort records timeline lifecycle events.
// Satisfied by repository.TrackingRepository.
type OrderTrackingPort interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error)
}
