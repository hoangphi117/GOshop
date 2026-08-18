package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type UserRepository interface {
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsers(ctx context.Context, search string, deleted bool) (int64, error)
	Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
}

type CategoryRepository interface {
	Create(ctx context.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error)
	GetBySlug(ctx context.Context, slug string) (sqlc.Category, error)
}
