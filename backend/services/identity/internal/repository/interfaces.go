package repository

import (
	"context"
	"goshop/services/identity/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserReader is the read-only user interface
type UserReader interface {
	GetByID(ctx context.Context, id int32) (sqlc.User, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetActiveAdminUserIDs(ctx context.Context) ([]int32, error)
}

// UserRepository defines all user storage operations in Identity service
type UserRepository interface {
	UserReader
	GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error)
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsers(ctx context.Context, search string, deleted bool) (int64, error)
	CountCustomersCreatedBetween(ctx context.Context, fromTime, toTime time.Time) (int64, error)
	Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error)
	CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error)
}

// AddressRepository defines address storage operations in Identity service
type AddressRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error)
	GetByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	GetByUserID(ctx context.Context, userID int32) ([]sqlc.UserAddress, error)
	ClearDefault(ctx context.Context, tx pgx.Tx, userID int32) error
	Update(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error)
	Count(ctx context.Context, userID int32) (int64, error)
	SetDefault(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	Delete(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (bool, error)
	SetLatestAsDefault(ctx context.Context, tx pgx.Tx, userID int32) error
}

// OutboxRepository defines transactional outbox storage in Identity service
type OutboxRepository interface {
	Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error)
	GetPending(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error)
	GetPendingByAggregateTypes(ctx context.Context, aggregateTypes []string, limit int32) ([]sqlc.OutboxEvent, error)
	MarkProcessed(ctx context.Context, eventIDs []int64) error
	MarkFailed(ctx context.Context, eventID int64, lastErr *string) error
}
