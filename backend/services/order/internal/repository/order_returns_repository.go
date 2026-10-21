package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SqlOrderReturnsRepository struct {
	db ordersqlc.Querier
}

func NewSqlOrderReturnsRepository(db ordersqlc.Querier) OrderReturnsRepository {
	return &SqlOrderReturnsRepository{
		db: db,
	}
}

func (r *SqlOrderReturnsRepository) getQuerier(tx pgx.Tx) ordersqlc.Querier {
	if tx != nil {
		return ordersqlc.New(tx)
	}
	return r.db
}

func (r *SqlOrderReturnsRepository) Create(ctx context.Context, tx pgx.Tx, params ordersqlc.CreateOrderReturnParams) (ordersqlc.OrderReturn, error) {
	return r.getQuerier(tx).CreateOrderReturn(ctx, params)
}

func (r *SqlOrderReturnsRepository) GetActiveByOrderID(ctx context.Context, orderID int32) (ordersqlc.OrderReturn, error) {
	return r.db.GetActiveReturnByOrderID(ctx, orderID)
}

func (r *SqlOrderReturnsRepository) GetLatestByOrderID(ctx context.Context, orderID int32) (ordersqlc.OrderReturn, error) {
	return r.db.GetLatestReturnByOrderID(ctx, orderID)
}

func (r *SqlOrderReturnsRepository) GetReturns(ctx context.Context, params ordersqlc.AdminGetReturnsParams) ([]ordersqlc.AdminGetReturnsRow, error) {
	return r.db.AdminGetReturns(ctx, params)
}

func (r *SqlOrderReturnsRepository) GetByUUID(ctx context.Context, returnUUID uuid.UUID) (ordersqlc.OrderReturn, error) {
	return r.db.GetReturnByUUID(ctx, returnUUID)
}

func (r *SqlOrderReturnsRepository) CountReturns(ctx context.Context, params ordersqlc.AdminCountReturnsParams) (int64, error) {
	return r.db.AdminCountReturns(ctx, params)
}

func (r *SqlOrderReturnsRepository) GetReturnsByUserID(ctx context.Context, params ordersqlc.GetReturnsByUserIDParams) ([]ordersqlc.GetReturnsByUserIDRow, error) {
	return r.db.GetReturnsByUserID(ctx, params)
}

func (r *SqlOrderReturnsRepository) CountReturnsByUserID(ctx context.Context, params ordersqlc.CountReturnsByUserIDParams) (int64, error) {
	return r.db.CountReturnsByUserID(ctx, params)
}

func (r *SqlOrderReturnsRepository) AdminGetReturnByUUID(ctx context.Context, returnUUID uuid.UUID) (ordersqlc.AdminGetReturnByUUIDRow, error) {
	return r.db.AdminGetReturnByUUID(ctx, returnUUID)
}

func (r *SqlOrderReturnsRepository) AdminGetLatestReturnByOrderID(ctx context.Context, orderID int32) (ordersqlc.AdminGetLatestReturnByOrderIDRow, error) {
	return r.db.AdminGetLatestReturnByOrderID(ctx, orderID)
}

func (r *SqlOrderReturnsRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnStatusParams) (ordersqlc.OrderReturn, error) {
	return r.getQuerier(tx).UpdateReturnStatus(ctx, params)
}

func (r *SqlOrderReturnsRepository) UpdateReturnShippingInfo(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnShippingInfoParams) (ordersqlc.OrderReturn, error) {
	return r.getQuerier(tx).UpdateReturnShippingInfo(ctx, params)
}

func (r *SqlOrderReturnsRepository) UpdateReturnBankAccount(ctx context.Context, tx pgx.Tx, params ordersqlc.UpdateReturnBankAccountParams) (ordersqlc.OrderReturn, error) {
	return r.getQuerier(tx).UpdateReturnBankAccount(ctx, params)
}
