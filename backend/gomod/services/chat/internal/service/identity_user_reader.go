package service

import (
	"context"
	"fmt"

	identitycontract "goshop/services/identity/contract"

	"github.com/google/uuid"
)

// IdentityUserReader adapts the identity gRPC contract to the chat domain's
// UserReader port.
type IdentityUserReader struct {
	client identitycontract.IdentityClient
}

func NewIdentityUserReader(client identitycontract.IdentityClient) *IdentityUserReader {
	return &IdentityUserReader{client: client}
}

func (r *IdentityUserReader) toUser(dto *identitycontract.InternalUserDTO) (User, error) {
	if dto == nil {
		return User{}, fmt.Errorf("identity client returned no user")
	}
	return User{
		UserID:       dto.UserID,
		UserUUID:     dto.UserUUID,
		UserFullname: dto.UserFullname,
		UserEmail:    dto.UserEmail,
		UserAvatar:   dto.UserAvatar,
		UserStatus:   dto.UserStatus,
		UserLevel:    dto.UserLevel,
	}, nil
}

func (r *IdentityUserReader) GetByUUID(ctx context.Context, userUUID uuid.UUID) (User, error) {
	dto, err := r.client.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return User{}, err
	}
	return r.toUser(dto)
}

func (r *IdentityUserReader) GetByID(ctx context.Context, id int32) (User, error) {
	dto, err := r.client.GetUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return r.toUser(dto)
}
