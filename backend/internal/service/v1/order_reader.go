package v1service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Standard order status constants used across domain boundaries.
const (
	OrderStatusPending    int32 = 1
	OrderStatusConfirmed  int32 = 2
	OrderStatusProcessing int32 = 3
	OrderStatusShipped    int32 = 4
	OrderStatusDelivered  int32 = 5
	OrderStatusCancelled  int32 = 6

	OrderReturnStatusPending   int16 = 1
	OrderReturnStatusApproved  int16 = 2
	OrderReturnStatusRejected  int16 = 3
	OrderReturnStatusCompleted int16 = 4
	OrderReturnStatusCancelled int16 = 5
)

// OrderSummary contains minimal order info needed for cross-domain reads (e.g. Chat, Reporting).
type OrderSummary struct {
	OrderID       int32
	OrderUUID     uuid.UUID
	OrderNumber   string
	UserID        int32
	UserUUID      uuid.UUID
	CustomerName  string
	CustomerEmail string
	Status        int32
	TotalAmount   float64
	DeliveredAt   *time.Time
	UpdatedAt     time.Time
}

// OrderReader is the inbound read-only port for other domains (Chat) to query orders via gRPC or client adapter.
type OrderReader interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error)
}
