package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	CountCategories(ctx context.Context, search string, deleted bool) (int64, error)
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error)
	GetBySlug(ctx context.Context, slug string) (sqlc.Category, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Update(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
}

type ProductRepository interface {
	Create(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error)
	CountProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error)
	GetAll(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error)
	GetBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	GetByUUIDs(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error)
	Update(ctx context.Context, productParams sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	DeductStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
	RestoreStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
}

type OrderRepository interface {
	Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderParams) (sqlc.Order, error)
	GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error)
	UpdateStatus(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error)
}
