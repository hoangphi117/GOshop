package repository

import (
	"context"
	"goshop/internal/db/sqlc"
)

type UserRepository interface {
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	CountUsers(ctx context.Context, search string) (int64, error)
	Create(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error)
	FindByUUID(uuid string)
	Update(uuid string)
	Delete(uuid string)
	FindByEmail(email string)
}
