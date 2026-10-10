package v1service

import (
	"context"
	"goshop/internal/repository"

	"github.com/google/uuid"
)

// OrderCustomerAdapter adapts repository.UserReader to OrderCustomerReader.
type OrderCustomerAdapter struct {
	userReader repository.UserReader
}

// NewOrderCustomerAdapter creates a new OrderCustomerAdapter.
func NewOrderCustomerAdapter(userReader repository.UserReader) OrderCustomerReader {
	if userReader == nil {
		return nil
	}
	return &OrderCustomerAdapter{userReader: userReader}
}

func (a *OrderCustomerAdapter) GetByUUID(ctx context.Context, u uuid.UUID) (*OrderCustomer, error) {
	user, err := a.userReader.GetByUUID(ctx, u)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(
		user.UserID,
		user.UserUuid,
		user.UserFullname,
		user.UserEmail,
		user.UserLevel,
		user.UserStatus,
	), nil
}

func (a *OrderCustomerAdapter) GetByID(ctx context.Context, id int32) (*OrderCustomer, error) {
	user, err := a.userReader.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewOrderCustomer(
		user.UserID,
		user.UserUuid,
		user.UserFullname,
		user.UserEmail,
		user.UserLevel,
		user.UserStatus,
	), nil
}
