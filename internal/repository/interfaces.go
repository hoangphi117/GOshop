package repository

import (
	"context"
	"goshop/internal/db/sqlc"
	"time"

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
	GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (sqlc.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error)
	UpdateStatus(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error)
	CountOrders(ctx context.Context, userID, status *int32, search string) (int64, error)
	GetAll(ctx context.Context, userID, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error)
}

type CartRepository interface {
	GetOrCreateCart(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartItems(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error)
	AddOrUpdateItem(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error)
	GetCartItemQuantity(ctx context.Context, cartID, productID int32) (int32, error)
	SetCartItemQuantity(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error)
	DeleteItem(ctx context.Context, cartID int32, itemUUID uuid.UUID) error
	ClearCart(ctx context.Context, cartID int32) error
	DeleteCartItemsByProductIDs(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error
}

type CouponRepository interface {
	Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	GetAll(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error)
	GetByCode(ctx context.Context, code string) (sqlc.Coupon, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	CountCoupons(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	CountUserUsages(ctx context.Context, userID, couponID int32) (int64, error)

	RecordUserUsage(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error)
	IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error
}

type PaymentRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error)
	GetByIdempotencyKey(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error)
	GetByTxnRef(ctx context.Context, tx pgx.Tx, provider, txnRef string) (sqlc.PaymentTransaction, error)
	GetByProviderTxnID(ctx context.Context, tx pgx.Tx, provider, providerTxnID string) (sqlc.PaymentTransaction, error)
	GetByOrderID(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error)
	UpdateStatusAtomically(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error)
	CreateRefund(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error)
	GetPendingTransactionsOlderThan(ctx context.Context, tx pgx.Tx, params sqlc.GetPendingTransactionsOlderThanParams) ([]sqlc.GetPendingTransactionsOlderThanRow, error)
}

type WebhookEventRepository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error)
	MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error
}

type AddressRepository interface {
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

type ReviewRow struct {
	ReviewID        int32
	ReviewUUID      uuid.UUID
	ReviewRating    int16
	ReviewTitle     *string
	ReviewComment   *string
	ReviewImages    []string
	ReviewVideoURL  *string
	ReviewHasMedia  bool
	ReviewCreatedAt time.Time
	ReviewUpdatedAt time.Time
	ReviewerName    string
}

type ReviewRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateReviewParams) (sqlc.ProductReview, error)
	CheckPurchased(ctx context.Context, orderID, userID, productID int32) (bool, error)
	CheckExists(ctx context.Context, userID, productID, orderID int32) (bool, error)
	RecalcRating(ctx context.Context, tx pgx.Tx, productID int32) error
	GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]ReviewRow, error)
	CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error)
	GetRatingSummary(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error)
	GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error)
	CountByUserID(ctx context.Context, userID int32) (int64, error)
}
