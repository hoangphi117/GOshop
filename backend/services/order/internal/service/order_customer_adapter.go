package service

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"
	identitycontract "goshop/services/identity/contract"

	"github.com/google/uuid"
)

type LegacyUserReader interface {
	GetByUUID(ctx context.Context, u uuid.UUID) (ordersqlc.User, error)
	GetByID(ctx context.Context, id int32) (ordersqlc.User, error)
}

type IdentityUserClient interface {
	GetUserByUUID(ctx context.Context, u uuid.UUID) (*identitycontract.InternalUserDTO, error)
	GetUserByID(ctx context.Context, id int32) (*identitycontract.InternalUserDTO, error)
}

type legacyUserAdapter struct {
	reader LegacyUserReader
}

func (a *legacyUserAdapter) GetByUUID(ctx context.Context, u uuid.UUID) (*OrderCustomer, error) {
	user, err := a.reader.GetByUUID(ctx, u)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(user.UserID, user.UserUuid, user.UserFullname, user.UserEmail, user.UserLevel, user.UserStatus), nil
}

func (a *legacyUserAdapter) GetByID(ctx context.Context, id int32) (*OrderCustomer, error) {
	user, err := a.reader.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(user.UserID, user.UserUuid, user.UserFullname, user.UserEmail, user.UserLevel, user.UserStatus), nil
}

type grpcIdentityAdapter struct {
	client IdentityUserClient
}

func (a *grpcIdentityAdapter) GetByUUID(ctx context.Context, u uuid.UUID) (*OrderCustomer, error) {
	user, err := a.client.GetUserByUUID(ctx, u)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(user.UserID, user.UserUUID, user.UserFullname, user.UserEmail, user.UserLevel, user.UserStatus), nil
}

func (a *grpcIdentityAdapter) GetByID(ctx context.Context, id int32) (*OrderCustomer, error) {
	user, err := a.client.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(user.UserID, user.UserUUID, user.UserFullname, user.UserEmail, user.UserLevel, user.UserStatus), nil
}

// NewOrderCustomerAdapter creates a new OrderCustomerReader backed by either an IdentityClient, a LegacyUserReader, or directly returns an OrderCustomerReader.
func NewOrderCustomerAdapter(source any) OrderCustomerReader {
	if source == nil {
		return nil
	}
	if cr, ok := source.(OrderCustomerReader); ok {
		return cr
	}
	if lr, ok := source.(LegacyUserReader); ok {
		return &legacyUserAdapter{reader: lr}
	}
	if ic, ok := source.(IdentityUserClient); ok {
		return &grpcIdentityAdapter{client: ic}
	}
	return nil
}
