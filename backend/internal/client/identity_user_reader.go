package client

import (
	"context"
	"fmt"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/services/identity/contract"

	"github.com/google/uuid"
)

// IdentityUserReader implements repository.UserReader by calling IdentityClient directly without local DB fallback.
type IdentityUserReader struct {
	identityClient contract.IdentityClient
}

// NewIdentityUserReader creates a new UserReader backed by IdentityClient without local DB fallback.
func NewIdentityUserReader(client contract.IdentityClient) repository.UserReader {
	return &IdentityUserReader{
		identityClient: client,
	}
}

func (r *IdentityUserReader) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if r.identityClient == nil {
		return sqlc.User{}, fmt.Errorf("identity client is not initialized")
	}
	dto, err := r.identityClient.GetUserByID(ctx, id)
	if err != nil {
		return sqlc.User{}, err
	}
	if dto == nil {
		return sqlc.User{}, fmt.Errorf("user not found: %d", id)
	}
	return sqlc.User{
		UserID:        dto.UserID,
		UserUuid:      dto.UserUUID,
		UserFullname:  dto.UserFullname,
		UserEmail:     dto.UserEmail,
		UserAvatar:    dto.UserAvatar,
		UserAvatarUrl: dto.UserAvatarUrl,
		UserStatus:    dto.UserStatus,
		UserLevel:     dto.UserLevel,
	}, nil
}

func (r *IdentityUserReader) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
	if r.identityClient == nil {
		return sqlc.User{}, fmt.Errorf("identity client is not initialized")
	}
	dto, err := r.identityClient.GetUserByUUID(ctx, u)
	if err != nil {
		return sqlc.User{}, err
	}
	if dto == nil {
		return sqlc.User{}, fmt.Errorf("user not found: %s", u.String())
	}
	return sqlc.User{
		UserID:        dto.UserID,
		UserUuid:      dto.UserUUID,
		UserFullname:  dto.UserFullname,
		UserEmail:     dto.UserEmail,
		UserAvatar:    dto.UserAvatar,
		UserAvatarUrl: dto.UserAvatarUrl,
		UserStatus:    dto.UserStatus,
		UserLevel:     dto.UserLevel,
	}, nil
}

func (r *IdentityUserReader) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if r.identityClient == nil {
		return nil, fmt.Errorf("identity client is not initialized")
	}
	return r.identityClient.GetActiveAdminUserIDs(ctx)
}
