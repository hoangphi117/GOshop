package service

import (
	"context"
	"fmt"
	"time"

	"goshop/pkg/utils"
	"goshop/services/payment/internal/dto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderReaderDBClient struct {
	pool *pgxpool.Pool
}

func NewOrderReaderDBClient(pool *pgxpool.Pool) OrderReaderClient {
	return &OrderReaderDBClient{pool: pool}
}

func (c *OrderReaderDBClient) GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*dto.PaymentOrderSummary, error) {
	if c.pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT order_id, order_uuid, order_number, order_user_id, order_user_uuid,
		       order_total_amount, order_payment_status, order_status, order_payment_method,
		       order_refunded_amount, order_created_at
		FROM orders
		WHERE order_uuid = $1
		LIMIT 1;
	`

	var (
		orderID             int32
		resUUID             uuid.UUID
		orderNumber         string
		userID              int32
		userUUID            uuid.UUID
		totalAmount         pgtype.Numeric
		paymentStatus       int32
		orderStatus         int32
		paymentMethod       int32
		orderRefundedAmount pgtype.Numeric
		createdAt           time.Time
	)

	row := c.pool.QueryRow(ctx, query, orderUUID)
	if err := row.Scan(
		&orderID, &resUUID, &orderNumber, &userID, &userUUID,
		&totalAmount, &paymentStatus, &orderStatus, &paymentMethod,
		&orderRefundedAmount, &createdAt,
	); err != nil {
		return nil, fmt.Errorf("failed to query order by uuid %s: %w", orderUUID, err)
	}

	return &dto.PaymentOrderSummary{
		OrderID:             orderID,
		OrderUUID:           resUUID,
		OrderNumber:         orderNumber,
		UserID:              userID,
		UserUUID:            userUUID,
		TotalAmount:         utils.NumericToFloat(totalAmount),
		PaymentStatus:       paymentStatus,
		OrderStatus:         orderStatus,
		PaymentMethod:       paymentMethod,
		OrderRefundedAmount: utils.NumericToFloat(orderRefundedAmount),
		CreatedAt:           createdAt,
	}, nil
}

func (c *OrderReaderDBClient) GetByOrderNumber(ctx context.Context, orderNumber string) (*dto.PaymentOrderSummary, error) {
	if c.pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT order_id, order_uuid, order_number, order_user_id, order_user_uuid,
		       order_total_amount, order_payment_status, order_status, order_payment_method,
		       order_refunded_amount, order_created_at
		FROM orders
		WHERE order_number = $1
		LIMIT 1;
	`

	var (
		orderID             int32
		resUUID             uuid.UUID
		resNumber           string
		userID              int32
		userUUID            uuid.UUID
		totalAmount         pgtype.Numeric
		paymentStatus       int32
		orderStatus         int32
		paymentMethod       int32
		orderRefundedAmount pgtype.Numeric
		createdAt           time.Time
	)

	row := c.pool.QueryRow(ctx, query, orderNumber)
	if err := row.Scan(
		&orderID, &resUUID, &resNumber, &userID, &userUUID,
		&totalAmount, &paymentStatus, &orderStatus, &paymentMethod,
		&orderRefundedAmount, &createdAt,
	); err != nil {
		return nil, fmt.Errorf("failed to query order by number %s: %w", orderNumber, err)
	}

	return &dto.PaymentOrderSummary{
		OrderID:             orderID,
		OrderUUID:           resUUID,
		OrderNumber:         resNumber,
		UserID:              userID,
		UserUUID:            userUUID,
		TotalAmount:         utils.NumericToFloat(totalAmount),
		PaymentStatus:       paymentStatus,
		OrderStatus:         orderStatus,
		PaymentMethod:       paymentMethod,
		OrderRefundedAmount: utils.NumericToFloat(orderRefundedAmount),
		CreatedAt:           createdAt,
	}, nil
}
