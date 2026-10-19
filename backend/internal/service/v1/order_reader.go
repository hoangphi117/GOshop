package v1service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OrderSummary contains minimal order info needed for cross-domain reads (e.g. Chat, Review).
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

// PurchaseVerification contains the verification result for a user reviewing a product in an order.
type PurchaseVerification struct {
	OrderID       int32
	OrderUUID     uuid.UUID
	UserID        int32
	IsPurchased   bool
	CanReview     bool
	DeliveredAt   *time.Time
	CustomerName  string
	CustomerEmail string
	ShippingName  string
}

// OrderReader is the inbound read-only port for other domains (Chat, Review) to query orders without accessing OrderRepository directly.
type OrderReader interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error)
	VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID int32, productID int32) (*PurchaseVerification, error)
}

type inprocessOrderReader struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
}

// NewInprocessOrderReader creates an in-process OrderReader adapter wrapping OrderRepository and OrderItemRepository.
func NewInprocessOrderReader(orderRepo repository.OrderRepository, orderItemRepo ...repository.OrderItemRepository) OrderReader {
	var itemRepo repository.OrderItemRepository
	if len(orderItemRepo) > 0 {
		itemRepo = orderItemRepo[0]
	}
	return &inprocessOrderReader{
		orderRepo:     orderRepo,
		orderItemRepo: itemRepo,
	}
}

func (r *inprocessOrderReader) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error) {
	if r.orderRepo == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "order repository not configured")
	}

	order, err := r.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "order not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	var deliveredAt *time.Time
	if order.OrderDeliveredAt.Valid {
		t := order.OrderDeliveredAt.Time
		deliveredAt = &t
	}

	return &OrderSummary{
		OrderID:       order.OrderID,
		OrderUUID:     order.OrderUuid,
		OrderNumber:   order.OrderNumber,
		UserID:        order.OrderUserID,
		UserUUID:      order.UserUuid,
		CustomerName:  order.OrderCustomerName,
		CustomerEmail: order.OrderCustomerEmail,
		Status:        order.OrderStatus,
		TotalAmount:   utils.NumericToFloat(order.OrderTotalAmount),
		DeliveredAt:   deliveredAt,
		UpdatedAt:     order.OrderUpdatedAt,
	}, nil
}

func (r *inprocessOrderReader) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID int32, productID int32) (*PurchaseVerification, error) {
	if r.orderRepo == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "order repository not configured")
	}

	order, err := r.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "order not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.OrderUserID != userID {
		return nil, utils.NewError(utils.ErrCodeForbidden, "user is not authorized for this order")
	}

	const maxReviewWindow = 30 * 24 * time.Hour
	canReview := false
	var deliveredAt *time.Time

	if order.OrderDeliveredAt.Valid {
		t := order.OrderDeliveredAt.Time
		deliveredAt = &t
		if time.Since(t) <= maxReviewWindow {
			canReview = true
		}
	} else if order.OrderStatus == 4 && !order.OrderUpdatedAt.IsZero() && time.Since(order.OrderUpdatedAt) <= maxReviewWindow {
		canReview = true
	}

	isPurchased := false
	if r.orderItemRepo != nil {
		items, err := r.orderItemRepo.GetByOrderID(ctx, order.OrderID)
		if err == nil {
			for _, it := range items {
				if it.OrderItemProductID == productID {
					isPurchased = true
					break
				}
			}
		}
	} else {
		// When item repo is not provided, default purchased to true if order exists and belongs to user
		isPurchased = true
	}

	return &PurchaseVerification{
		OrderID:       order.OrderID,
		OrderUUID:     order.OrderUuid,
		UserID:        order.OrderUserID,
		IsPurchased:   isPurchased,
		CanReview:     canReview,
		DeliveredAt:   deliveredAt,
		CustomerName:  order.OrderCustomerName,
		CustomerEmail: order.OrderCustomerEmail,
		ShippingName:  order.OrderShippingName,
	}, nil
}
