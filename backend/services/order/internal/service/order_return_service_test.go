package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/internal/testutils"
	"goshop/pkg/db"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"goshop/services/order/internal/repository"
	orderservice "goshop/services/order/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReturnServiceDeps struct {
	txManager       db.TxManager
	orderRepo       *MockOrderRepository
	orderItemRepo   *MockOrderItemRepository
	orderReturnRepo *MockOrderReturnsRepository
	stockPort       orderservice.OrderStockPort
	userRepo        *MockUserRepository
	trackingRepo    *MockTrackingRepository
	paymentService  orderservice.OrderPaymentRefundPort
	outboxStore     orderservice.OrderOutboxPort
}

func newTestReturnService(deps testReturnServiceDeps) orderservice.OrderReturnService {
	txMgr := deps.txManager
	if txMgr == nil {
		txMgr = testutils.NewNoopTxManager()
	}
	var orderRepo repository.OrderRepository
	if deps.orderRepo != nil {
		orderRepo = deps.orderRepo
	} else {
		orderRepo = &MockOrderRepository{}
	}
	var orderItemRepo repository.OrderItemRepository
	if deps.orderItemRepo != nil {
		orderItemRepo = deps.orderItemRepo
	} else {
		orderItemRepo = &MockOrderItemRepository{}
	}
	var returnRepo repository.OrderReturnsRepository
	if deps.orderReturnRepo != nil {
		returnRepo = deps.orderReturnRepo
	} else {
		returnRepo = &MockOrderReturnsRepository{}
	}
	var stockPort orderservice.OrderStockPort
	if deps.stockPort != nil {
		stockPort = deps.stockPort
	} else {
		stockPort = &MockOrderStockPort{}
	}
	var trackingRepo repository.TrackingRepository
	if deps.trackingRepo != nil {
		trackingRepo = deps.trackingRepo
	} else {
		trackingRepo = &MockTrackingRepository{}
	}
	var paymentSvc orderservice.OrderPaymentRefundPort
	if deps.paymentService != nil {
		paymentSvc = deps.paymentService
	} else {
		paymentSvc = &MockPaymentRefundPort{}
	}
	var outboxStore orderservice.OrderOutboxPort
	if deps.outboxStore != nil {
		outboxStore = deps.outboxStore
	} else {
		outboxStore = &MockOutboxRepository{}
	}

	var customerReader orderservice.OrderCustomerReader
	if deps.userRepo != nil {
		customerReader = orderservice.NewOrderCustomerAdapter(deps.userRepo)
	} else {
		customerReader = orderservice.NewOrderCustomerAdapter(&MockUserRepository{})
	}

	return orderservice.NewOrderReturnService(orderservice.OrderReturnServiceDeps{
		TxManager:       txMgr,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		OrderReturnRepo: returnRepo,
		StockPort:       stockPort,
		UserRepo:        customerReader,
		TrackingRepo:    trackingRepo,
		PaymentService:  paymentSvc,
		OutboxStore:     outboxStore,
	})
}

func TestOrderService_RequestOrderReturn(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	now := time.Now()
	deliveredRecently := now.Add(-24 * time.Hour)
	deliveredLongAgo := now.Add(-10 * 24 * time.Hour)

	input := v1dto.CreateOrderReturnInput{
		Reason:          "Damaged in transit",
		BankName:        "Vietcombank",
		BankAccount:     "1234567890",
		BankAccountName: "Nguyen Van A",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Order Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:  mockUserRepo,
			orderRepo: mockOrderRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden - Different User - Returns 403", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 999, // Different owner
					OrderUuid:   orderUUID,
					OrderStatus: orderservice.OrderStatusDelivered,
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:  mockUserRepo,
			orderRepo: mockOrderRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: Invalid Order Status - Not Delivered - Returns 400", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: orderservice.OrderStatusShipped, // Not delivered
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:  mockUserRepo,
			orderRepo: mockOrderRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "delivered")
	})

	t.Run("Case 5: Return Window Expired (>7 days) - Returns 400", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: orderservice.OrderStatusDelivered,
					OrderDeliveredAt: pgtype.Timestamptz{
						Time:  deliveredLongAgo,
						Valid: true,
					},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:  mockUserRepo,
			orderRepo: mockOrderRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Case 6: Active Return Already Exists - Returns 409 Conflict", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: orderservice.OrderStatusDelivered,
					OrderDeliveredAt: pgtype.Timestamptz{
						Time:  deliveredRecently,
						Valid: true,
					},
				}, nil
			},
		}
		mockReturnRepo := &MockOrderReturnsRepository{
			GetActiveByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{ReturnID: 55}, nil
			},
		}
		mockOrderItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemProductID: 100, OrderItemQuantity: 1},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			orderItemRepo:   mockOrderItemRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 7: Happy Path - Creates Return, Tracking Event, and Sends Notification", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
					OrderStatus: orderservice.OrderStatusDelivered,
					OrderDeliveredAt: pgtype.Timestamptz{
						Time:  deliveredRecently,
						Valid: true,
					},
				}, nil
			},
		}

		returnUUID := uuid.New()
		var returnCreated bool
		mockReturnRepo := &MockOrderReturnsRepository{
			GetActiveByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error) {
				returnCreated = true
				assert.Equal(t, int32(10), params.ReturnOrderID)
				assert.Equal(t, int32(1), params.ReturnUserID)
				assert.Equal(t, "Damaged in transit", params.ReturnReason)
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnOrderID:   params.ReturnOrderID,
					ReturnUserID:    params.ReturnUserID,
					ReturnReason:    params.ReturnReason,
					ReturnStatus:    orderservice.OrderReturnStatusPending,
					ReturnCreatedAt: time.Now(),
					ReturnUpdatedAt: time.Now(),
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, int32(10), params.EventOrderID)
				assert.Equal(t, "return_requested", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var outboxInserted bool
		mockOutbox := &MockOutboxRepository{
			InsertFunc: func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
				outboxInserted = true
				assert.Equal(t, "order", aggregateType)
				assert.Equal(t, rabbitmq.QueueOrderReturnRequested, eventType)
				return sqlc.OutboxEvent{}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			trackingRepo:    mockTrackingRepo,
			outboxStore:     mockOutbox,
		})

		res, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, []string{"img1.png"}, nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.True(t, returnCreated)
		assert.True(t, trackingInserted)
		assert.True(t, outboxInserted)
		assert.Equal(t, returnUUID.String(), res.ReturnUUID)
		assert.Equal(t, orderUUID.String(), res.OrderUUID)
		assert.Equal(t, "Pending", res.Status)
	})
}

func TestOrderService_GetOrderReturnDetails(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()
	returnUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.GetOrderReturnDetails(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Return Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
				}, nil
			},
		}
		mockReturnRepo := &MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{}, pgx.ErrNoRows
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
		})

		_, err := svc.GetOrderReturnDetails(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path - Member Fetches Return Details", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}
		mockReturnRepo := &MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnOrderID:   10,
					ReturnUserID:    1,
					ReturnReason:    "Defective screen",
					ReturnStatus:    orderservice.OrderReturnStatusApproved,
					ReturnImages:    []string{"screen.jpg"},
					ReturnCreatedAt: time.Now(),
					ReturnUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
		})

		res, err := svc.GetOrderReturnDetails(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		assert.Equal(t, returnUUID.String(), res.ReturnUUID)
		assert.Equal(t, orderUUID.String(), res.OrderUUID)
		assert.Equal(t, "ORD-2026-001", res.OrderNumber)
		assert.Equal(t, "Defective screen", res.Reason)
		assert.Equal(t, int16(orderservice.OrderReturnStatusApproved), res.Status)
		assert.Equal(t, "Approved", res.StatusName)
	})
}

func TestOrderService_GetMyOrderReturns(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	returnUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo: mockUserRepo,
		})

		_, _, err := svc.GetMyOrderReturns(ctx, userUUID, v1dto.GetMyOrderReturnsQuery{Page: 1, Limit: 10})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - List Returns with Pagination", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockReturnRepo := &MockOrderReturnsRepository{
			CountReturnsByUserIDFunc: func(ctx context.Context, params sqlc.CountReturnsByUserIDParams) (int64, error) {
				return 1, nil
			},
			GetReturnsByUserIDFunc: func(ctx context.Context, params sqlc.GetReturnsByUserIDParams) ([]sqlc.GetReturnsByUserIDRow, error) {
				return []sqlc.GetReturnsByUserIDRow{
					{
						ReturnID:         1,
						ReturnUuid:       returnUUID,
						OrderUuid:        orderUUID,
						OrderNumber:      "ORD-2026-001",
						OrderTotalAmount: utils.FloatToNumeric(500000),
						ReturnReason:     "Broken seal",
						ReturnStatus:     orderservice.OrderReturnStatusPending,
						ReturnImages:     []string{},
						ReturnCreatedAt:  time.Now(),
					},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderReturnRepo: mockReturnRepo,
		})

		items, total, err := svc.GetMyOrderReturns(ctx, userUUID, v1dto.GetMyOrderReturnsQuery{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
		assert.Equal(t, returnUUID.String(), items[0].ReturnUUID)
		assert.Equal(t, orderUUID.String(), items[0].OrderUUID)
	})
}

func TestOrderService_AdminGetReturns(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	returnUUID := uuid.New()
	orderUUID := uuid.New()

	adminUser := sqlc.User{
		UserID:    1,
		UserUuid:  adminUUID,
		UserLevel: orderservice.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Unauthorized/Forbidden for Non-Staff", func(t *testing.T) {
		memberUser := sqlc.User{
			UserID:    2,
			UserUuid:  adminUUID,
			UserLevel: orderservice.RoleMember,
		}
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return memberUser, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo: mockUserRepo,
		})

		_, _, err := svc.AdminGetReturns(ctx, adminUUID, v1dto.AdminGetReturnsQuery{Page: 1, Limit: 10})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 2: Happy Path - Admin List Returns", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}
		shippingProvider := "GHN"
		shippingCode := "GHN123456"
		shippingNote := "Fragile"
		mockReturnRepo := &MockOrderReturnsRepository{
			CountReturnsFunc: func(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error) {
				return 1, nil
			},
			GetReturnsFunc: func(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error) {
				return []sqlc.AdminGetReturnsRow{
					{
						ReturnID:               1,
						ReturnUuid:             returnUUID,
						OrderUuid:              orderUUID,
						OrderNumber:            "ORD-2026-001",
						OrderTotalAmount:       utils.FloatToNumeric(750000),
						OrderShippingName:      "Customer A",
						UserFullname:           "Customer A",
						UserEmail:              "customer@example.com",
						ReturnReason:           "Wrong item received",
						ReturnStatus:           orderservice.OrderReturnStatusApproved,
						ReturnShippingProvider: &shippingProvider,
						ReturnShippingCode:     &shippingCode,
						ReturnShippingNote:     &shippingNote,
						ReturnCreatedAt:        time.Now(),
					},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderReturnRepo: mockReturnRepo,
		})

		items, total, err := svc.AdminGetReturns(ctx, adminUUID, v1dto.AdminGetReturnsQuery{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
		assert.Equal(t, returnUUID.String(), items[0].ReturnUUID)
		assert.Equal(t, "Customer A", items[0].CustomerName)
		assert.Equal(t, "Approved", items[0].StatusName)
		require.NotNil(t, items[0].ShippingProvider)
		assert.Equal(t, "GHN", *items[0].ShippingProvider)
		require.NotNil(t, items[0].ShippingCode)
		assert.Equal(t, "GHN123456", *items[0].ShippingCode)
		require.NotNil(t, items[0].ShippingNote)
		assert.Equal(t, "Fragile", *items[0].ShippingNote)
	})
}

func TestOrderService_AdminGetReturnDetail(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	returnUUID := uuid.New()
	orderUUID := uuid.New()

	adminUser := sqlc.User{
		UserID:    1,
		UserUuid:  adminUUID,
		UserLevel: orderservice.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Admin Gets Return Detail by Return UUID", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			AdminGetReturnByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error) {
				return sqlc.AdminGetReturnByUUIDRow{
					ReturnID:         1,
					ReturnUuid:       returnUUID,
					OrderID:          10,
					OrderUuid:        orderUUID,
					OrderNumber:      "ORD-2026-001",
					OrderTotalAmount: utils.FloatToNumeric(500000),
					UserFullname:     "John Doe",
					UserEmail:        "john@example.com",
					ReturnReason:     "Defective on arrival",
					ReturnStatus:     orderservice.OrderReturnStatusPending,
					ReturnImages:     []string{"img.jpg"},
					ReturnCreatedAt:  time.Now(),
					ReturnUpdatedAt:  time.Now(),
				}, nil
			},
		}

		mockOrderItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemID:          1,
						OrderItemUuid:        uuid.New(),
						ProductUuid:          uuid.New(),
						OrderItemProductName: "Item 1",
						ProductSlug:          "item-1",
						OrderItemPrice:       utils.FloatToNumeric(250000),
						OrderItemQuantity:    2,
					},
				}, nil
			},
		}

		mockTrackingRepo := &MockTrackingRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error) {
				return []sqlc.OrderTrackingEvent{
					{EventID: 1, EventStatus: "return_requested"},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderReturnRepo: mockReturnRepo,
			orderItemRepo:   mockOrderItemRepo,
			trackingRepo:    mockTrackingRepo,
		})

		res, err := svc.AdminGetReturnDetail(ctx, adminUUID, returnUUID)
		require.NoError(t, err)
		assert.Equal(t, returnUUID.String(), res.ReturnUUID)
		assert.Equal(t, orderUUID.String(), res.Order.OrderUUID)
		assert.Len(t, res.Items, 1)
		assert.Len(t, res.Events, 1)
	})

	t.Run("Case 2: Success Fallback by Order UUID", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			AdminGetReturnByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error) {
				return sqlc.AdminGetReturnByUUIDRow{}, pgx.ErrNoRows
			},
			AdminGetLatestReturnByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.AdminGetLatestReturnByOrderIDRow, error) {
				return sqlc.AdminGetLatestReturnByOrderIDRow{
					ReturnID:         1,
					ReturnUuid:       returnUUID,
					OrderID:          10,
					OrderUuid:        orderUUID,
					OrderNumber:      "ORD-2026-001",
					OrderTotalAmount: utils.FloatToNumeric(500000),
					ReturnReason:     "Wrong item",
					ReturnStatus:     orderservice.OrderReturnStatusPending,
				}, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:   10,
					OrderUuid: orderUUID,
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
		})

		res, err := svc.AdminGetReturnDetail(ctx, adminUUID, orderUUID)
		require.NoError(t, err)
		assert.Equal(t, returnUUID.String(), res.ReturnUUID)
	})
}

func TestOrderService_AdminUpdateReturnStatus(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	returnUUID := uuid.New()
	orderUUID := uuid.New()

	adminUser := sqlc.User{
		UserID:    1,
		UserUuid:  adminUUID,
		UserLevel: orderservice.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Approve Return", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusPending,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnStatus:    orderservice.OrderReturnStatusApproved,
					ReturnAdminNote: params.AdminNote,
				}, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:     10,
					OrderUserID: 2,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "return_approved", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var outboxInserted bool
		mockOutbox := &MockOutboxRepository{
			InsertFunc: func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
				outboxInserted = true
				assert.Equal(t, "order", aggregateType)
				assert.Contains(t, []string{
					rabbitmq.QueueOrderReturnUpdated,
					rabbitmq.QueueOrderStatusChanged,
				}, eventType)
				return sqlc.OutboxEvent{}, nil
			},
		}

		note := "Return accepted"
		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			trackingRepo:    mockTrackingRepo,
			outboxStore:     mockOutbox,
		})

		res, err := svc.AdminUpdateReturnStatus(ctx, adminUUID, returnUUID, v1dto.AdminUpdateReturnInput{
			Status:    orderservice.OrderReturnStatusApproved,
			AdminNote: &note,
		})
		require.NoError(t, err)
		assert.True(t, trackingInserted)
		assert.True(t, outboxInserted)
		assert.Equal(t, orderservice.OrderReturnStatusApproved, res.Status)
	})

	t.Run("Case 2: Happy Path - Complete Return Restores Stock", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusApproved,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnStatus:    orderservice.OrderReturnStatusCompleted,
					ReturnAdminNote: params.AdminNote,
				}, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:     10,
					OrderUserID: 2,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
				assert.Equal(t, orderservice.OrderStatusReturned, params.OrderStatus)
				return sqlc.Order{OrderID: 10, OrderStatus: orderservice.OrderStatusReturned}, nil
			},
		}

		mockOrderItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemProductID: 100, OrderItemQuantity: 2},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderItemRepo:   mockOrderItemRepo,
			orderReturnRepo: mockReturnRepo,
		})

		res, err := svc.AdminUpdateReturnStatus(ctx, adminUUID, returnUUID, v1dto.AdminUpdateReturnInput{
			Status: orderservice.OrderReturnStatusCompleted,
		})
		require.NoError(t, err)
		assert.Equal(t, orderservice.OrderReturnStatusCompleted, res.Status)
	})

	t.Run("Case 3: Return Completion Does Not Restock Directly", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusApproved,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnStatus:    orderservice.OrderReturnStatusCompleted,
					ReturnAdminNote: params.AdminNote,
				}, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:     10,
					OrderUserID: 2,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 10, OrderStatus: orderservice.OrderStatusReturned}, nil
			},
		}
		mockOrderItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemProductID: 100, OrderItemQuantity: 1},
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			orderItemRepo:   mockOrderItemRepo,
		})

		_, err := svc.AdminUpdateReturnStatus(ctx, adminUUID, returnUUID, v1dto.AdminUpdateReturnInput{
			Status: orderservice.OrderReturnStatusCompleted,
		})
		require.NoError(t, err)
	})
}

func TestOrderService_CancelOrderReturn(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Member Cancels Pending Return", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}

		returnUUID := uuid.New()
		mockReturnRepo := &MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  orderservice.OrderReturnStatusPending,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:     1,
					ReturnUuid:   returnUUID,
					ReturnStatus: orderservice.OrderReturnStatusCancelled,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "return_cancelled", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			trackingRepo:    mockTrackingRepo,
		})

		res, err := svc.CancelOrderReturn(ctx, userUUID, orderUUID, v1dto.CancelOrderReturnInput{Reason: "Decided to keep it"})
		require.NoError(t, err)
		assert.True(t, trackingInserted)
		assert.Equal(t, orderservice.OrderReturnStatusCancelled, res.Status)
	})
}

func TestOrderService_SubmitReturnShipping(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Submits Return Tracking", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}

		returnUUID := uuid.New()
		mockReturnRepo := &MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  orderservice.OrderReturnStatusApproved,
				}, nil
			},
			UpdateReturnShippingInfoFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:               1,
					ReturnUuid:             returnUUID,
					ReturnStatus:           orderservice.OrderReturnStatusApproved,
					ReturnShippingProvider: params.ReturnShippingProvider,
					ReturnShippingCode:     params.ReturnShippingCode,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "return_shipping_submitted", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			trackingRepo:    mockTrackingRepo,
		})

		res, err := svc.SubmitReturnShipping(ctx, userUUID, orderUUID, v1dto.SubmitReturnShippingInput{
			ShippingProvider: "ghn",
			ShippingCode:     "RETGHN9999",
		})
		require.NoError(t, err)
		assert.True(t, trackingInserted)
		assert.Equal(t, "ghn", res.ShippingProvider)
		assert.Equal(t, "RETGHN9999", res.ShippingCode)
	})
}

func TestOrderService_UpdateReturnBankAccount(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: orderservice.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Updates Bank Account", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}

		returnUUID := uuid.New()
		mockReturnRepo := &MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  orderservice.OrderReturnStatusApproved,
				}, nil
			},
			UpdateReturnBankAccountFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:              1,
					ReturnUuid:            returnUUID,
					ReturnStatus:          orderservice.OrderReturnStatusApproved,
					ReturnBankName:        params.ReturnBankName,
					ReturnBankAccount:     params.ReturnBankAccount,
					ReturnBankAccountName: params.ReturnBankAccountName,
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
		})

		res, err := svc.UpdateReturnBankAccount(ctx, userUUID, orderUUID, v1dto.UpdateReturnBankAccountInput{
			BankName:        "MBBank",
			BankAccount:     "999888777",
			BankAccountName: "Nguyen Van B",
		})
		require.NoError(t, err)
		assert.Equal(t, "MBBank", res.BankName)
		assert.Equal(t, "999888777", res.BankAccount)
	})
}

func TestOrderService_AdminProcessReturnRefund(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	returnUUID := uuid.New()
	orderUUID := uuid.New()

	adminUser := sqlc.User{
		UserID:    1,
		UserUuid:  adminUUID,
		UserLevel: orderservice.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Manual Bank Refund via PaymentService", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         orderservice.OrderStatusReturned,
						OrderPaymentMethod:  orderservice.OrderPaymentMethodCOD,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(300000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, id int32) (sqlc.GetOrderByIDRow, error) {
				if id == 10 {
					return sqlc.GetOrderByIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         int32(orderservice.OrderStatusReturned),
						OrderPaymentMethod:  orderservice.OrderPaymentMethodCOD,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(300000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByIDRow{}, pgx.ErrNoRows
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
				assert.Equal(t, int32(orderservice.OrderStatusReturned), params.OrderStatus)
				return sqlc.Order{}, nil
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusCompleted,
					ReturnReason:  "Defective part",
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				assert.Equal(t, int16(orderservice.OrderReturnStatusCompleted), params.Status)
				return sqlc.OrderReturn{
					ReturnID:     1,
					ReturnUuid:   returnUUID,
					ReturnStatus: orderservice.OrderReturnStatusCompleted,
				}, nil
			},
		}

		var refundCalled bool
		mockPaymentService := &MockPaymentRefundPort{
			RefundPaymentFunc: func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
				refundCalled = true
				assert.Equal(t, orderUUID.String(), input.OrderUUID)
				assert.Equal(t, 300000.0, input.Amount)
				refAmt := 300000.0
				refTxn := "MB123456789"
				return v1dto.PaymentTransactionDTO{
					UUID:          uuid.New().String(),
					Amount:        300000.0,
					RefundAmount:  &refAmt,
					Provider:      "bank_transfer",
					ProviderTxnID: &refTxn,
					Status:        "Paid",
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "refunded", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var outboxInserted bool
		var refundEventChangeType string
		mockOutbox := &MockOutboxRepository{
			InsertFunc: func(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
				outboxInserted = true
				assert.Equal(t, "order", aggregateType)
				assert.Contains(t, []string{
					rabbitmq.QueueOrderReturnUpdated,
					rabbitmq.QueueOrderStatusChanged,
				}, eventType)
				if eventType == rabbitmq.QueueOrderReturnUpdated {
					event, ok := payload.(rabbitmq.OrderReturnUpdatedEvent)
					require.True(t, ok)
					refundEventChangeType = event.ChangeType
				}
				return sqlc.OutboxEvent{}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			trackingRepo:    mockTrackingRepo,
			paymentService:  mockPaymentService,
			outboxStore:     mockOutbox,
		})

		refundAmt := 300000.0
		res, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{
			Amount:     &refundAmt,
			BankTxnRef: "MB123456789",
		})
		require.NoError(t, err)
		assert.False(t, refundCalled)
		assert.True(t, trackingInserted)
		assert.True(t, outboxInserted)
		assert.Equal(t, rabbitmq.ReturnChangeTypeRefunded, refundEventChangeType)
		assert.Equal(t, 300000.0, res.RefundAmount)
		assert.Equal(t, "MB123456789", res.RefundTxnID)
	})

	t.Run("Case 2: Happy Path - Gateway Refund (MoMo/VNPAY) via PaymentService", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         orderservice.OrderStatusReturned,
						OrderPaymentMethod:  orderservice.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, id int32) (sqlc.GetOrderByIDRow, error) {
				if id == 10 {
					return sqlc.GetOrderByIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         int32(orderservice.OrderStatusReturned),
						OrderPaymentMethod:  orderservice.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByIDRow{}, pgx.ErrNoRows
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusCompleted,
					ReturnReason:  "Defective item",
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{}, nil
			},
		}

		mockPaymentService := &MockPaymentRefundPort{
			RefundPaymentFunc: func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
				assert.Equal(t, 200000.0, input.Amount)
				refAmt := 200000.0
				refTxn := "MOMO_REFUND_999"
				return v1dto.PaymentTransactionDTO{
					UUID:          uuid.New().String(),
					Amount:        200000.0,
					RefundAmount:  &refAmt,
					Provider:      "momo",
					ProviderTxnID: &refTxn,
					Status:        "Paid",
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			paymentService:  mockPaymentService,
		})

		partialAmt := 200000.0
		res, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{
			Amount: &partialAmt,
		})
		require.NoError(t, err)
		assert.Equal(t, 200000.0, res.RefundAmount)
		assert.Equal(t, "MOMO_REFUND_999", res.RefundTxnID)
	})

	t.Run("Case 3: Gateway Refund Failure - Propagates PaymentService Error", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         orderservice.OrderStatusReturned,
						OrderPaymentMethod:  orderservice.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, id int32) (sqlc.GetOrderByIDRow, error) {
				if id == 10 {
					return sqlc.GetOrderByIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         int32(orderservice.OrderStatusReturned),
						OrderPaymentMethod:  orderservice.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  orderservice.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByIDRow{}, pgx.ErrNoRows
			},
		}

		mockReturnRepo := &MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  orderservice.OrderReturnStatusCompleted,
					ReturnReason:  "Defective item",
				}, nil
			},
		}

		mockPaymentService := &MockPaymentRefundPort{
			RefundPaymentFunc: func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
				return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadGateway, "gateway connection timeout")
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			paymentService:  mockPaymentService,
		})

		_, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadGateway)
	})
}
