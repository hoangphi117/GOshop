package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"
	"goshop/pkg/payment"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReturnServiceDeps struct {
	txManager           db.TxManager
	orderRepo           *testutils.MockOrderRepository
	orderItemRepo       *testutils.MockOrderItemRepository
	orderReturnRepo     *testutils.MockOrderReturnsRepository
	productRepo         *testutils.MockProductRepository
	userRepo            *testutils.MockUserRepository
	trackingRepo        *testutils.MockTrackingRepository
	paymentRepo         *testutils.MockPaymentRepository
	paymentProviders    map[string]payment.PaymentProvider
	notificationService *testutils.MockNotificationService
}

func newTestReturnService(deps testReturnServiceDeps) v1service.OrderService {
	txMgr := deps.txManager
	if txMgr == nil {
		txMgr = testutils.NewNoopTxManager()
	}
	var userRepo repository.UserRepository
	if deps.userRepo != nil {
		userRepo = deps.userRepo
	} else {
		userRepo = &testutils.MockUserRepository{}
	}
	var orderRepo repository.OrderRepository
	if deps.orderRepo != nil {
		orderRepo = deps.orderRepo
	} else {
		orderRepo = &testutils.MockOrderRepository{}
	}
	var orderItemRepo repository.OrderItemRepository
	if deps.orderItemRepo != nil {
		orderItemRepo = deps.orderItemRepo
	} else {
		orderItemRepo = &testutils.MockOrderItemRepository{}
	}
	var returnRepo repository.OrderReturnsRepository
	if deps.orderReturnRepo != nil {
		returnRepo = deps.orderReturnRepo
	} else {
		returnRepo = &testutils.MockOrderReturnsRepository{}
	}
	var productRepo repository.ProductRepository
	if deps.productRepo != nil {
		productRepo = deps.productRepo
	} else {
		productRepo = &testutils.MockProductRepository{}
	}
	var trackingRepo repository.TrackingRepository
	if deps.trackingRepo != nil {
		trackingRepo = deps.trackingRepo
	} else {
		trackingRepo = &testutils.MockTrackingRepository{}
	}
	var paymentRepo repository.PaymentRepository
	if deps.paymentRepo != nil {
		paymentRepo = deps.paymentRepo
	} else {
		paymentRepo = &testutils.MockPaymentRepository{}
	}
	var notifSvc v1service.NotificationService
	if deps.notificationService != nil {
		notifSvc = deps.notificationService
	} else {
		notifSvc = &testutils.MockNotificationService{}
	}

	return v1service.NewOrderService(
		txMgr,
		&testutils.MockOrderPricingService{},
		&testutils.MockInventoryService{},
		orderRepo,
		orderItemRepo,
		returnRepo,
		productRepo,
		userRepo,
		&testutils.MockCartService{},
		&testutils.MockCartRepository{},
		&testutils.MockCouponRepository{},
		trackingRepo,
		nil,
		paymentRepo,
		deps.paymentProviders,
		notifSvc,
		nil,
		nil,
		nil,
	)
}

func TestOrderService_RequestOrderReturn(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
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
		mockUserRepo := &testutils.MockUserRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 999, // Different owner
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusShipped, // Not delivered
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
					OrderDeliveredAt: pgtype.Timestamptz{
						Time:  deliveredRecently,
						Valid: true,
					},
				}, nil
			},
		}
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetActiveByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{ReturnID: 55}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
		})

		_, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, nil, nil)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 7: Happy Path - Creates Return, Tracking Event, and Sends Notification", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
					OrderStatus: v1service.OrderStatusDelivered,
					OrderDeliveredAt: pgtype.Timestamptz{
						Time:  deliveredRecently,
						Valid: true,
					},
				}, nil
			},
		}

		returnUUID := uuid.New()
		var returnCreated bool
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
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
					ReturnStatus:    v1service.OrderReturnStatusPending,
					ReturnCreatedAt: time.Now(),
					ReturnUpdatedAt: time.Now(),
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, int32(10), params.EventOrderID)
				assert.Equal(t, "return_requested", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var notificationSent bool
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notificationSent = true
				assert.Equal(t, "return_requested", input.Type)
				return nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:            mockUserRepo,
			orderRepo:           mockOrderRepo,
			orderReturnRepo:     mockReturnRepo,
			trackingRepo:        mockTrackingRepo,
			notificationService: mockNotification,
		})

		res, err := svc.RequestOrderReturn(ctx, userUUID, orderUUID, input, []string{"img1.png"}, nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.True(t, returnCreated)
		assert.True(t, trackingInserted)
		time.Sleep(25 * time.Millisecond) // Allow async notification to run
		assert.True(t, notificationSent)
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
		UserLevel: v1service.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
				}, nil
			},
		}
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnOrderID:   10,
					ReturnUserID:    1,
					ReturnReason:    "Defective screen",
					ReturnStatus:    v1service.OrderReturnStatusApproved,
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
		assert.Equal(t, int16(v1service.OrderReturnStatusApproved), res.Status)
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
		UserLevel: v1service.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
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
						ReturnStatus:     v1service.OrderReturnStatusPending,
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
		UserLevel: v1service.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Unauthorized/Forbidden for Non-Staff", func(t *testing.T) {
		memberUser := sqlc.User{
			UserID:    2,
			UserUuid:  adminUUID,
			UserLevel: v1service.RoleMember,
		}
		mockUserRepo := &testutils.MockUserRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			CountReturnsFunc: func(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error) {
				return 1, nil
			},
			GetReturnsFunc: func(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error) {
				return []sqlc.AdminGetReturnsRow{
					{
						ReturnID:         1,
						ReturnUuid:       returnUUID,
						OrderUuid:        orderUUID,
						OrderNumber:      "ORD-2026-001",
						OrderTotalAmount: utils.FloatToNumeric(750000),
						UserFullname:     "Customer A",
						UserEmail:        "customer@example.com",
						ReturnReason:     "Wrong item received",
						ReturnStatus:     v1service.OrderReturnStatusPending,
						ReturnCreatedAt:  time.Now(),
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
		UserLevel: v1service.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Admin Gets Return Detail by Return UUID", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
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
					ReturnStatus:     v1service.OrderReturnStatusPending,
					ReturnImages:     []string{"img.jpg"},
					ReturnCreatedAt:  time.Now(),
					ReturnUpdatedAt:  time.Now(),
				}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
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

		mockTrackingRepo := &testutils.MockTrackingRepository{
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
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
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
					ReturnStatus:     v1service.OrderReturnStatusPending,
				}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
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
		UserLevel: v1service.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Approve Return", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  v1service.OrderReturnStatusPending,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnStatus:    v1service.OrderReturnStatusApproved,
					ReturnAdminNote: params.AdminNote,
				}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
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
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "return_approved", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var notificationSent bool
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notificationSent = true
				assert.Equal(t, "return_approved", input.Type)
				return nil
			},
		}

		note := "Return accepted"
		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:            mockUserRepo,
			orderRepo:           mockOrderRepo,
			orderReturnRepo:     mockReturnRepo,
			trackingRepo:        mockTrackingRepo,
			notificationService: mockNotification,
		})

		res, err := svc.AdminUpdateReturnStatus(ctx, adminUUID, returnUUID, v1dto.AdminUpdateReturnInput{
			Status:    v1service.OrderReturnStatusApproved,
			AdminNote: &note,
		})
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
		assert.True(t, trackingInserted)
		assert.True(t, notificationSent)
		assert.Equal(t, v1service.OrderReturnStatusApproved, res.Status)
	})

	t.Run("Case 2: Happy Path - Complete Return Restores Stock", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:        1,
					ReturnUuid:      returnUUID,
					ReturnStatus:    v1service.OrderReturnStatusCompleted,
					ReturnAdminNote: params.AdminNote,
				}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:     10,
					OrderUserID: 2,
					OrderUuid:   orderUUID,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
				assert.Equal(t, v1service.OrderStatusReturned, params.OrderStatus)
				return sqlc.Order{OrderID: 10, OrderStatus: v1service.OrderStatusReturned}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemProductID: 100, OrderItemQuantity: 2},
				}, nil
			},
		}

		var stockRestored bool
		mockProductRepo := &testutils.MockProductRepository{
			RestoreStockFunc: func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
				stockRestored = true
				assert.Equal(t, int32(100), productID)
				assert.Equal(t, int32(2), quantity)
				return nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderItemRepo:   mockOrderItemRepo,
			orderReturnRepo: mockReturnRepo,
			productRepo:     mockProductRepo,
		})

		res, err := svc.AdminUpdateReturnStatus(ctx, adminUUID, returnUUID, v1dto.AdminUpdateReturnInput{
			Status: v1service.OrderReturnStatusCompleted,
		})
		require.NoError(t, err)
		assert.True(t, stockRestored)
		assert.Equal(t, v1service.OrderReturnStatusCompleted, res.Status)
	})
}

func TestOrderService_CancelOrderReturn(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Member Cancels Pending Return", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
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
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  v1service.OrderReturnStatusPending,
				}, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:     1,
					ReturnUuid:   returnUUID,
					ReturnStatus: v1service.OrderReturnStatusCancelled,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
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
		assert.Equal(t, v1service.OrderReturnStatusCancelled, res.Status)
	})
}

func TestOrderService_SubmitReturnShipping(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Submits Return Tracking", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
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
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
				}, nil
			},
			UpdateReturnShippingInfoFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:               1,
					ReturnUuid:             returnUUID,
					ReturnStatus:           v1service.OrderReturnStatusApproved,
					ReturnShippingProvider: params.ReturnShippingProvider,
					ReturnShippingCode:     params.ReturnShippingCode,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
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
		UserLevel: v1service.RoleMember,
		UserEmail: "buyer@example.com",
	}

	t.Run("Case 1: Happy Path - Updates Bank Account", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
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
		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetLatestByOrderIDFunc: func(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnUserID:  1,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
				}, nil
			},
			UpdateReturnBankAccountFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:              1,
					ReturnUuid:            returnUUID,
					ReturnStatus:          v1service.OrderReturnStatusApproved,
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
		UserLevel: v1service.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	t.Run("Case 1: Happy Path - Manual Bank Refund", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         v1service.OrderStatusReturned,
						OrderPaymentMethod:  v1service.OrderPaymentMethodCOD,
						OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(300000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:             10,
					OrderUserID:         2,
					OrderUuid:           orderUUID,
					OrderNumber:         "ORD-2026-001",
					OrderStatus:         v1service.OrderStatusReturned,
					OrderPaymentMethod:  v1service.OrderPaymentMethodCOD,
					OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
					OrderTotalAmount:    utils.FloatToNumeric(300000),
					OrderRefundedAmount: utils.FloatToNumeric(0),
				}, nil
			},
			UpdateOrderPaymentStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:             10,
					OrderPaymentStatus:  params.PaymentStatus,
					OrderRefundedAmount: params.RefundedAmount,
				}, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
					ReturnReason:  "Defective part",
				}, nil
			},
		}

		var refundTxnCreated bool
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByOrderIDFunc: func(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
				return []sqlc.PaymentTransaction{}, nil
			},
			CreateRefundFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error) {
				refundTxnCreated = true
				assert.Equal(t, int32(10), params.OrderID)
				return sqlc.PaymentTransaction{
					PtID: 100,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "refunded", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var notificationSent bool
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notificationSent = true
				assert.Equal(t, "return_refunded", input.Type)
				return nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:            mockUserRepo,
			orderRepo:           mockOrderRepo,
			orderReturnRepo:     mockReturnRepo,
			trackingRepo:        mockTrackingRepo,
			paymentRepo:         mockPaymentRepo,
			notificationService: mockNotification,
		})

		refundAmt := 300000.0
		res, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{
			Amount:     &refundAmt,
			BankTxnRef: "MB123456789",
		})
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
		assert.True(t, refundTxnCreated)
		assert.True(t, trackingInserted)
		assert.True(t, notificationSent)
		assert.Equal(t, 300000.0, res.RefundAmount)
		assert.Equal(t, "Refunded", res.PaymentStatus)
	})

	t.Run("Case 2: Happy Path - Gateway Refund (MoMo/VNPAY)", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         v1service.OrderStatusReturned,
						OrderPaymentMethod:  v1service.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:             10,
					OrderUserID:         2,
					OrderUuid:           orderUUID,
					OrderNumber:         "ORD-2026-001",
					OrderStatus:         v1service.OrderStatusReturned,
					OrderPaymentMethod:  v1service.OrderPaymentMethodEWallet,
					OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
					OrderTotalAmount:    utils.FloatToNumeric(500000),
					OrderRefundedAmount: utils.FloatToNumeric(0),
				}, nil
			},
			UpdateOrderPaymentStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:             10,
					OrderPaymentStatus:  params.PaymentStatus,
					OrderRefundedAmount: params.RefundedAmount,
				}, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
					ReturnReason:  "Defective item",
				}, nil
			},
		}

		providerTxnID := "MOMO_TRANS_123"
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByOrderIDFunc: func(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
				return []sqlc.PaymentTransaction{
					{
						PtID:            5,
						PtProvider:      "momo",
						PtStatus:        v1service.PaymentStatusPaid,
						PtProviderTxnID: &providerTxnID,
					},
				}, nil
			},
			CreateRefundFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{PtID: 105}, nil
			},
		}

		mockPaymentProvider := &testutils.MockPaymentProvider{
			RefundFunc: func(ctx context.Context, req payment.RefundRequest) (payment.RefundResponse, error) {
				assert.Equal(t, 200000.0, req.Amount)
				return payment.RefundResponse{
					RefundTxnID: "MOMO_REFUND_999",
					Amount:      req.Amount,
					Status:      payment.TxnStatusSuccess,
				}, nil
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			paymentRepo:     mockPaymentRepo,
			paymentProviders: map[string]payment.PaymentProvider{
				"momo": mockPaymentProvider,
			},
		})

		partialAmt := 200000.0
		res, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{
			Amount: &partialAmt,
		})
		require.NoError(t, err)
		assert.Equal(t, 200000.0, res.RefundAmount)
		assert.Equal(t, "Partially Refunded", res.PaymentStatus)
		assert.Equal(t, "MOMO_REFUND_999", res.RefundTxnID)
	})

	t.Run("Case 3: Gateway Refund Failure - Returns 502 Bad Gateway", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				if u == orderUUID {
					return sqlc.GetOrderByUUIDRow{
						OrderID:             10,
						OrderUserID:         2,
						OrderUuid:           orderUUID,
						OrderNumber:         "ORD-2026-001",
						OrderStatus:         v1service.OrderStatusReturned,
						OrderPaymentMethod:  v1service.OrderPaymentMethodEWallet,
						OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
						OrderTotalAmount:    utils.FloatToNumeric(500000),
						OrderRefundedAmount: utils.FloatToNumeric(0),
					}, nil
				}
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
			GetByIDFunc: func(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error) {
				return sqlc.GetOrderByIDRow{
					OrderID:             10,
					OrderUserID:         2,
					OrderUuid:           orderUUID,
					OrderNumber:         "ORD-2026-001",
					OrderStatus:         v1service.OrderStatusReturned,
					OrderPaymentMethod:  v1service.OrderPaymentMethodEWallet,
					OrderPaymentStatus:  v1service.OrderPaymentStatusPaid,
					OrderTotalAmount:    utils.FloatToNumeric(500000),
					OrderRefundedAmount: utils.FloatToNumeric(0),
				}, nil
			},
		}

		mockReturnRepo := &testutils.MockOrderReturnsRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.OrderReturn, error) {
				return sqlc.OrderReturn{
					ReturnID:      1,
					ReturnUuid:    returnUUID,
					ReturnOrderID: 10,
					ReturnStatus:  v1service.OrderReturnStatusApproved,
					ReturnReason:  "Defective item",
				}, nil
			},
		}

		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByOrderIDFunc: func(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
				providerTxnID := "MOMO_TRANS_123"
				return []sqlc.PaymentTransaction{
					{
						PtID:            5,
						PtProvider:      "momo",
						PtStatus:        v1service.PaymentStatusPaid,
						PtProviderTxnID: &providerTxnID,
					},
				}, nil
			},
		}

		mockPaymentProvider := &testutils.MockPaymentProvider{
			RefundFunc: func(ctx context.Context, req payment.RefundRequest) (payment.RefundResponse, error) {
				return payment.RefundResponse{}, errors.New("gateway connection timeout")
			},
		}

		svc := newTestReturnService(testReturnServiceDeps{
			userRepo:        mockUserRepo,
			orderRepo:       mockOrderRepo,
			orderReturnRepo: mockReturnRepo,
			paymentRepo:     mockPaymentRepo,
			paymentProviders: map[string]payment.PaymentProvider{
				"momo": mockPaymentProvider,
			},
		})

		_, err := svc.AdminProcessReturnRefund(ctx, adminUUID, returnUUID, v1dto.AdminReturnRefundInput{})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadGateway)
	})
}
