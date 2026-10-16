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
	CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error)
}

type OrderQueryService interface {
	GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error)
	GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
}

type OrderLifecycleService interface {
	UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error)
	CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error)
	CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error
	UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error)
	UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error)
	BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

// ─── Consumer-driven Outbound Ports (Narrow Dependency Ports) ─────────────────

// OrderCustomer represents customer information needed within the order domain.
type OrderCustomer struct {
	UserID       int32     `json:"user_id"`
	UserUUID     uuid.UUID `json:"user_uuid"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	UserLevel    int32     `json:"user_level"`
	Status       int32     `json:"status"`

	// Aliases for seamless migration
	UserUuid     uuid.UUID `json:"-"`
	UserFullname string    `json:"-"`
	UserEmail    string    `json:"-"`
	UserStatus   int32     `json:"-"`
}

func NewOrderCustomer(userID int32, userUUID uuid.UUID, fullName, email string, userLevel, status int32) *OrderCustomer {
	return &OrderCustomer{
		UserID:       userID,
		UserUUID:     userUUID,
		UserUuid:     userUUID,
		FullName:     fullName,
		UserFullname: fullName,
		Email:        email,
		UserEmail:    email,
		UserLevel:    userLevel,
		Status:       status,
		UserStatus:   status,
	}
}

type customerContextKey struct{}

// WithOrderCustomer attaches an authenticated OrderCustomer to the context,
// bypassing redundant repository round-trips when user claims are known.
func WithOrderCustomer(ctx context.Context, customer *OrderCustomer) context.Context {
	return context.WithValue(ctx, customerContextKey{}, customer)
}

// GetOrderCustomerFromContext retrieves an OrderCustomer attached to the context.
func GetOrderCustomerFromContext(ctx context.Context) (*OrderCustomer, bool) {
	if ctx == nil {
		return nil, false
	}
	customer, ok := ctx.Value(customerContextKey{}).(*OrderCustomer)
	return customer, ok && customer != nil
}

// OrderCustomerReader resolves customer information without leaking sqlc.User.
type OrderCustomerReader interface {
	GetByUUID(ctx context.Context, userUUID uuid.UUID) (*OrderCustomer, error)
	GetByID(ctx context.Context, userID int32) (*OrderCustomer, error)
}

// OrderStockPort manages stock reservation and restoration.
// Satisfied by InventoryService.
type OrderStockPort interface {
	LockAndDeductStockWithRef(ctx context.Context, items []DeductItem, refID string, refType string, actorID *int32, note *string) (map[string]InventoryItemStock, error)
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

// OrderOutboxPort writes domain events to outbox within a database transaction.
// Satisfied by repository.OutboxRepository.
type OrderOutboxPort interface {
	Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error)
}

// OrderStorePort is the narrow persistence port for orders.
// Satisfied by repository.OrderRepository.
type OrderStorePort interface {
	Create(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error)
	GetByUUID(ctx context.Context, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, error)
	GetByID(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error)
	GetAll(ctx context.Context, userID *int32, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error)
	CountOrders(ctx context.Context, userID *int32, status *int32, search string) (int64, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error)
	UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error)
	GetByShippingCode(ctx context.Context, shippingCode string) (sqlc.GetOrderByShippingCodeRow, error)
	MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (sqlc.Order, error)
	MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	GetActiveShippedOrders(ctx context.Context, limit int32) ([]sqlc.Order, error)
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
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error)
}

// OrderReturnStorePort is the narrow persistence port for returns.
// Satisfied by repository.OrderReturnsRepository.
type OrderReturnStorePort interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error)
	GetActiveByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetLatestByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetReturns(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error)
	GetByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.OrderReturn, error)
	CountReturns(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error)
	GetReturnsByUserID(ctx context.Context, params sqlc.GetReturnsByUserIDParams) ([]sqlc.GetReturnsByUserIDRow, error)
	CountReturnsByUserID(ctx context.Context, params sqlc.CountReturnsByUserIDParams) (int64, error)
	AdminGetReturnByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error)
	AdminGetLatestReturnByOrderID(ctx context.Context, orderID int32) (sqlc.AdminGetLatestReturnByOrderIDRow, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error)
	UpdateReturnShippingInfo(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error)
	UpdateReturnBankAccount(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error)
}

// ShippingWebhookPort is the narrow persistence port for shipping webhook events.
// Satisfied by repository.WebhookEventRepository.
type ShippingWebhookPort interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error)
	MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error
}

// OrderPaymentRefundPort handles issuing refunds via payment service.
// Satisfied by PaymentService.
type OrderPaymentRefundPort interface {
	RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
}

