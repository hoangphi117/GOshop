package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrderRepository interface {
	Create(ctx context.Context, params ordersqlc.CreateOrderParams) (ordersqlc.Order, error)
	GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (ordersqlc.GetOrderByUUIDRow, error)
	GetByID(ctx context.Context, orderID int32) (ordersqlc.GetOrderByIDRow, error)
	GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (ordersqlc.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (ordersqlc.GetOrderByOrderNumberRow, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateOrderStatusParams) (ordersqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params ordersqlc.UpdateOrderShippingInfoParams) (ordersqlc.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateOrderPaymentStatusParams) (ordersqlc.Order, error)
	CountOrders(ctx context.Context, userID, status *int32, search string) (int64, error)
	GetAll(ctx context.Context, userID, status *int32, search, orderBy, sort string, limit, offset int32) ([]ordersqlc.Order, error)
	UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params ordersqlc.UpdateOrderShippingAndStatusParams) (ordersqlc.Order, error)
	GetByShippingCode(ctx context.Context, shippingCode string) (ordersqlc.GetOrderByShippingCodeRow, error)
	MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (ordersqlc.Order, error)
	MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error)
	CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error)
	GetActiveShippedOrders(ctx context.Context, limit int32) ([]ordersqlc.Order, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, tx pgx.Tx, input ordersqlc.CreateOrderItemParams) (ordersqlc.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]ordersqlc.GetOrderItemsByOrderIDRow, error)
}

type TrackingRepository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params ordersqlc.InsertTrackingEventParams) (ordersqlc.OrderTrackingEvent, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]ordersqlc.OrderTrackingEvent, error)
}

type OrderReturnsRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params ordersqlc.CreateOrderReturnParams) (ordersqlc.OrderReturn, error)
	GetActiveByOrderID(ctx context.Context, orderID int32) (ordersqlc.OrderReturn, error)
	GetLatestByOrderID(ctx context.Context, orderID int32) (ordersqlc.OrderReturn, error)
	GetReturns(ctx context.Context, params ordersqlc.AdminGetReturnsParams) ([]ordersqlc.AdminGetReturnsRow, error)
	GetByUUID(ctx context.Context, returnUUID uuid.UUID) (ordersqlc.OrderReturn, error)
	CountReturns(ctx context.Context, params ordersqlc.AdminCountReturnsParams) (int64, error)
	GetReturnsByUserID(ctx context.Context, params ordersqlc.GetReturnsByUserIDParams) ([]ordersqlc.GetReturnsByUserIDRow, error)
	CountReturnsByUserID(ctx context.Context, params ordersqlc.CountReturnsByUserIDParams) (int64, error)
	AdminGetReturnByUUID(ctx context.Context, returnUUID uuid.UUID) (ordersqlc.AdminGetReturnByUUIDRow, error)
	AdminGetLatestReturnByOrderID(ctx context.Context, orderID int32) (ordersqlc.AdminGetLatestReturnByOrderIDRow, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnStatusParams) (ordersqlc.OrderReturn, error)
	UpdateReturnShippingInfo(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnShippingInfoParams) (ordersqlc.OrderReturn, error)
	UpdateReturnBankAccount(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnBankAccountParams) (ordersqlc.OrderReturn, error)
}

type SagaRepository interface {
	Start(ctx context.Context, tx pgx.Tx, params ordersqlc.StartSagaParams) (ordersqlc.OrderSaga, error)
	Confirm(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error)
	ClaimForCompensation(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	MarkFailed(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	ScheduleRetry(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	ClaimStale(ctx context.Context, tx pgx.Tx, limit int32) ([]ordersqlc.OrderSaga, error)
	UpdateStep(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error)
	GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) (ordersqlc.OrderSaga, error)
	GetByIdempotencyKey(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (ordersqlc.OrderSaga, error)
}

type WebhookEventRepository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, input ordersqlc.InsertWebhookEventParams) (ordersqlc.WebhookEvent, error)
	MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error
}

type OutboxRepository interface {
	Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (ordersqlc.OutboxEvent, error)
	Create(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload []byte) (ordersqlc.OutboxEvent, error)
	GetPending(ctx context.Context, limit int32) ([]ordersqlc.OutboxEvent, error)
	GetPendingByAggregateTypes(ctx context.Context, limit int32, aggregateTypes []string) ([]ordersqlc.OutboxEvent, error)
	MarkProcessed(ctx context.Context, eventIDs []int64) error
	MarkFailed(ctx context.Context, eventID int64, lastErr string) error
}
