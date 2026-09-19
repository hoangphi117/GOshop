package testutils

import (
	"context"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Ensure all mocks implement repository interfaces at compile time
var (
	_ repository.OrderRepository        = (*MockOrderRepository)(nil)
	_ repository.OrderItemRepository    = (*MockOrderItemRepository)(nil)
	_ repository.ProductRepository      = (*MockProductRepository)(nil)
	_ repository.CouponRepository       = (*MockCouponRepository)(nil)
	_ repository.CartRepository         = (*MockCartRepository)(nil)
	_ repository.TrackingRepository     = (*MockTrackingRepository)(nil)
	_ repository.UserRepository         = (*MockUserRepository)(nil)
	_ repository.WebhookEventRepository = (*MockWebhookEventRepository)(nil)
	_ repository.OrderReturnsRepository = (*MockOrderReturnsRepository)(nil)
	_ repository.PaymentRepository      = (*MockPaymentRepository)(nil)
	_ repository.CategoryRepository     = (*MockCategoryRepository)(nil)
	_ repository.ReviewRepository       = (*MockReviewRepository)(nil)
	_ repository.AddressRepository      = (*MockAddressRepository)(nil)
	_ repository.NotificationRepository = (*MockNotificationRepository)(nil)
)

// =========================================================================
// 1. MockOrderRepository
// =========================================================================
type MockOrderRepository struct {
	CreateFunc                      func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error)
	GetLastOrderNumberFunc          func(ctx context.Context, orderNumber string) (string, error)
	GetByUUIDFunc                   func(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error)
	GetByIDFunc                     func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error)
	GetByUUIDForUpdateFunc          func(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (sqlc.Order, error)
	GetByOrderNumberFunc            func(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error)
	UpdateStatusFunc                func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrderFunc                 func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfoFunc     func(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
	UpdateOrderPaymentStatusFunc    func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error)
	CountOrdersFunc                 func(ctx context.Context, userID, status *int32, search string) (int64, error)
	GetAllFunc                      func(ctx context.Context, userID, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error)
	UpdateShippingAndStatusFunc     func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error)
	GetByShippingCodeFunc           func(ctx context.Context, shippingCode string) (sqlc.GetOrderByShippingCodeRow, error)
	MarkDeliveredByShippingCodeFunc func(ctx context.Context, tx pgx.Tx, shippingCode string) (sqlc.Order, error)
	MarkDeliveredByUUIDFunc         func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	CancelShippingFunc              func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	GetActiveShippedOrdersFunc      func(ctx context.Context, limit int32) ([]sqlc.Order, error)
}

func (m *MockOrderRepository) Create(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error) {
	if m.GetLastOrderNumberFunc != nil {
		return m.GetLastOrderNumberFunc(ctx, orderNumber)
	}
	return "", nil
}

func (m *MockOrderRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.GetOrderByUUIDRow{}, nil
}

func (m *MockOrderRepository) GetByID(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, orderID)
	}
	return sqlc.GetOrderByIDRow{}, nil
}

func (m *MockOrderRepository) GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (sqlc.Order, error) {
	if m.GetByUUIDForUpdateFunc != nil {
		return m.GetByUUIDForUpdateFunc(ctx, tx, uuid)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error) {
	if m.GetByOrderNumberFunc != nil {
		return m.GetByOrderNumberFunc(ctx, orderNumber)
	}
	return sqlc.GetOrderByOrderNumberRow{}, nil
}

func (m *MockOrderRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, tx, params)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {
	if m.CancelOrderFunc != nil {
		return m.CancelOrderFunc(ctx, tx, orderUUID)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error) {
	if m.UpdateOrderShippingInfoFunc != nil {
		return m.UpdateOrderShippingInfoFunc(ctx, params)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error) {
	if m.UpdateOrderPaymentStatusFunc != nil {
		return m.UpdateOrderPaymentStatusFunc(ctx, tx, params)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) CountOrders(ctx context.Context, userID, status *int32, search string) (int64, error) {
	if m.CountOrdersFunc != nil {
		return m.CountOrdersFunc(ctx, userID, status, search)
	}
	return 0, nil
}

func (m *MockOrderRepository) GetAll(ctx context.Context, userID, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, userID, status, search, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockOrderRepository) UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error) {
	if m.UpdateShippingAndStatusFunc != nil {
		return m.UpdateShippingAndStatusFunc(ctx, tx, orderUUID, params)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) GetByShippingCode(ctx context.Context, shippingCode string) (sqlc.GetOrderByShippingCodeRow, error) {
	if m.GetByShippingCodeFunc != nil {
		return m.GetByShippingCodeFunc(ctx, shippingCode)
	}
	return sqlc.GetOrderByShippingCodeRow{}, nil
}

func (m *MockOrderRepository) MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (sqlc.Order, error) {
	if m.MarkDeliveredByShippingCodeFunc != nil {
		return m.MarkDeliveredByShippingCodeFunc(ctx, tx, shippingCode)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {
	if m.MarkDeliveredByUUIDFunc != nil {
		return m.MarkDeliveredByUUIDFunc(ctx, tx, orderUUID)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error) {
	if m.CancelShippingFunc != nil {
		return m.CancelShippingFunc(ctx, tx, orderUUID)
	}
	return sqlc.Order{}, nil
}

func (m *MockOrderRepository) GetActiveShippedOrders(ctx context.Context, limit int32) ([]sqlc.Order, error) {
	if m.GetActiveShippedOrdersFunc != nil {
		return m.GetActiveShippedOrdersFunc(ctx, limit)
	}
	return nil, nil
}

// =========================================================================
// 2. MockOrderItemRepository
// =========================================================================
type MockOrderItemRepository struct {
	CreateFunc       func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error)
	GetByOrderIDFunc func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error)
}

func (m *MockOrderItemRepository) Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, input)
	}
	return sqlc.OrderItem{}, nil
}

func (m *MockOrderItemRepository) GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
	if m.GetByOrderIDFunc != nil {
		return m.GetByOrderIDFunc(ctx, orderID)
	}
	return nil, nil
}

// =========================================================================
// 3. MockProductRepository
// =========================================================================
type MockProductRepository struct {
	CreateFunc        func(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error)
	CountProductsFunc func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error)
	GetAllFunc        func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error)
	GetBySlugFunc     func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error)
	GetByUUIDFunc     func(ctx context.Context, uuid uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	GetByUUIDsFunc    func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error)
	UpdateFunc        func(ctx context.Context, productParams sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error)
	SoftDeleteFunc    func(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	RestoreFunc       func(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	TrashFunc         func(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	DeductStockFunc   func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
	RestoreStockFunc  func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
}

func (m *MockProductRepository) Create(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, productParams)
	}
	return sqlc.CreateProductRow{}, nil
}

func (m *MockProductRepository) CountProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error) {
	if m.CountProductsFunc != nil {
		return m.CountProductsFunc(ctx, search, categoryUUID, minPrice, maxPrice, status, deleted)
	}
	return 0, nil
}

func (m *MockProductRepository) GetAll(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, categoryUUID, minPrice, maxPrice, status, orderBy, sort, limit, offset, deleted)
	}
	return nil, nil
}

func (m *MockProductRepository) GetBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
	if m.GetBySlugFunc != nil {
		return m.GetBySlugFunc(ctx, slug)
	}
	return sqlc.GetProductBySlugRow{}, nil
}

func (m *MockProductRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.GetProductByUUIDRow{}, nil
}

func (m *MockProductRepository) GetByUUIDs(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
	if m.GetByUUIDsFunc != nil {
		return m.GetByUUIDsFunc(ctx, tx, uuids)
	}
	return nil, nil
}

func (m *MockProductRepository) Update(ctx context.Context, productParams sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, productParams)
	}
	return sqlc.UpdateProductRow{}, nil
}

func (m *MockProductRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, uuid)
	}
	return sqlc.Product{}, nil
}

func (m *MockProductRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, uuid)
	}
	return sqlc.Product{}, nil
}

func (m *MockProductRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, uuid)
	}
	return sqlc.Product{}, nil
}

func (m *MockProductRepository) DeductStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
	if m.DeductStockFunc != nil {
		return m.DeductStockFunc(ctx, tx, productID, quantity)
	}
	return nil
}

func (m *MockProductRepository) RestoreStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
	if m.RestoreStockFunc != nil {
		return m.RestoreStockFunc(ctx, tx, productID, quantity)
	}
	return nil
}

// =========================================================================
// 4. MockCouponRepository
// =========================================================================
type MockCouponRepository struct {
	CreateFunc             func(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	GetAllFunc             func(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error)
	GetByCodeFunc          func(ctx context.Context, code string) (sqlc.Coupon, error)
	GetByUUIDFunc          func(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	UpdateFunc             func(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	CountCouponsFunc       func(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error)
	SoftDeleteFunc         func(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	RestoreFunc            func(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	TrashFunc              func(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	CountUserUsagesFunc    func(ctx context.Context, userID, couponID int32) (int64, error)
	RecordUserUsageFunc    func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error)
	IncrementUsedCountFunc func(ctx context.Context, tx pgx.Tx, couponID int32) error
}

func (m *MockCouponRepository) Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) GetAll(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, orderBy, sort, coupon_type, isActive, limit, offset, deleted)
	}
	return nil, nil
}

func (m *MockCouponRepository) GetByCode(ctx context.Context, code string) (sqlc.Coupon, error) {
	if m.GetByCodeFunc != nil {
		return m.GetByCodeFunc(ctx, code)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) CountCoupons(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error) {
	if m.CountCouponsFunc != nil {
		return m.CountCouponsFunc(ctx, search, coupon_type, isActive, deleted)
	}
	return 0, nil
}

func (m *MockCouponRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, uuid)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, uuid)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error) {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, uuid)
	}
	return sqlc.Coupon{}, nil
}

func (m *MockCouponRepository) CountUserUsages(ctx context.Context, userID, couponID int32) (int64, error) {
	if m.CountUserUsagesFunc != nil {
		return m.CountUserUsagesFunc(ctx, userID, couponID)
	}
	return 0, nil
}

func (m *MockCouponRepository) RecordUserUsage(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
	if m.RecordUserUsageFunc != nil {
		return m.RecordUserUsageFunc(ctx, tx, params)
	}
	return sqlc.CouponUsage{}, nil
}

func (m *MockCouponRepository) IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error {
	if m.IncrementUsedCountFunc != nil {
		return m.IncrementUsedCountFunc(ctx, tx, couponID)
	}
	return nil
}

// =========================================================================
// 5. MockCartRepository
// =========================================================================
type MockCartRepository struct {
	GetOrCreateCartFunc             func(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartByUserIDFunc             func(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartItemsFunc                func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error)
	AddOrUpdateItemFunc             func(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error)
	GetCartItemQuantityFunc         func(ctx context.Context, cartID, productID int32) (int32, error)
	SetCartItemQuantityFunc         func(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error)
	DeleteItemFunc                  func(ctx context.Context, cartID int32, itemUUID uuid.UUID) error
	ClearCartFunc                   func(ctx context.Context, cartID int32) error
	DeleteCartItemsByProductIDsFunc func(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error
}

func (m *MockCartRepository) GetOrCreateCart(ctx context.Context, userID int32) (sqlc.Cart, error) {
	if m.GetOrCreateCartFunc != nil {
		return m.GetOrCreateCartFunc(ctx, userID)
	}
	return sqlc.Cart{}, nil
}

func (m *MockCartRepository) GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error) {
	if m.GetCartByUserIDFunc != nil {
		return m.GetCartByUserIDFunc(ctx, userID)
	}
	return sqlc.Cart{}, nil
}

func (m *MockCartRepository) GetCartItems(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
	if m.GetCartItemsFunc != nil {
		return m.GetCartItemsFunc(ctx, cartID)
	}
	return nil, nil
}

func (m *MockCartRepository) AddOrUpdateItem(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error) {
	if m.AddOrUpdateItemFunc != nil {
		return m.AddOrUpdateItemFunc(ctx, params)
	}
	return sqlc.CartItem{}, nil
}

func (m *MockCartRepository) GetCartItemQuantity(ctx context.Context, cartID, productID int32) (int32, error) {
	if m.GetCartItemQuantityFunc != nil {
		return m.GetCartItemQuantityFunc(ctx, cartID, productID)
	}
	return 0, nil
}

func (m *MockCartRepository) SetCartItemQuantity(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error) {
	if m.SetCartItemQuantityFunc != nil {
		return m.SetCartItemQuantityFunc(ctx, params)
	}
	return sqlc.CartItem{}, nil
}

func (m *MockCartRepository) DeleteItem(ctx context.Context, cartID int32, itemUUID uuid.UUID) error {
	if m.DeleteItemFunc != nil {
		return m.DeleteItemFunc(ctx, cartID, itemUUID)
	}
	return nil
}

func (m *MockCartRepository) ClearCart(ctx context.Context, cartID int32) error {
	if m.ClearCartFunc != nil {
		return m.ClearCartFunc(ctx, cartID)
	}
	return nil
}

func (m *MockCartRepository) DeleteCartItemsByProductIDs(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error {
	if m.DeleteCartItemsByProductIDsFunc != nil {
		return m.DeleteCartItemsByProductIDsFunc(ctx, tx, cartID, productIDs)
	}
	return nil
}

// =========================================================================
// 6. MockTrackingRepository
// =========================================================================
type MockTrackingRepository struct {
	InsertEventFunc  func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error)
	GetByOrderIDFunc func(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error)
}

func (m *MockTrackingRepository) InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
	if m.InsertEventFunc != nil {
		return m.InsertEventFunc(ctx, tx, params)
	}
	return sqlc.OrderTrackingEvent{}, nil
}

func (m *MockTrackingRepository) GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error) {
	if m.GetByOrderIDFunc != nil {
		return m.GetByOrderIDFunc(ctx, orderID)
	}
	return nil, nil
}

// =========================================================================
// 7. MockUserRepository
// =========================================================================
type MockUserRepository struct {
	GetAllFunc         func(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2Func       func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsersFunc     func(ctx context.Context, search string, deleted bool) (int64, error)
	CreateFunc         func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetByUUIDFunc      func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	UpdateFunc         func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDeleteFunc     func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	RestoreFunc        func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	TrashFunc          func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmailFunc     func(ctx context.Context, email string) (sqlc.User, error)
	UpdatePasswordFunc func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
}

func (m *MockUserRepository) GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockUserRepository) GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
	if m.GetAllV2Func != nil {
		return m.GetAllV2Func(ctx, search, orderBy, sort, limit, offset, deleted)
	}
	return nil, nil
}

func (m *MockUserRepository) CountUsers(ctx context.Context, search string, deleted bool) (int64, error) {
	if m.CountUsersFunc != nil {
		return m.CountUsersFunc(ctx, search, deleted)
	}
	return 0, nil
}

func (m *MockUserRepository) Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

// =========================================================================
// 8. MockWebhookEventRepository
// =========================================================================
type MockWebhookEventRepository struct {
	InsertEventFunc   func(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error)
	MarkProcessedFunc func(ctx context.Context, tx pgx.Tx, eventID int32) error
}

func (m *MockWebhookEventRepository) InsertEvent(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
	if m.InsertEventFunc != nil {
		return m.InsertEventFunc(ctx, tx, input)
	}
	return sqlc.WebhookEvent{}, nil
}

func (m *MockWebhookEventRepository) MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error {
	if m.MarkProcessedFunc != nil {
		return m.MarkProcessedFunc(ctx, tx, eventID)
	}
	return nil
}

// =========================================================================
// 9. MockOrderReturnsRepository
// =========================================================================
type MockOrderReturnsRepository struct {
	CreateFunc                        func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error)
	GetActiveByOrderIDFunc            func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetLatestByOrderIDFunc            func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetReturnsFunc                    func(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error)
	GetByUUIDFunc                     func(ctx context.Context, returnUUID uuid.UUID) (sqlc.OrderReturn, error)
	CountReturnsFunc                  func(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error)
	GetReturnsByUserIDFunc            func(ctx context.Context, params sqlc.GetReturnsByUserIDParams) ([]sqlc.GetReturnsByUserIDRow, error)
	CountReturnsByUserIDFunc          func(ctx context.Context, params sqlc.CountReturnsByUserIDParams) (int64, error)
	AdminGetReturnByUUIDFunc          func(ctx context.Context, returnUUID uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error)
	AdminGetLatestReturnByOrderIDFunc func(ctx context.Context, orderID int32) (sqlc.AdminGetLatestReturnByOrderIDRow, error)
	UpdateStatusFunc                  func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error)
	UpdateReturnShippingInfoFunc      func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error)
	UpdateReturnBankAccountFunc       func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error)
}

func (m *MockOrderReturnsRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, params)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) GetActiveByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
	if m.GetActiveByOrderIDFunc != nil {
		return m.GetActiveByOrderIDFunc(ctx, orderID)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) GetLatestByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
	if m.GetLatestByOrderIDFunc != nil {
		return m.GetLatestByOrderIDFunc(ctx, orderID)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) GetReturns(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error) {
	if m.GetReturnsFunc != nil {
		return m.GetReturnsFunc(ctx, params)
	}
	return nil, nil
}

func (m *MockOrderReturnsRepository) GetByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.OrderReturn, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, returnUUID)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) CountReturns(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error) {
	if m.CountReturnsFunc != nil {
		return m.CountReturnsFunc(ctx, params)
	}
	return 0, nil
}

func (m *MockOrderReturnsRepository) GetReturnsByUserID(ctx context.Context, params sqlc.GetReturnsByUserIDParams) ([]sqlc.GetReturnsByUserIDRow, error) {
	if m.GetReturnsByUserIDFunc != nil {
		return m.GetReturnsByUserIDFunc(ctx, params)
	}
	return nil, nil
}

func (m *MockOrderReturnsRepository) CountReturnsByUserID(ctx context.Context, params sqlc.CountReturnsByUserIDParams) (int64, error) {
	if m.CountReturnsByUserIDFunc != nil {
		return m.CountReturnsByUserIDFunc(ctx, params)
	}
	return 0, nil
}

func (m *MockOrderReturnsRepository) AdminGetReturnByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error) {
	if m.AdminGetReturnByUUIDFunc != nil {
		return m.AdminGetReturnByUUIDFunc(ctx, returnUUID)
	}
	return sqlc.AdminGetReturnByUUIDRow{}, nil
}

func (m *MockOrderReturnsRepository) AdminGetLatestReturnByOrderID(ctx context.Context, orderID int32) (sqlc.AdminGetLatestReturnByOrderIDRow, error) {
	if m.AdminGetLatestReturnByOrderIDFunc != nil {
		return m.AdminGetLatestReturnByOrderIDFunc(ctx, orderID)
	}
	return sqlc.AdminGetLatestReturnByOrderIDRow{}, nil
}

func (m *MockOrderReturnsRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, tx, params)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) UpdateReturnShippingInfo(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error) {
	if m.UpdateReturnShippingInfoFunc != nil {
		return m.UpdateReturnShippingInfoFunc(ctx, tx, params)
	}
	return sqlc.OrderReturn{}, nil
}

func (m *MockOrderReturnsRepository) UpdateReturnBankAccount(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error) {
	if m.UpdateReturnBankAccountFunc != nil {
		return m.UpdateReturnBankAccountFunc(ctx, tx, params)
	}
	return sqlc.OrderReturn{}, nil
}

// =========================================================================
// 10. MockPaymentRepository
// =========================================================================
type MockPaymentRepository struct {
	CreateFunc                          func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error)
	GetByIdempotencyKeyFunc             func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error)
	GetByTxnRefFunc                     func(ctx context.Context, tx pgx.Tx, provider, txnRef string) (sqlc.PaymentTransaction, error)
	GetByProviderTxnIDFunc              func(ctx context.Context, tx pgx.Tx, provider, providerTxnID string) (sqlc.PaymentTransaction, error)
	GetByOrderIDFunc                    func(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error)
	UpdateStatusAtomicallyFunc          func(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error)
	CreateRefundFunc                    func(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error)
	GetPendingTransactionsOlderThanFunc func(ctx context.Context, tx pgx.Tx, params sqlc.GetPendingTransactionsOlderThanParams) ([]sqlc.GetPendingTransactionsOlderThanRow, error)
}

func (m *MockPaymentRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, params)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) GetByIdempotencyKey(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
	if m.GetByIdempotencyKeyFunc != nil {
		return m.GetByIdempotencyKeyFunc(ctx, tx, idempotencyKey)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) GetByTxnRef(ctx context.Context, tx pgx.Tx, provider, txnRef string) (sqlc.PaymentTransaction, error) {
	if m.GetByTxnRefFunc != nil {
		return m.GetByTxnRefFunc(ctx, tx, provider, txnRef)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) GetByProviderTxnID(ctx context.Context, tx pgx.Tx, provider, providerTxnID string) (sqlc.PaymentTransaction, error) {
	if m.GetByProviderTxnIDFunc != nil {
		return m.GetByProviderTxnIDFunc(ctx, tx, provider, providerTxnID)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) GetByOrderID(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
	if m.GetByOrderIDFunc != nil {
		return m.GetByOrderIDFunc(ctx, tx, orderID)
	}
	return nil, nil
}

func (m *MockPaymentRepository) UpdateStatusAtomically(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error) {
	if m.UpdateStatusAtomicallyFunc != nil {
		return m.UpdateStatusAtomicallyFunc(ctx, tx, params)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) CreateRefund(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error) {
	if m.CreateRefundFunc != nil {
		return m.CreateRefundFunc(ctx, tx, params)
	}
	return sqlc.PaymentTransaction{}, nil
}

func (m *MockPaymentRepository) GetPendingTransactionsOlderThan(ctx context.Context, tx pgx.Tx, params sqlc.GetPendingTransactionsOlderThanParams) ([]sqlc.GetPendingTransactionsOlderThanRow, error) {
	if m.GetPendingTransactionsOlderThanFunc != nil {
		return m.GetPendingTransactionsOlderThanFunc(ctx, tx, params)
	}
	return nil, nil
}

// =========================================================================
// 11. MockCategoryRepository
// =========================================================================
type MockCategoryRepository struct {
	CreateFunc          func(ctx context.Context, categoryParams sqlc.CreateCategoryParams) (sqlc.Category, error)
	CountCategoriesFunc func(ctx context.Context, search string, deleted bool) (int64, error)
	GetAllFunc          func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error)
	GetBySlugFunc       func(ctx context.Context, slug string) (sqlc.Category, error)
	GetByUUIDFunc       func(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	UpdateFunc          func(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDeleteFunc      func(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	RestoreFunc         func(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	TrashFunc           func(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
}

func (m *MockCategoryRepository) Create(ctx context.Context, categoryParams sqlc.CreateCategoryParams) (sqlc.Category, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, categoryParams)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) CountCategories(ctx context.Context, search string, deleted bool) (int64, error) {
	if m.CountCategoriesFunc != nil {
		return m.CountCategoriesFunc(ctx, search, deleted)
	}
	return 0, nil
}

func (m *MockCategoryRepository) GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, orderBy, sort, limit, offset, deleted)
	}
	return nil, nil
}

func (m *MockCategoryRepository) GetBySlug(ctx context.Context, slug string) (sqlc.Category, error) {
	if m.GetBySlugFunc != nil {
		return m.GetBySlugFunc(ctx, slug)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) Update(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, input)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, uuid)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, uuid)
	}
	return sqlc.Category{}, nil
}

func (m *MockCategoryRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, uuid)
	}
	return sqlc.Category{}, nil
}

// =========================================================================
// 12. MockReviewRepository
// =========================================================================
type MockReviewRepository struct {
	CreateFunc           func(ctx context.Context, tx pgx.Tx, params sqlc.CreateReviewParams) (sqlc.ProductReview, error)
	CheckPurchasedFunc   func(ctx context.Context, orderID, userID, productID int32) (bool, error)
	CheckExistsFunc      func(ctx context.Context, userID, productID, orderID int32) (bool, error)
	RecalcRatingFunc     func(ctx context.Context, tx pgx.Tx, productID int32) error
	GetByProductFunc     func(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]v1dto.ReviewRow, error)
	CountByProductFunc   func(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error)
	GetRatingSummaryFunc func(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error)
	GetByUserIDFunc      func(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error)
	CountByUserIDFunc    func(ctx context.Context, userID int32) (int64, error)
	GetByUUIDFunc        func(ctx context.Context, reviewUUID uuid.UUID) (sqlc.GetReviewByUUIDRow, error)
	UpdateFunc           func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReviewParams) (sqlc.ProductReview, error)
	DeleteFunc           func(ctx context.Context, tx pgx.Tx, reviewID int32) error
	HasReportedFunc      func(ctx context.Context, userID, reviewID int32) (bool, error)
	CreateReportFunc     func(ctx context.Context, tx pgx.Tx, params sqlc.InsertReviewReportParams) error
	CountReportsFunc     func(ctx context.Context, tx pgx.Tx, reviewID int32) (int64, error)
	GetReportsByReviewIDFunc func(ctx context.Context, reviewID int32) ([]sqlc.GetReviewReportsByReviewIDRow, error)
	DeleteReportsByReviewIDFunc func(ctx context.Context, tx pgx.Tx, reviewID int32) error
	HasHelpfulVoteFunc   func(ctx context.Context, userID, reviewID int32) (bool, error)
	AddHelpfulVoteFunc   func(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error)
	RemoveHelpfulVoteFunc func(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error)
	AdminGetAllFunc      func(ctx context.Context, status *int16, rating *int32, productUUID, search *string, orderBy, sort string, limit, offset int32) ([]v1dto.AdminReviewRow, error)
	AdminCountFunc       func(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error)
	UpdateStatusFunc     func(ctx context.Context, tx pgx.Tx, reviewID int32, status int16) (sqlc.ProductReview, error)
	ReplyFunc            func(ctx context.Context, tx pgx.Tx, reviewID int32, reply string) (sqlc.ProductReview, error)
}

func (m *MockReviewRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateReviewParams) (sqlc.ProductReview, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, params)
	}
	return sqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) CheckPurchased(ctx context.Context, orderID, userID, productID int32) (bool, error) {
	if m.CheckPurchasedFunc != nil {
		return m.CheckPurchasedFunc(ctx, orderID, userID, productID)
	}
	return false, nil
}

func (m *MockReviewRepository) CheckExists(ctx context.Context, userID, productID, orderID int32) (bool, error) {
	if m.CheckExistsFunc != nil {
		return m.CheckExistsFunc(ctx, userID, productID, orderID)
	}
	return false, nil
}

func (m *MockReviewRepository) RecalcRating(ctx context.Context, tx pgx.Tx, productID int32) error {
	if m.RecalcRatingFunc != nil {
		return m.RecalcRatingFunc(ctx, tx, productID)
	}
	return nil
}

func (m *MockReviewRepository) GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]v1dto.ReviewRow, error) {
	if m.GetByProductFunc != nil {
		return m.GetByProductFunc(ctx, productID, rating, hasMedia, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error) {
	if m.CountByProductFunc != nil {
		return m.CountByProductFunc(ctx, productID, rating, hasMedia)
	}
	return 0, nil
}

func (m *MockReviewRepository) GetRatingSummary(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error) {
	if m.GetRatingSummaryFunc != nil {
		return m.GetRatingSummaryFunc(ctx, productID)
	}
	return sqlc.GetProductRatingSummaryRow{}, nil
}

func (m *MockReviewRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	if m.CountByUserIDFunc != nil {
		return m.CountByUserIDFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockReviewRepository) GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, reviewUUID)
	}
	return sqlc.GetReviewByUUIDRow{}, nil
}

func (m *MockReviewRepository) Update(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReviewParams) (sqlc.ProductReview, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, tx, params)
	}
	return sqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) Delete(ctx context.Context, tx pgx.Tx, reviewID int32) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, tx, reviewID)
	}
	return nil
}

func (m *MockReviewRepository) HasReported(ctx context.Context, userID, reviewID int32) (bool, error) {
	if m.HasReportedFunc != nil {
		return m.HasReportedFunc(ctx, userID, reviewID)
	}
	return false, nil
}

func (m *MockReviewRepository) CreateReport(ctx context.Context, tx pgx.Tx, params sqlc.InsertReviewReportParams) error {
	if m.CreateReportFunc != nil {
		return m.CreateReportFunc(ctx, tx, params)
	}
	return nil
}

func (m *MockReviewRepository) CountReports(ctx context.Context, tx pgx.Tx, reviewID int32) (int64, error) {
	if m.CountReportsFunc != nil {
		return m.CountReportsFunc(ctx, tx, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) HasHelpfulVote(ctx context.Context, userID, reviewID int32) (bool, error) {
	if m.HasHelpfulVoteFunc != nil {
		return m.HasHelpfulVoteFunc(ctx, userID, reviewID)
	}
	return false, nil
}

func (m *MockReviewRepository) AddHelpfulVote(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error) {
	if m.AddHelpfulVoteFunc != nil {
		return m.AddHelpfulVoteFunc(ctx, tx, userID, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) RemoveHelpfulVote(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error) {
	if m.RemoveHelpfulVoteFunc != nil {
		return m.RemoveHelpfulVoteFunc(ctx, tx, userID, reviewID)
	}
	return 0, nil
}

func (m *MockReviewRepository) AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID, search *string, orderBy, sort string, limit, offset int32) ([]v1dto.AdminReviewRow, error) {
	if m.AdminGetAllFunc != nil {
		return m.AdminGetAllFunc(ctx, status, rating, productUUID, search, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockReviewRepository) AdminCount(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error) {
	if m.AdminCountFunc != nil {
		return m.AdminCountFunc(ctx, status, rating, productID, search)
	}
	return 0, nil
}

func (m *MockReviewRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, reviewID int32, status int16) (sqlc.ProductReview, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, tx, reviewID, status)
	}
	return sqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) Reply(ctx context.Context, tx pgx.Tx, reviewID int32, reply string) (sqlc.ProductReview, error) {
	if m.ReplyFunc != nil {
		return m.ReplyFunc(ctx, tx, reviewID, reply)
	}
	return sqlc.ProductReview{}, nil
}

func (m *MockReviewRepository) GetReportsByReviewID(ctx context.Context, reviewID int32) ([]sqlc.GetReviewReportsByReviewIDRow, error) {
	if m.GetReportsByReviewIDFunc != nil {
		return m.GetReportsByReviewIDFunc(ctx, reviewID)
	}
	return nil, nil
}

func (m *MockReviewRepository) DeleteReportsByReviewID(ctx context.Context, tx pgx.Tx, reviewID int32) error {
	if m.DeleteReportsByReviewIDFunc != nil {
		return m.DeleteReportsByReviewIDFunc(ctx, tx, reviewID)
	}
	return nil
}

// =========================================================================
// 12. MockBannerRepository
// =========================================================================

type MockBannerRepository struct {
	GetActiveByPositionFunc func(ctx context.Context, position string) ([]sqlc.GetActiveBannersByPositionRow, error)
	CreateFunc              func(ctx context.Context, params sqlc.CreateBannerParams) (sqlc.Banner, error)
	GetByUUIDFunc           func(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	AdminListFunc           func(ctx context.Context, params sqlc.AdminListBannersParams) ([]sqlc.Banner, error)
	AdminCountFunc          func(ctx context.Context, params sqlc.AdminCountBannersParams) (int64, error)
	UpdateFunc              func(ctx context.Context, params sqlc.UpdateBannerParams) (sqlc.Banner, error)
	UpdateStatusFunc        func(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (sqlc.Banner, error)
	SoftDeleteFunc          func(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	RestoreFunc             func(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	TrashFunc               func(ctx context.Context, bannerUUID uuid.UUID) error
}

func (m *MockBannerRepository) GetActiveByPosition(ctx context.Context, position string) ([]sqlc.GetActiveBannersByPositionRow, error) {
	if m.GetActiveByPositionFunc != nil {
		return m.GetActiveByPositionFunc(ctx, position)
	}
	return nil, nil
}

func (m *MockBannerRepository) Create(ctx context.Context, params sqlc.CreateBannerParams) (sqlc.Banner, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) GetByUUID(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, bannerUUID)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) AdminList(ctx context.Context, params sqlc.AdminListBannersParams) ([]sqlc.Banner, error) {
	if m.AdminListFunc != nil {
		return m.AdminListFunc(ctx, params)
	}
	return nil, nil
}

func (m *MockBannerRepository) AdminCount(ctx context.Context, params sqlc.AdminCountBannersParams) (int64, error) {
	if m.AdminCountFunc != nil {
		return m.AdminCountFunc(ctx, params)
	}
	return 0, nil
}

func (m *MockBannerRepository) Update(ctx context.Context, params sqlc.UpdateBannerParams) (sqlc.Banner, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) UpdateStatus(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (sqlc.Banner, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, bannerUUID, isActive)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) SoftDelete(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, bannerUUID)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) Restore(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, bannerUUID)
	}
	return sqlc.Banner{}, nil
}

func (m *MockBannerRepository) Trash(ctx context.Context, bannerUUID uuid.UUID) error {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, bannerUUID)
	}
	return nil
}

// =========================================================================
// 13. MockAddressRepository
// =========================================================================

type MockAddressRepository struct {
	CreateFunc             func(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error)
	CountFunc              func(ctx context.Context, userID int32) (int64, error)
	ClearDefaultFunc       func(ctx context.Context, tx pgx.Tx, userID int32) error
	UpdateFunc             func(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error)
	SetDefaultFunc         func(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	DeleteFunc             func(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (bool, error)
	SetLatestAsDefaultFunc func(ctx context.Context, tx pgx.Tx, userID int32) error
	GetByUserIDFunc        func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error)
	GetByUUIDFunc          func(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
}

func (m *MockAddressRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, params)
	}
	return sqlc.UserAddress{}, nil
}

func (m *MockAddressRepository) Count(ctx context.Context, userID int32) (int64, error) {
	if m.CountFunc != nil {
		return m.CountFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockAddressRepository) ClearDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	if m.ClearDefaultFunc != nil {
		return m.ClearDefaultFunc(ctx, tx, userID)
	}
	return nil
}

func (m *MockAddressRepository) Update(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return sqlc.UserAddress{}, nil
}

func (m *MockAddressRepository) SetDefault(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
	if m.SetDefaultFunc != nil {
		return m.SetDefaultFunc(ctx, tx, userID, addressUUID)
	}
	return sqlc.UserAddress{}, nil
}

func (m *MockAddressRepository) Delete(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (bool, error) {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, tx, userID, addressUUID)
	}
	return false, nil
}

func (m *MockAddressRepository) SetLatestAsDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	if m.SetLatestAsDefaultFunc != nil {
		return m.SetLatestAsDefaultFunc(ctx, tx, userID)
	}
	return nil
}

func (m *MockAddressRepository) GetByUserID(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockAddressRepository) GetByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, userID, addressUUID)
	}
	return sqlc.UserAddress{}, nil
}

// =========================================================================
// MockNotificationRepository
// =========================================================================

type MockNotificationRepository struct {
	CreateFunc            func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error)
	CreateForAllUsersFunc func(ctx context.Context, nType, title, body string, data []byte) error
	GetByUserIDFunc       func(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error)
	CountByUserIDFunc     func(ctx context.Context, userID int32) (int64, error)
	CountUnreadByUserIDFunc func(ctx context.Context, userID int32) (int64, error)
	MarkAsReadFunc        func(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error)
	MarkAllAsReadFunc     func(ctx context.Context, userID int32) error
	DeleteFunc            func(ctx context.Context, notifUUID uuid.UUID, userID int32) error
}

func (m *MockNotificationRepository) Create(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return sqlc.Notification{}, nil
}

func (m *MockNotificationRepository) CreateForAllUsers(ctx context.Context, nType, title, body string, data []byte) error {
	if m.CreateForAllUsersFunc != nil {
		return m.CreateForAllUsersFunc(ctx, nType, title, body, data)
	}
	return nil
}

func (m *MockNotificationRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *MockNotificationRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	if m.CountByUserIDFunc != nil {
		return m.CountByUserIDFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockNotificationRepository) CountUnreadByUserID(ctx context.Context, userID int32) (int64, error) {
	if m.CountUnreadByUserIDFunc != nil {
		return m.CountUnreadByUserIDFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error) {
	if m.MarkAsReadFunc != nil {
		return m.MarkAsReadFunc(ctx, notifUUID, userID)
	}
	return sqlc.Notification{}, nil
}

func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID int32) error {
	if m.MarkAllAsReadFunc != nil {
		return m.MarkAllAsReadFunc(ctx, userID)
	}
	return nil
}

func (m *MockNotificationRepository) Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, notifUUID, userID)
	}
	return nil
}

type MockAnalyticsRepository struct {
	GetOverviewFunc                 func(ctx context.Context) (sqlc.GetOverviewStatsRow, error)
	GetOverviewCustomFunc           func(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error)
	GetRevenueDailyFunc             func(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error)
	GetRevenueDailyCustomFunc       func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error)
	GetRevenueMonthlyFunc           func(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error)
	GetRecentOrdersFunc             func(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error)
	GetTopSellingProductsFunc       func(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error)
	GetTopSellingProductsCustomFunc func(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error)
	GetOrderStatusDistributionFunc func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error)
	GetLowStockProductsFunc         func(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error)
	GetPaymentMethodBreakdownFunc   func(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error)
}

func (m *MockAnalyticsRepository) GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error) {
	if m.GetOverviewFunc != nil {
		return m.GetOverviewFunc(ctx)
	}
	return sqlc.GetOverviewStatsRow{}, nil
}

func (m *MockAnalyticsRepository) GetOverviewCustom(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error) {
	if m.GetOverviewCustomFunc != nil {
		return m.GetOverviewCustomFunc(ctx, fromDate, toDate, prevFromDate)
	}
	return sqlc.GetOverviewStatsCustomRow{}, nil
}

func (m *MockAnalyticsRepository) GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error) {
	if m.GetRevenueDailyFunc != nil {
		return m.GetRevenueDailyFunc(ctx, days)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetRevenueDailyCustom(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error) {
	if m.GetRevenueDailyCustomFunc != nil {
		return m.GetRevenueDailyCustomFunc(ctx, fromDate, toDate)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetRevenueMonthly(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error) {
	if m.GetRevenueMonthlyFunc != nil {
		return m.GetRevenueMonthlyFunc(ctx, months)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetRecentOrders(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error) {
	if m.GetRecentOrdersFunc != nil {
		return m.GetRecentOrdersFunc(ctx, limit)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetTopSellingProducts(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error) {
	if m.GetTopSellingProductsFunc != nil {
		return m.GetTopSellingProductsFunc(ctx, days, limit)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetTopSellingProductsCustom(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error) {
	if m.GetTopSellingProductsCustomFunc != nil {
		return m.GetTopSellingProductsCustomFunc(ctx, fromDate, toDate, limit)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error) {
	if m.GetOrderStatusDistributionFunc != nil {
		return m.GetOrderStatusDistributionFunc(ctx, fromDate, toDate)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetLowStockProducts(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error) {
	if m.GetLowStockProductsFunc != nil {
		return m.GetLowStockProductsFunc(ctx, threshold, limit)
	}
	return nil, nil
}

func (m *MockAnalyticsRepository) GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error) {
	if m.GetPaymentMethodBreakdownFunc != nil {
		return m.GetPaymentMethodBreakdownFunc(ctx, fromDate, toDate)
	}
	return nil, nil
}




