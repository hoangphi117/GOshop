package repository

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SqlOrderRepository struct {
	db sqlc.Querier
}

func NewSqlOrderRepository(db sqlc.Querier) OrderRepository {
	return &SqlOrderRepository{
		db: db,
	}
}

func (r *SqlOrderRepository) Create(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {

	q := db.GetQuerier(ctx, r.db)
	return q.CreateOrder(ctx, params)
}

func (r *SqlOrderRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
	order, err := r.db.GetOrderByUUID(ctx, uuid)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, err
	}
	return order, nil
}
func (r *SqlOrderRepository) GetByID(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
	order, err := r.db.GetOrderByID(ctx, orderID)
	if err != nil {
		return sqlc.GetOrderByIDRow{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}
	order, err := q.GetOrderByUUIDForUpdate(ctx, uuid)
	if err != nil {
		return sqlc.Order{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error) {
	order, err := r.db.GetOrderByOrderNumber(ctx, orderNumber)
	if err != nil {
		return sqlc.GetOrderByOrderNumberRow{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error) {
	lastOrderNumber, err := r.db.GetLatestOrderNumber(ctx, orderNumber)
	if err != nil {
		return "", err
	}
	return lastOrderNumber, nil
}

func (r *SqlOrderRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}
	order, err := q.UpdateOrderStatus(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {

	q := sqlc.New(tx)

	order, err := q.CancelOrder(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error) {
	order, err := r.db.UpdateOrderShippingInfo(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}
	return order, nil
}

func (r *SqlOrderRepository) UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx) // Run in transaction's context
	}
	order, err := q.UpdateOrderPaymentStatus(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}

	return order, nil
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
	count, err := r.db.CountOrders(ctx, sqlc.CountOrdersParams{
		UserID: userID,
		Status: statusPtr,
		Search: searchPtr,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SqlOrderRepository) GetAll(
	ctx context.Context,
	userID, status *int32,
	search, orderBy, sort string,
	limit, offset int32,
) ([]sqlc.Order, error) {
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

	rows, err := db.DBPool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]sqlc.Order, 0)
	for rows.Next() {
		var o sqlc.Order
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// Shipping

func (r *SqlOrderRepository) UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}

	order, err := q.UpdateOrderShippingAndStatus(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}

	return order, nil
}

func (r *SqlOrderRepository) GetByShippingCode(ctx context.Context, shippingCode string) (sqlc.GetOrderByShippingCodeRow, error) {
	return r.db.GetOrderByShippingCode(ctx, &shippingCode)
}

func (r *SqlOrderRepository) MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}

	return q.MarkOrderDeliveredByShippingCode(ctx, &shippingCode)
}

func (r *SqlOrderRepository) MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}

	return q.MarkOrderDeliveredByUUID(ctx, orderUUID)
}

func (r *SqlOrderRepository) CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}

	order, err := q.CancelOrderShipping(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, err
	}

	return order, nil
}

func (r *SqlOrderRepository) GetActiveShippedOrders(ctx context.Context, limit int32) ([]sqlc.Order, error) {
	return r.db.GetActiveShippedOrders(ctx, limit)
}
