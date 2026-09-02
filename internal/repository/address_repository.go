package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SqlAddressRepository struct {
	db sqlc.Querier
}

func NewSqlAddressRepository(db sqlc.Querier) AddressRepository {
	return &SqlAddressRepository{
		db: db,
	}
}

func (uar *SqlAddressRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error) {
	q := sqlc.New(tx)

	return q.CreateUserAddress(ctx, params)
}

func (uar *SqlAddressRepository) Count(ctx context.Context, userID int32) (int64, error) {
	return uar.db.CountUserAddresses(ctx, userID)
}

func (uar *SqlAddressRepository) ClearDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	q := sqlc.New(tx)

	return q.ClearDefaultAddress(ctx, userID)
}

func (uar *SqlAddressRepository) Update(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
	return uar.db.UpdateUserAddress(ctx, params)
}

func (uar *SqlAddressRepository) SetDefault(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
	q := sqlc.New(tx)
	return q.SetDefaultAddress(ctx, sqlc.SetDefaultAddressParams{
		UaUserID: userID,
		UaUuid:   addressUUID,
	})
}

func (uar *SqlAddressRepository) Delete(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (bool, error) {
	q := sqlc.New(tx)
	result, err := q.DeleteUserAddress(ctx, sqlc.DeleteUserAddressParams{
		UaUserID: userID,
		UaUuid:   addressUUID,
	})
	if err != nil {
		return false, err
	}
	return result, nil
}

func (uar *SqlAddressRepository) SetLatestAsDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	q := sqlc.New(tx)
	return q.SetLatestAddressAsDefault(ctx, userID)
}
