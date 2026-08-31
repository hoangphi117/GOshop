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

func (r *SqlOrderRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderParams) (sqlc.Order, error) {

	q := sqlc.New(tx)

	order, err := q.CreateOrder(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}

	return order, nil
}

func (r *SqlOrderRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
	order, err := r.db.GetOrderByUUID(ctx, uuid)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, err
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

func (r *SqlOrderRepository) UpdateStatus(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
	order, err := r.db.UpdateOrderStatus(ctx, params)
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

func (r *SqlOrderRepository) UpdateOrderPaymentStatus(ctx context.Context, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error) {
	order, err := r.db.UpdateOrderPaymentStatus(ctx, params)
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
		SELECT o.*
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
		)`, paramIdx, paramIdx, paramIdx)
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
