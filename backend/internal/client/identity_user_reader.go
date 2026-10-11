package client

import (
	"context"
	"fmt"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"

	"github.com/google/uuid"
)

// IdentityUserReader implements repository.UserReader by querying the remote Identity microservice
// through IdentityClient (cached by Redis), with graceful fallback to a local projection reader.
type IdentityUserReader struct {
	identityClient IdentityClient
	localFallback  repository.UserReader
}

// NewIdentityUserReader creates a new UserReader backed by IdentityClient.
func NewIdentityUserReader(client IdentityClient, fallback repository.UserReader) repository.UserReader {
	return &IdentityUserReader{
		identityClient: client,
		localFallback:  fallback,
	}
}

func (r *IdentityUserReader) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if r.identityClient != nil {
		if dto, err := r.identityClient.GetUserByID(ctx, id); err == nil && dto != nil {
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
	}

	if r.localFallback != nil {
		return r.localFallback.GetByID(ctx, id)
	}

	return sqlc.User{}, fmt.Errorf("user not found: %d", id)
}

func (r *IdentityUserReader) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
	if r.identityClient != nil {
		if dto, err := r.identityClient.GetUserByUUID(ctx, u); err == nil && dto != nil {
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
	}

	if r.localFallback != nil {
		return r.localFallback.GetByUUID(ctx, u)
	}

	return sqlc.User{}, fmt.Errorf("user not found: %s", u.String())
}

func (r *IdentityUserReader) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if r.identityClient != nil {
		if adminIDs, err := r.identityClient.GetActiveAdminUserIDs(ctx); err == nil && len(adminIDs) > 0 {
			return adminIDs, nil
		}
	}

	if r.localFallback != nil {
		return r.localFallback.GetActiveAdminUserIDs(ctx)
	}

	return []int32{}, nil
}
