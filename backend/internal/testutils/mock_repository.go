package testutils

import (
	"context"
	"time"

	"goshop/internal/db/sqlc"
	"goshop/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Ensure all mocks implement repository interfaces at compile time
var (
	_ repository.ProductRepository      = (*MockProductRepository)(nil)
	_ repository.UserRepository         = (*MockUserRepository)(nil)
	_ repository.UserReader             = (*MockUserRepository)(nil)
	_ repository.AddressRepository      = (*MockAddressRepository)(nil)
	_ repository.NotificationRepository = (*MockNotificationRepository)(nil)
	_ repository.OutboxRepository       = (*MockOutboxRepository)(nil)
)

// =========================================================================
// 3. MockProductRepository
// =========================================================================
type MockProductRepository struct {
	CreateFunc        func(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error)
	CountProductsFunc func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, deleted bool) (int64, error)
	GetAllFunc        func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error)
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

func (m *MockProductRepository) CountProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, deleted bool) (int64, error) {
	if m.CountProductsFunc != nil {
		return m.CountProductsFunc(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, deleted)
	}
	return 0, nil
}

func (m *MockProductRepository) GetAll(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, limit, offset, deleted)
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
// 7. MockUserRepository
// =========================================================================
type MockUserRepository struct {
	GetAllFunc                func(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2Func              func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsersFunc            func(ctx context.Context, search string, deleted bool) (int64, error)
	CreateFunc                func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetByIDFunc               func(ctx context.Context, id int32) (sqlc.User, error)
	GetByUUIDFunc             func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	UpdateFunc                func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDeleteFunc            func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	RestoreFunc               func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	TrashFunc                 func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmailFunc            func(ctx context.Context, email string) (sqlc.User, error)
	GetActiveAdminUserIDsFunc func(ctx context.Context) ([]int32, error)
	UpdatePasswordFunc        func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	GetByOAuthFunc            func(ctx context.Context, provider, oauthID string) (sqlc.User, error)
	CreateOAuthUserFunc       func(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error)
	GetByIDsFunc              func(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error)
}

func (m *MockUserRepository) GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
	if m.GetByIDsFunc != nil {
		return m.GetByIDsFunc(ctx, ids)
	}
	return []sqlc.GetUsersByIDsRow{}, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return sqlc.User{}, nil
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

func (m *MockUserRepository) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if m.GetActiveAdminUserIDsFunc != nil {
		return m.GetActiveAdminUserIDsFunc(ctx)
	}
	return nil, nil
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error) {
	if m.GetByOAuthFunc != nil {
		return m.GetByOAuthFunc(ctx, provider, oauthID)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error) {
	if m.CreateOAuthUserFunc != nil {
		return m.CreateOAuthUserFunc(ctx, params)
	}
	return sqlc.User{}, nil
}

// =========================================================================
// =========================================================================

type MockAddressRepository struct {
	BeginTxFunc            func(ctx context.Context) (pgx.Tx, error)
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

func (m *MockAddressRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if m.BeginTxFunc != nil {
		return m.BeginTxFunc(ctx)
	}
	return nil, nil
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
	CreateFunc                        func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error)
	CreateForAllUsersFunc             func(ctx context.Context, nType, title, body string, data []byte) error
	GetByUserIDFunc                   func(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error)
	CountByUserIDFunc                 func(ctx context.Context, userID int32) (int64, error)
	CountUnreadByUserIDFunc           func(ctx context.Context, userID int32) (int64, error)
	CountAdminUnreadNotificationsFunc func(ctx context.Context, userID int32) (int64, error)
	MarkAsReadFunc                    func(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error)
	MarkAllAsReadFunc                 func(ctx context.Context, userID int32) error
	MarkAllAdminAsReadFunc            func(ctx context.Context, userID int32) error
	DeleteFunc                        func(ctx context.Context, notifUUID uuid.UUID, userID int32) error
	CreateForAdminsFunc               func(ctx context.Context, nType, title, body string, data []byte) error
	GetAdminNotificationsFunc         func(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error)
	CountAdminNotificationsFunc       func(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error)
	DeleteOldReadNotificationsFunc    func(ctx context.Context, cutoff time.Time) (int64, error)
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

func (m *MockNotificationRepository) CountAdminUnreadNotifications(ctx context.Context, userID int32) (int64, error) {
	if m.CountAdminUnreadNotificationsFunc != nil {
		return m.CountAdminUnreadNotificationsFunc(ctx, userID)
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

func (m *MockNotificationRepository) MarkAllAdminAsRead(ctx context.Context, userID int32) error {
	if m.MarkAllAdminAsReadFunc != nil {
		return m.MarkAllAdminAsReadFunc(ctx, userID)
	}
	return nil
}

func (m *MockNotificationRepository) Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, notifUUID, userID)
	}
	return nil
}

func (m *MockNotificationRepository) CreateForAdmins(ctx context.Context, nType, title, body string, data []byte) error {
	if m.CreateForAdminsFunc != nil {
		return m.CreateForAdminsFunc(ctx, nType, title, body, data)
	}
	return nil
}

func (m *MockNotificationRepository) GetAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error) {
	if m.GetAdminNotificationsFunc != nil {
		return m.GetAdminNotificationsFunc(ctx, userID, nType, isRead, limit, offset)
	}
	return nil, nil
}

func (m *MockNotificationRepository) CountAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error) {
	if m.CountAdminNotificationsFunc != nil {
		return m.CountAdminNotificationsFunc(ctx, userID, nType, isRead)
	}
	return 0, nil
}

func (m *MockNotificationRepository) DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error) {
	if m.DeleteOldReadNotificationsFunc != nil {
		return m.DeleteOldReadNotificationsFunc(ctx, cutoff)
	}
	return 0, nil
}

// =========================================================================
// 16. MockOutboxRepository
// =========================================================================
type MockOutboxRepository struct {
	InsertFunc                     func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error)
	GetPendingFunc                 func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error)
	GetPendingByAggregateTypesFunc func(ctx context.Context, aggregateTypes []string, limit int32) ([]sqlc.OutboxEvent, error)
	MarkProcessedFunc              func(ctx context.Context, eventIDs []int64) error
	MarkFailedFunc                 func(ctx context.Context, eventID int64, lastErr *string) error
}

func (m *MockOutboxRepository) Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, tx, aggregateType, aggregateID, eventType, payload)
	}
	return sqlc.OutboxEvent{}, nil
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
	if m.GetPendingFunc != nil {
		return m.GetPendingFunc(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) GetPendingByAggregateTypes(ctx context.Context, aggregateTypes []string, limit int32) ([]sqlc.OutboxEvent, error) {
	if m.GetPendingByAggregateTypesFunc != nil {
		return m.GetPendingByAggregateTypesFunc(ctx, aggregateTypes, limit)
	}
	if m.GetPendingFunc != nil {
		return m.GetPendingFunc(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) MarkProcessed(ctx context.Context, eventIDs []int64) error {
	if m.MarkProcessedFunc != nil {
		return m.MarkProcessedFunc(ctx, eventIDs)
	}
	return nil
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, eventID int64, lastErr *string) error {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(ctx, eventID, lastErr)
	}
	return nil
}
