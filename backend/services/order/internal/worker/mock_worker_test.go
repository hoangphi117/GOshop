package worker

import (
	"context"
	"time"
	v1dto "goshop/services/order/internal/dto"
	"goshop/services/order/internal/db/sqlc"
	"goshop/services/order/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	orderservice "goshop/services/order/internal/service"
)

var (
	_ repository.OrderRepository = (*MockOrderRepository)(nil)
	_ repository.OrderItemRepository = (*MockOrderItemRepository)(nil)
	_ repository.TrackingRepository = (*MockTrackingRepository)(nil)
	_ repository.OrderReturnsRepository = (*MockOrderReturnsRepository)(nil)
	_ repository.SagaRepository = (*MockSagaRepository)(nil)
)

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
// 10. MockCategoryRepository
// =========================================================================
type MockSagaRepository struct {
	StartFunc                func(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error)
	ConfirmFunc              func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error)
	ClaimForCompensationFunc func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	MarkFailedFunc           func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	ScheduleRetryFunc        func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error)
	ClaimStaleFunc           func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error)
	UpdateStepFunc           func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error)
	GetByOrderUUIDFunc       func(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error)
	GetByIdempotencyKeyFunc  func(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (sqlc.OrderSaga, error)
}

func (m *MockSagaRepository) Start(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
	if m.StartFunc != nil {
		return m.StartFunc(ctx, tx, params)
	}
	return sqlc.OrderSaga{}, nil
}

func (m *MockSagaRepository) Confirm(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
	if m.ConfirmFunc != nil {
		return m.ConfirmFunc(ctx, tx, orderUUID)
	}
	return true, nil
}

func (m *MockSagaRepository) ClaimForCompensation(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	if m.ClaimForCompensationFunc != nil {
		return m.ClaimForCompensationFunc(ctx, tx, orderUUID, failureReason)
	}
	return true, nil
}

func (m *MockSagaRepository) MarkFailed(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(ctx, tx, orderUUID, failureReason)
	}
	return true, nil
}

func (m *MockSagaRepository) ScheduleRetry(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	if m.ScheduleRetryFunc != nil {
		return m.ScheduleRetryFunc(ctx, tx, orderUUID, failureReason)
	}
	return true, nil
}

func (m *MockSagaRepository) ClaimStale(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
	if m.ClaimStaleFunc != nil {
		return m.ClaimStaleFunc(ctx, tx, limit)
	}
	return nil, nil
}

func (m *MockSagaRepository) UpdateStep(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
	if m.UpdateStepFunc != nil {
		return m.UpdateStepFunc(ctx, tx, orderUUID, lastStep, snapshot)
	}
	return true, nil
}

func (m *MockSagaRepository) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error) {
	if m.GetByOrderUUIDFunc != nil {
		return m.GetByOrderUUIDFunc(ctx, orderUUID)
	}
	return sqlc.OrderSaga{}, nil
}

func (m *MockSagaRepository) GetByIdempotencyKey(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (sqlc.OrderSaga, error) {
	if m.GetByIdempotencyKeyFunc != nil {
		return m.GetByIdempotencyKeyFunc(ctx, userUUID, idempotencyKey)
	}
	return sqlc.OrderSaga{}, nil
}

type MockUserRepository struct {
	GetByUUIDFunc func(ctx context.Context, u uuid.UUID) (sqlc.User, error)
	GetByIDFunc   func(ctx context.Context, id int32) (sqlc.User, error)
}

func (m *MockUserRepository) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, u)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return sqlc.User{}, nil
}

type MockCustomerReader struct {
	GetByUUIDFunc func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error)
	GetByIDFunc   func(ctx context.Context, id int32) (*orderservice.OrderCustomer, error)
}

func (m *MockCustomerReader) GetByUUID(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, u)
	}
	return orderservice.NewOrderCustomer(1, u, "Test User", "test@example.com", orderservice.RoleMember, 1), nil
}

func (m *MockCustomerReader) GetByID(ctx context.Context, id int32) (*orderservice.OrderCustomer, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return orderservice.NewOrderCustomer(id, uuid.New(), "Test User", "test@example.com", orderservice.RoleMember, 1), nil
}

type MockPaymentRefundPort struct {
	RefundPaymentFunc func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
}

func (m *MockPaymentRefundPort) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if m.RefundPaymentFunc != nil {
		return m.RefundPaymentFunc(ctx, idempotencyKey, input)
	}
	return v1dto.PaymentTransactionDTO{}, nil
}

type MockOutboxRepository struct {
	InsertFunc                     func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error)
	CreateFunc                     func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload []byte) (sqlc.OutboxEvent, error)
	GetPendingFunc                 func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error)
	GetPendingByAggregateTypesFunc func(ctx context.Context, limit int32, aggregateTypes []string) ([]sqlc.OutboxEvent, error)
	MarkProcessedFunc             func(ctx context.Context, eventIDs []int64) error
	MarkFailedFunc                func(ctx context.Context, eventID int64, lastErr string) error
}

func (m *MockOutboxRepository) Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, tx, aggregateType, aggregateID, eventType, payload)
	}
	return sqlc.OutboxEvent{}, nil
}

func (m *MockOutboxRepository) Create(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload []byte) (sqlc.OutboxEvent, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, aggregateType, aggregateID, eventType, payload)
	}
	return sqlc.OutboxEvent{}, nil
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
	if m.GetPendingFunc != nil {
		return m.GetPendingFunc(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) GetPendingByAggregateTypes(ctx context.Context, limit int32, aggregateTypes []string) ([]sqlc.OutboxEvent, error) {
	if m.GetPendingByAggregateTypesFunc != nil {
		return m.GetPendingByAggregateTypesFunc(ctx, limit, aggregateTypes)
	}
	return nil, nil
}

func (m *MockOutboxRepository) MarkProcessed(ctx context.Context, eventIDs []int64) error {
	if m.MarkProcessedFunc != nil {
		return m.MarkProcessedFunc(ctx, eventIDs)
	}
	return nil
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, eventID int64, lastErr string) error {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(ctx, eventID, lastErr)
	}
	return nil
}

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










// Additional mocks for order service domain ports

type MockCatalogPort struct {
	GetProductsForCheckoutFunc func(ctx context.Context, productUUIDs []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error)
}

func (m *MockCatalogPort) GetProductsForCheckout(ctx context.Context, productUUIDs []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
	if m.GetProductsForCheckoutFunc != nil {
		return m.GetProductsForCheckoutFunc(ctx, productUUIDs)
	}
	return make(map[string]orderservice.CheckoutProductSnapshot), nil
}

type MockOrderStockPort struct {
	LockAndDeductStockWithRefFunc func(ctx context.Context, items []orderservice.DeductItem, refID string, refType string, actorID *int32, note *string) error
	RestoreStockWithRefFunc       func(ctx context.Context, items []orderservice.RestoreItem, refID string, refType string, actorID *int32, note *string) error
}

func (m *MockOrderStockPort) LockAndDeductStockWithRef(ctx context.Context, items []orderservice.DeductItem, refID string, refType string, actorID *int32, note *string) error {
	if m.LockAndDeductStockWithRefFunc != nil {
		return m.LockAndDeductStockWithRefFunc(ctx, items, refID, refType, actorID, note)
	}
	return nil
}

func (m *MockOrderStockPort) RestoreStockWithRef(ctx context.Context, items []orderservice.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
	if m.RestoreStockWithRefFunc != nil {
		return m.RestoreStockWithRefFunc(ctx, items, refID, refType, actorID, note)
	}
	return nil
}

type MockOrderCouponPort struct {
	PreviewCouponFunc      func(ctx context.Context, code string, amount float64, userUUID uuid.UUID) (*orderservice.CouponDiscountResult, error)
	LockAndApplyCouponFunc func(ctx context.Context, couponID int32, userID int32, orderID int32, orderUUID *uuid.UUID) error
	ReleaseCouponFunc      func(ctx context.Context, couponID int32, orderID int32, orderUUID *uuid.UUID) error
}

func (m *MockOrderCouponPort) PreviewCoupon(ctx context.Context, code string, amount float64, userUUID uuid.UUID) (*orderservice.CouponDiscountResult, error) {
	if m.PreviewCouponFunc != nil {
		return m.PreviewCouponFunc(ctx, code, amount, userUUID)
	}
	return nil, nil
}

func (m *MockOrderCouponPort) LockAndApplyCoupon(ctx context.Context, couponID int32, userID int32, orderID int32, orderUUID *uuid.UUID) error {
	if m.LockAndApplyCouponFunc != nil {
		return m.LockAndApplyCouponFunc(ctx, couponID, userID, orderID, orderUUID)
	}
	return nil
}

func (m *MockOrderCouponPort) ReleaseCoupon(ctx context.Context, couponID int32, orderID int32, orderUUID *uuid.UUID) error {
	if m.ReleaseCouponFunc != nil {
		return m.ReleaseCouponFunc(ctx, couponID, orderID, orderUUID)
	}
	return nil
}

type MockOrderPricingService struct {
	CalculateItemsFunc       func(items []orderservice.ItemPriceInput) ([]orderservice.ItemPriceResult, float64, error)
	CalculateOrderTotalsFunc func(ctx context.Context, items []orderservice.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*orderservice.OrderPricingResult, error)
}

func (m *MockOrderPricingService) CalculateItems(items []orderservice.ItemPriceInput) ([]orderservice.ItemPriceResult, float64, error) {
	if m.CalculateItemsFunc != nil {
		return m.CalculateItemsFunc(items)
	}
	return nil, 0, nil
}

func (m *MockOrderPricingService) CalculateOrderTotals(ctx context.Context, items []orderservice.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
	if m.CalculateOrderTotalsFunc != nil {
		return m.CalculateOrderTotalsFunc(ctx, items, couponCode, userID, shippingFee, now)
	}
	return &orderservice.OrderPricingResult{}, nil
}

type MockCartService struct {
	RemovePurchasedItemsFunc func(ctx context.Context, userID int32, productIDs []int32) error
}

func (m *MockCartService) RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error {
	if m.RemovePurchasedItemsFunc != nil {
		return m.RemovePurchasedItemsFunc(ctx, userID, productIDs)
	}
	return nil
}

type MockShippingValidatorPort struct {
	ValidateOrCalculateShippingFeeFunc func(ctx context.Context, provider string, districtID int, wardCode string, totalWeight int, insuranceValue float64, declaredFee float64) (float64, error)
}

func (m *MockShippingValidatorPort) ValidateOrCalculateShippingFee(ctx context.Context, provider string, districtID int, wardCode string, totalWeight int, insuranceValue float64, declaredFee float64) (float64, error) {
	if m.ValidateOrCalculateShippingFeeFunc != nil {
		return m.ValidateOrCalculateShippingFeeFunc(ctx, provider, districtID, wardCode, totalWeight, insuranceValue, declaredFee)
	}
	return declaredFee, nil
}

type MockOrderService struct {
	CancelExpiredOrderFunc       func(ctx context.Context, orderUUID uuid.UUID) error
	UpdateOrderPaymentStatusFunc func(ctx context.Context, userUUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error)

	UpdateOrderStatusFunc func(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error)
	CancelOrderFunc       func(ctx context.Context, userUUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error)
	GetAllOrdersFunc      func(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	orderservice.OrderService
}

func (m *MockOrderService) UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error) {
	if m.UpdateOrderStatusFunc != nil {
		return m.UpdateOrderStatusFunc(ctx, orderUUID, input)
	}
	return nil, nil
}

func (m *MockOrderService) CancelOrder(ctx context.Context, userUUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error) {
	if m.CancelOrderFunc != nil {
		return m.CancelOrderFunc(ctx, userUUID, orderUUID, reason)
	}
	return nil, nil
}

func (m *MockOrderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if m.GetAllOrdersFunc != nil {
		return m.GetAllOrdersFunc(ctx, input)
	}
	return nil, 0, nil
}



func (m *MockOrderService) CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error {
	if m.CancelExpiredOrderFunc != nil {
		return m.CancelExpiredOrderFunc(ctx, orderUUID)
	}
	return nil
}

func (m *MockOrderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
	if m.UpdateOrderPaymentStatusFunc != nil {
		return m.UpdateOrderPaymentStatusFunc(ctx, userUUID, orderUUID, input)
	}
	return nil, nil
}


type MockOrderShippingService struct {
	ProcessShippingOrderFunc func(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error
	orderservice.OrderShippingService
}

func (m *MockOrderShippingService) ProcessShippingOrder(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error {
	if m.ProcessShippingOrderFunc != nil {
		return m.ProcessShippingOrderFunc(ctx, msg)
	}
	return nil
}
