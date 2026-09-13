package testutils

import (
	"context"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Ensure all mocks implement repository interfaces at compile time
var (
	_ repository.OrderRepository     = (*MockOrderRepository)(nil)
	_ repository.OrderItemRepository = (*MockOrderItemRepository)(nil)
	_ repository.ProductRepository   = (*MockProductRepository)(nil)
	_ repository.CouponRepository    = (*MockCouponRepository)(nil)
	_ repository.CartRepository      = (*MockCartRepository)(nil)
	_ repository.TrackingRepository  = (*MockTrackingRepository)(nil)
	_ repository.UserRepository      = (*MockUserRepository)(nil)
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
	UpdateStatusFunc                func(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
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

func (m *MockOrderRepository) UpdateStatus(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, params)
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
