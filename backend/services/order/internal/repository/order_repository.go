package repository

import (
	"context"
	"fmt"
	"strings"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SqlOrderRepository struct {
	db   ordersqlc.Querier
	pool *pgxpool.Pool
}

func NewSqlOrderRepository(db ordersqlc.Querier, pool ...*pgxpool.Pool) OrderRepository {
	var p *pgxpool.Pool
	if len(pool) > 0 {
		p = pool[0]
	}
	return &SqlOrderRepository{
		db:   db,
		pool: p,
	}
}

func (r *SqlOrderRepository) getQuerier(tx pgx.Tx) ordersqlc.Querier {
	if tx != nil {
		return ordersqlc.New(tx)
	}
	return r.db
}

func (r *SqlOrderRepository) Create(ctx context.Context, params ordersqlc.CreateOrderParams) (ordersqlc.Order, error) {
	return r.db.CreateOrder(ctx, params)
}

func (r *SqlOrderRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (ordersqlc.GetOrderByUUIDRow, error) {
	return r.db.GetOrderByUUID(ctx, uuid)
}

func (r *SqlOrderRepository) GetByID(ctx context.Context, orderID int32) (ordersqlc.GetOrderByIDRow, error) {
	return r.db.GetOrderByID(ctx, orderID)
}

func (r *SqlOrderRepository) GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (ordersqlc.Order, error) {
	return r.getQuerier(tx).GetOrderByUUIDForUpdate(ctx, uuid)
}

func (r *SqlOrderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (ordersqlc.GetOrderByOrderNumberRow, error) {
	return r.db.GetOrderByOrderNumber(ctx, orderNumber)
}

func (r *SqlOrderRepository) GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error) {
	return r.db.GetLatestOrderNumber(ctx, orderNumber)
}

func (r *SqlOrderRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateOrderStatusParams) (ordersqlc.Order, error) {
	return r.getQuerier(tx).UpdateOrderStatus(ctx, params)
}

func (r *SqlOrderRepository) CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error) {
	return r.getQuerier(tx).CancelOrder(ctx, orderUUID)
}

func (r *SqlOrderRepository) UpdateOrderShippingInfo(ctx context.Context, params ordersqlc.UpdateOrderShippingInfoParams) (ordersqlc.Order, error) {
	return r.db.UpdateOrderShippingInfo(ctx, params)
}

func (r *SqlOrderRepository) UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateOrderPaymentStatusParams) (ordersqlc.Order, error) {
	return r.getQuerier(tx).UpdateOrderPaymentStatus(ctx, params)
}

func (r *SqlOrderRepository) CountOrders(ctx context.Context, userID, status *int32, search string) (int64, error) {
	var searchPtr *string
	if strings.TrimSpace(search) != "" {
		trimmed := strings.TrimSpace(search)
		searchPtr = &trimmed
	}
	var statusPtr *int32
	if status != nil && *status > 0 {
		statusPtr = status
	}
	return r.db.CountOrders(ctx, ordersqlc.CountOrdersParams{
		UserID: userID,
		Status: statusPtr,
		Search: searchPtr,
	})
}

func (r *SqlOrderRepository) GetAll(
	ctx context.Context,
	userID, status *int32,
	search, orderBy, sort string,
	limit, offset int32,
) ([]ordersqlc.Order, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("db pool not configured for dynamic query")
	}

	query := `
		SELECT 
			o.order_id,
			o.order_uuid,
			o.order_number,
			o.order_user_id,
			o.order_shipping_name,
			o.order_shipping_phone,
			o.order_shipping_address,
			o.order_total_amount,
			o.order_refunded_amount,
			o.order_payment_method,
			o.order_payment_status,
			o.order_note,
			o.order_status,
			o.order_cancelled_at,
			o.order_created_at,
			o.order_updated_at,
			o.order_coupon_id,
			o.order_coupon_code,
			o.order_subtotal_amount,
			o.order_discount_amount
		FROM orders o
		WHERE 1=1
	`

	args := []any{}
	paramIdx := 1

	if userID != nil && *userID > 0 {
		query += fmt.Sprintf(" AND o.order_user_id = $%d", paramIdx)
		args = append(args, *userID)
		paramIdx++
	}

	if status != nil && *status > 0 {
		query += fmt.Sprintf(" AND o.order_status = $%d", paramIdx)
		args = append(args, *status)
		paramIdx++
	}

	if strings.TrimSpace(search) != "" {
		searchTerm := strings.TrimSpace(search)
		query += fmt.Sprintf(` AND (
			unaccent(o.order_number) ILIKE '%%' || unaccent($%d) || '%%'
			OR unaccent(o.order_shipping_name) ILIKE '%%' || unaccent($%d) || '%%'
			OR o.order_shipping_phone ILIKE '%%' || $%d || '%%'
			OR EXISTS (
				SELECT 1 FROM order_items oi
				WHERE oi.order_item_order_id = o.order_id
				AND unaccent(oi.order_item_product_name) ILIKE '%%' || unaccent($%d) || '%%'
			)
		)`, paramIdx, paramIdx, paramIdx, paramIdx)
		args = append(args, searchTerm)
		paramIdx++
	}

	sortOrder := "DESC"
	if strings.ToLower(strings.TrimSpace(sort)) == "asc" {
		sortOrder = "ASC"
	}

	switch strings.TrimSpace(orderBy) {
	case "order_total_amount":
		query += fmt.Sprintf(" ORDER BY o.order_total_amount %s, o.order_id ASC", sortOrder)
	default:
		query += fmt.Sprintf(" ORDER BY o.order_created_at %s, o.order_id ASC", sortOrder)
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]ordersqlc.Order, 0)
	for rows.Next() {
		var o ordersqlc.Order
		if err := rows.Scan(
			&o.OrderID,
			&o.OrderUuid,
			&o.OrderNumber,
			&o.OrderUserID,
			&o.OrderShippingName,
			&o.OrderShippingPhone,
			&o.OrderShippingAddress,
			&o.OrderTotalAmount,
			&o.OrderRefundedAmount,
			&o.OrderPaymentMethod,
			&o.OrderPaymentStatus,
			&o.OrderNote,
			&o.OrderStatus,
			&o.OrderCancelledAt,
			&o.OrderCreatedAt,
			&o.OrderUpdatedAt,
			&o.OrderCouponID,
			&o.OrderCouponCode,
			&o.OrderSubtotalAmount,
			&o.OrderDiscountAmount,
		); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	return orders, rows.Err()
}

func (r *SqlOrderRepository) UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params ordersqlc.UpdateOrderShippingAndStatusParams) (ordersqlc.Order, error) {
	return r.getQuerier(tx).UpdateOrderShippingAndStatus(ctx, params)
}

func (r *SqlOrderRepository) GetByShippingCode(ctx context.Context, shippingCode string) (ordersqlc.GetOrderByShippingCodeRow, error) {
	return r.db.GetOrderByShippingCode(ctx, &shippingCode)
}

func (r *SqlOrderRepository) MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (ordersqlc.Order, error) {
	return r.getQuerier(tx).MarkOrderDeliveredByShippingCode(ctx, &shippingCode)
}

func (r *SqlOrderRepository) MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error) {
	return r.getQuerier(tx).MarkOrderDeliveredByUUID(ctx, orderUUID)
}

func (r *SqlOrderRepository) CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (ordersqlc.Order, error) {
	return r.getQuerier(tx).CancelOrderShipping(ctx, orderUUID)
}

func (r *SqlOrderRepository) GetActiveShippedOrders(ctx context.Context, limit int32) ([]ordersqlc.Order, error) {
	return r.db.GetActiveShippedOrders(ctx, limit)
}
