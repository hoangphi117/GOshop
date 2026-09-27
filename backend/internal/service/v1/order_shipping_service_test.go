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
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testShippingServiceDeps struct {
	txManager           db.TxManager
	orderRepo           repository.OrderRepository
	orderItemRepo       repository.OrderItemRepository
	trackingRepo        repository.TrackingRepository
	paymentRepo         repository.PaymentRepository
	shippingFactory     *shipping.ShippingFactory
	webhookRepo         repository.WebhookEventRepository
	notificationService v1service.NotificationService
	userRepo            repository.UserRepository
	redisClient         *redis.Client
	rabbitmq            rabbitmq.RabbitMQService
}

func newTestShippingService(deps testShippingServiceDeps) v1service.OrderShippingService {
	txMgr := deps.txManager
	if txMgr == nil {
		txMgr = testutils.NewNoopTxManager()
	}
	orderRepo := deps.orderRepo
	if orderRepo == nil {
		orderRepo = &testutils.MockOrderRepository{}
	}
	orderItemRepo := deps.orderItemRepo
	if orderItemRepo == nil {
		orderItemRepo = &testutils.MockOrderItemRepository{}
	}
	trackingRepo := deps.trackingRepo
	if trackingRepo == nil {
		trackingRepo = &testutils.MockTrackingRepository{}
	}
	paymentRepo := deps.paymentRepo
	if paymentRepo == nil {
		paymentRepo = &testutils.MockPaymentRepository{}
	}
	webhookRepo := deps.webhookRepo
	if webhookRepo == nil {
		webhookRepo = &testutils.MockWebhookEventRepository{}
	}
	notifService := deps.notificationService
	if notifService == nil {
		notifService = &testutils.MockNotificationService{}
	}
	userRepo := deps.userRepo
	if userRepo == nil {
		userRepo = &testutils.MockUserRepository{}
	}

	return v1service.NewOrderShippingService(
		txMgr,
		orderRepo,
		orderItemRepo,
		trackingRepo,
		paymentRepo,
		deps.shippingFactory,
		webhookRepo,
		notifService,
		userRepo,
		deps.redisClient,
		deps.rabbitmq,
	)
}

func assertAppErrorCode(t *testing.T, err error, expectedCode utils.ErrorCode) {
	t.Helper()
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, expectedCode, appErr.Code)
	} else {
		t.Fatalf("expected *utils.AppError, got %T: %v", err, err)
	}
}

func TestOrderService_CreateShipping(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()

	baseInput := v1dto.CreateShippingInput{
		Provider:     "ghn",
		DistrictID:   1442,
		WardCode:     "20101",
		RequiredNote: "KHONGCHOXEMHANG",
		Note:         "Handle with care",
		Weight:       500,
		Length:       10,
		Width:        10,
		Height:       10,
	}

	t.Run("Case 1: Order Not Found - Returns 404 NotFound", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		res, err := svc.CreateShipping(ctx, orderUUID, baseInput)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, res.OrderUUID)
	})

	t.Run("Case 2: Invalid Order Status - Not Confirmed", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusPending,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		res, err := svc.CreateShipping(ctx, orderUUID, baseInput)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "must be Confirmed")
		assert.Empty(t, res.OrderUUID)
	})

	t.Run("Case 3: Already Has Shipping Code - Returns 409 Conflict", func(t *testing.T) {
		existingCode := "GHN123456"
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderStatus:       v1service.OrderStatusConfirmed,
					OrderShippingCode: &existingCode,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		res, err := svc.CreateShipping(ctx, orderUUID, baseInput)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "already created")
		assert.Empty(t, res.OrderUUID)
	})

	t.Run("Case 4: RabbitMQ Publish Error - Returns Error", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusConfirmed,
				}, nil
			},
		}

		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				return errors.New("rabbitmq connection down")
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			rabbitmq: mockRabbitMQ,
		})

		res, err := svc.CreateShipping(ctx, orderUUID, baseInput)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
		assert.Empty(t, res.OrderUUID)
	})

	t.Run("Case 5: Happy Path - Enqueues Message and Returns Accepted", func(t *testing.T) {
		var publishedQueue string
		var publishedMsg any

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusConfirmed,
					OrderNumber: "ORD-2026-001",
				}, nil
			},
		}

		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				publishedQueue = queue
				publishedMsg = message
				return nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			rabbitmq: mockRabbitMQ,
		})

		res, err := svc.CreateShipping(ctx, orderUUID, baseInput)
		require.NoError(t, err)
		assert.Equal(t, orderUUID.String(), res.OrderUUID)
		assert.Equal(t, "processing", res.Status)
		assert.Equal(t, v1service.QueueShippingCreate, publishedQueue)
		assert.NotNil(t, publishedMsg)
	})
}

func TestOrderService_ProcessShippingOrderMessage(t *testing.T) {
	orderUUID := uuid.New()
	msg := v1dto.CreateShippingQueueMessage{
		OrderUUID: orderUUID.String(),
		Input: v1dto.CreateShippingInput{
			Provider:     "ghn",
			DistrictID:   1442,
			WardCode:     "20101",
			RequiredNote: "KHONGCHOXEMHANG",
			Note:         "Handle with care",
			Weight:       500,
			Length:       10,
			Width:        10,
			Height:       10,
		},
		EnqueuedAt: time.Now(),
	}

	t.Run("Case 1: Invalid Order UUID in Message - Returns Error", func(t *testing.T) {
		svc := newTestShippingService(testShippingServiceDeps{})

		err := svc.ProcessShippingOrder(context.Background(), v1dto.CreateShippingQueueMessage{OrderUUID: "invalid-uuid"})
		require.Error(t, err)
	})

	t.Run("Case 2: Order Not Found - Returns Error", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		err := svc.ProcessShippingOrder(context.Background(), msg)
		require.Error(t, err)
	})

	t.Run("Case 3: Provider Create Order Failure - Returns Error", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:          10,
					OrderUuid:        orderUUID,
					OrderStatus:      v1service.OrderStatusConfirmed,
					OrderNumber:      "ORD-2026-001",
					OrderShippingFee: utils.FloatToNumeric(30000),
				}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemID: 1, OrderItemProductName: "Item 1", OrderItemQuantity: 2, OrderItemPrice: utils.FloatToNumeric(100000)},
				}, nil
			},
		}

		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			CreateShippingOrderFunc: func(ctx context.Context, req shipping.CreateOrderRequest) (*shipping.CreateOrderResponse, error) {
				return nil, errors.New("GHN API timeout")
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo:       mockOrderRepo,
			orderItemRepo:   mockOrderItemRepo,
			shippingFactory: factory,
		})

		err := svc.ProcessShippingOrder(context.Background(), msg)
		require.Error(t, err)
	})

	t.Run("Case 4: Happy Path - Creates Shipping Order, Updates Status, Inserts Event, Sends Notification", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUserID:        1,
					OrderUuid:          orderUUID,
					OrderStatus:        v1service.OrderStatusConfirmed,
					OrderNumber:        "ORD-2026-001",
					OrderShippingFee:   utils.FloatToNumeric(30000),
					OrderTotalAmount:   utils.FloatToNumeric(230000),
					OrderPaymentMethod: v1service.OrderPaymentMethodCOD,
				}, nil
			},
			UpdateShippingAndStatusFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusShipped,
				}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{OrderItemID: 1, OrderItemProductName: "Item 1", OrderItemQuantity: 2, OrderItemPrice: utils.FloatToNumeric(100000)},
				}, nil
			},
		}

		var eventInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				eventInserted = true
				assert.Equal(t, "shipped", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var notificationSent bool
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notificationSent = true
				assert.Equal(t, "order_shipped", input.Type)
				return nil
			},
		}

		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			CreateShippingOrderFunc: func(ctx context.Context, req shipping.CreateOrderRequest) (*shipping.CreateOrderResponse, error) {
				return &shipping.CreateOrderResponse{
					ShippingCode: "GHN888888",
					ShippingFee:  35000,
				}, nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			orderItemRepo: mockOrderItemRepo,
			trackingRepo: mockTrackingRepo,
			shippingFactory: factory,
			notificationService: mockNotification,
		})

		err := svc.ProcessShippingOrder(context.Background(), msg)
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
		assert.True(t, eventInserted)
		assert.True(t, notificationSent)
	})
}

func TestOrderService_GetOrderTimeline(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "member@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.GetOrderTimeline(ctx, userUUID, orderUUID)
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

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.GetOrderTimeline(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden - Other Member Cannot View Order Timeline", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 999, // Owned by another user
					OrderUuid:   orderUUID,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.GetOrderTimeline(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: Success for Owner - Returns Timeline and Events", func(t *testing.T) {
		shippingCode := "GHN999888"
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUserID:       1,
					OrderUuid:         orderUUID,
					OrderNumber:       "ORD-2026-001",
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: &shippingCode,
				}, nil
			},
		}

		note := "Package handed over to carrier"
		mockTrackingRepo := &testutils.MockTrackingRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error) {
				return []sqlc.OrderTrackingEvent{
					{
						EventID:        1,
						EventOrderID:   10,
						EventStatus:    "shipped",
						EventNote:      &note,
						EventSource:    "system",
						EventCreatedAt: time.Now(),
					},
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			userRepo: mockUserRepo,
		})

		timeline, err := svc.GetOrderTimeline(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		assert.Equal(t, orderUUID.String(), timeline.OrderUUID)
		assert.Equal(t, "ORD-2026-001", timeline.OrderNumber)
		assert.Len(t, timeline.Events, 1)
		assert.Equal(t, "shipped", timeline.Events[0].Status)
	})
}

func TestOrderService_HandleShippingWebhook(t *testing.T) {
	ctx := context.Background()
	shippingCode := "GHN123456789"
	orderUUID := uuid.New()

	rawBody := []byte(`{"OrderCode":"GHN123456789","Status":"delivered"}`)
	payload := v1dto.GHNWebhookPayload{
		OrderCode: shippingCode,
		Status:    "delivered",
	}

	t.Run("Case 1: Duplicate Webhook Event - Returns Error", func(t *testing.T) {
		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{}, errors.New("duplicate key violates unique constraint")
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			webhookRepo: mockWebhookRepo,
		})

		err := svc.HandleShippingWebhook(ctx, rawBody, payload)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 2: Order Not Found by Shipping Code - Acknowledges and Marks Processed", func(t *testing.T) {
		var webhookMarkedProcessed bool
		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{WeID: 1}, nil
			},
			MarkProcessedFunc: func(ctx context.Context, tx pgx.Tx, eventID int32) error {
				webhookMarkedProcessed = true
				return nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByShippingCodeFunc: func(ctx context.Context, sc string) (sqlc.GetOrderByShippingCodeRow, error) {
				return sqlc.GetOrderByShippingCodeRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			webhookRepo: mockWebhookRepo,
		})

		err := svc.HandleShippingWebhook(ctx, rawBody, payload)
		require.NoError(t, err)
		assert.True(t, webhookMarkedProcessed)
	})

	t.Run("Case 3: Delivered Webhook - Marks Delivered, Inserts Event, Sends Notification, Marks Processed", func(t *testing.T) {
		var webhookMarkedProcessed bool
		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{WeID: 100}, nil
			},
			MarkProcessedFunc: func(ctx context.Context, tx pgx.Tx, eventID int32) error {
				webhookMarkedProcessed = true
				assert.Equal(t, int32(100), eventID)
				return nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByShippingCodeFunc: func(ctx context.Context, sc string) (sqlc.GetOrderByShippingCodeRow, error) {
				return sqlc.GetOrderByShippingCodeRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderNumber:        "ORD-2026-001",
					OrderStatus:        v1service.OrderStatusShipped,
					OrderPaymentMethod: v1service.OrderPaymentMethodCOD,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderTotalAmount:   utils.FloatToNumeric(250000),
					OrderShippingCode:  &shippingCode,
				}, nil
			},
			MarkDeliveredByShippingCodeFunc: func(ctx context.Context, tx pgx.Tx, sc string) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:            10,
					OrderUserID:        1,
					OrderUuid:          orderUUID,
					OrderStatus:        v1service.OrderStatusDelivered,
					OrderPaymentStatus: v1service.OrderPaymentStatusPaid,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "delivered", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var paymentTransactionCreated bool
		mockPaymentRepo := &testutils.MockPaymentRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				paymentTransactionCreated = true
				assert.Equal(t, int32(10), params.PtOrderID)
				assert.Equal(t, "ghn_cod", params.PtProvider)
				assert.Equal(t, v1service.PaymentStatusPaid, params.PtStatus)
				assert.Equal(t, 250000.0, utils.NumericToFloat(params.PtAmount))
				return sqlc.PaymentTransaction{PtID: 1}, nil
			},
		}

		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				assert.Equal(t, "order_delivered", input.Type)
				return nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo:           mockOrderRepo,
			trackingRepo:        mockTrackingRepo,
			paymentRepo:         mockPaymentRepo,
			webhookRepo:         mockWebhookRepo,
			notificationService: mockNotification,
		})

		err := svc.HandleShippingWebhook(ctx, rawBody, payload)
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
		assert.True(t, trackingInserted)
		assert.True(t, paymentTransactionCreated)
		assert.True(t, webhookMarkedProcessed)
	})

	t.Run("Case 4: Delivering Webhook - Inserts Delivering Tracking Event", func(t *testing.T) {
		deliveringPayload := v1dto.GHNWebhookPayload{
			OrderCode: shippingCode,
			Status:    "delivering",
			Warehouse: "Hub Tan Binh",
		}

		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{WeID: 101}, nil
			},
			MarkProcessedFunc: func(ctx context.Context, tx pgx.Tx, eventID int32) error {
				return nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByShippingCodeFunc: func(ctx context.Context, sc string) (sqlc.GetOrderByShippingCodeRow, error) {
				return sqlc.GetOrderByShippingCodeRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderNumber:       "ORD-2026-001",
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: &shippingCode,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "delivering", params.EventStatus)
				assert.Contains(t, *params.EventNote, "Hub Tan Binh")
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			webhookRepo: mockWebhookRepo,
		})

		err := svc.HandleShippingWebhook(ctx, rawBody, deliveringPayload)
		require.NoError(t, err)
		assert.True(t, trackingInserted)
	})
}

func TestOrderService_CalculateShippingFee(t *testing.T) {
	ctx := context.Background()

	input := v1dto.CalculateShippingFeeInput{
		Provider:   "ghn",
		DistrictID: 1442,
		WardCode:   "20101",
		Weight:     500,
	}

	t.Run("Case 1: Unsupported Provider - Returns 400", func(t *testing.T) {
		factory := shipping.NewShippingFactory()
		svc := newTestShippingService(testShippingServiceDeps{
			shippingFactory: factory,
		})

		res, err := svc.CalculateShippingFee(ctx, input)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Equal(t, float64(0), res.ShippingFee)
	})

	t.Run("Case 2: Happy Path - Returns Calculated Fee", func(t *testing.T) {
		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			CalculateFeeFunc: func(ctx context.Context, req shipping.CreateOrderRequest) (float64, error) {
				return 32000, nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			shippingFactory: factory,
		})

		res, err := svc.CalculateShippingFee(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, float64(32000), res.ShippingFee)
		assert.Equal(t, "ghn", res.Provider)
	})
}

func TestOrderService_CancelShipping(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()
	shippingCode := "GHN888999"

	input := v1dto.CancelShippingInput{
		Reason: "Customer requested urgent hold",
	}

	t.Run("Case 1: Order Not Found - Returns 404", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		_, err := svc.CancelShipping(ctx, orderUUID, input)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Invalid Status - Order Not Shipped", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusConfirmed,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		_, err := svc.CancelShipping(ctx, orderUUID, input)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Shipped")
	})

	t.Run("Case 3: Missing Shipping Code - Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: nil,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		_, err := svc.CancelShipping(ctx, orderUUID, input)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "shipping code")
	})

	t.Run("Case 4: Provider Cancel Failure - Returns Error", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: &shippingCode,
				}, nil
			},
		}

		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			CancelShippingOrderFunc: func(ctx context.Context, sc string) error {
				return errors.New("cannot cancel - parcel already picked up")
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			shippingFactory: factory,
		})

		_, err := svc.CancelShipping(ctx, orderUUID, input)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 5: Happy Path - Reverts Status to Confirmed and Inserts Tracking", func(t *testing.T) {
		var cancelledInDB bool
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: &shippingCode,
				}, nil
			},
			CancelShippingFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				cancelledInDB = true
				return sqlc.Order{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusConfirmed,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "shipping_cancelled", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			CancelShippingOrderFunc: func(ctx context.Context, sc string) error {
				assert.Equal(t, shippingCode, sc)
				return nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			shippingFactory: factory,
		})

		res, err := svc.CancelShipping(ctx, orderUUID, input)
		require.NoError(t, err)
		assert.True(t, cancelledInDB)
		assert.True(t, trackingInserted)
		assert.Equal(t, orderUUID.String(), res.OrderUUID)
		assert.Equal(t, "Confirmed", res.OrderStatus)
		assert.Equal(t, shippingCode, res.ShippingCode)
	})
}

func TestOrderService_ConfirmOrderDelivery(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "member@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.ConfirmOrderDelivery(ctx, userUUID, orderUUID)
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

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.ConfirmOrderDelivery(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden - Other Member Cannot Confirm Delivery", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 999, // Another owner
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusShipped,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.ConfirmOrderDelivery(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: Invalid Status - Order is Not Shipped (e.g., Pending)", func(t *testing.T) {
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
					OrderStatus: v1service.OrderStatusPending,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.ConfirmOrderDelivery(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "pending")
	})

	t.Run("Case 5: Happy Path - Marks Delivered, Inserts Event, Sends Notification", func(t *testing.T) {
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
					OrderStatus: v1service.OrderStatusShipped,
				}, nil
			},
			MarkDeliveredByUUIDFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "delivered", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		var notificationSent bool
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notificationSent = true
				assert.Equal(t, "order_completed", input.Type)
				return nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			notificationService: mockNotification,
			userRepo: mockUserRepo,
		})

		res, err := svc.ConfirmOrderDelivery(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
		assert.True(t, trackingInserted)
		assert.True(t, notificationSent)
		assert.Equal(t, orderUUID.String(), res.OrderUUID)
		assert.Equal(t, "Delivered", res.StatusName)
	})
}

func TestOrderService_GetPrintBill(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()
	shippingCode := "GHN888777"

	t.Run("Case 1: Order Not Found - Returns 404", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		_, err := svc.GetPrintBill(ctx, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Missing Shipping Code - Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderShippingCode: nil,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
		})

		_, err := svc.GetPrintBill(ctx, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "shipping code")
	})

	t.Run("Case 3: Happy Path - Returns Print URLs", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUuid:         orderUUID,
					OrderShippingCode: &shippingCode,
				}, nil
			},
		}

		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			GetPrintBillURLFunc: func(ctx context.Context, sc string) (*shipping.PrintBillResponse, error) {
				return &shipping.PrintBillResponse{
					Token:   "token_abc",
					PrintA5: "https://print.ghn.vn/a5/token_abc",
					Print80: "https://print.ghn.vn/80/token_abc",
					Print52: "https://print.ghn.vn/52/token_abc",
				}, nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			shippingFactory: factory,
		})

		res, err := svc.GetPrintBill(ctx, orderUUID)
		require.NoError(t, err)
		assert.Equal(t, shippingCode, res.ShippingCode)
		assert.Equal(t, "token_abc", res.Token)
		assert.Equal(t, "https://print.ghn.vn/a5/token_abc", res.PrintA5)
	})
}

func TestOrderService_SyncOrderTracking(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()
	shippingCode := "GHN555666"

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "member@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.SyncOrderTracking(ctx, userUUID, orderUUID)
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

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.SyncOrderTracking(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden - Other Member Cannot Sync Order", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUserID: 999, // Another owner
					OrderUuid:   orderUUID,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.SyncOrderTracking(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: No Shipping Code - Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUserID:       1,
					OrderUuid:         orderUUID,
					OrderShippingCode: nil,
				}, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			userRepo: mockUserRepo,
		})

		_, err := svc.SyncOrderTracking(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 5: Provider returns delivered - Transitions order to delivered and notifies", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:           10,
					OrderUserID:       1,
					OrderUuid:         orderUUID,
					OrderNumber:       "ORD-2026-001",
					OrderStatus:       v1service.OrderStatusShipped,
					OrderShippingCode: &shippingCode,
				}, nil
			},
			MarkDeliveredByShippingCodeFunc: func(ctx context.Context, tx pgx.Tx, sc string) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
				}, nil
			},
			MarkDeliveredByUUIDFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     10,
					OrderUserID: 1,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
				}, nil
			},
		}

		var trackingInserted bool
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				assert.Equal(t, "delivered", params.EventStatus)
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		now := time.Now()
		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			GetTrackingInfoFunc: func(ctx context.Context, sc string) (*shipping.TrackingInfo, error) {
				return &shipping.TrackingInfo{
					Status:     "delivered",
					StatusText: "Giao hàng thành công",
					Location:   "Hồ Chí Minh",
					UpdatedAt:  &now,
				}, nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			shippingFactory: factory,
			notificationService: &testutils.MockNotificationService{},
			userRepo: mockUserRepo,
		})

		res, err := svc.SyncOrderTracking(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		assert.True(t, trackingInserted)
		assert.Equal(t, shippingCode, res.ShippingCode)
		assert.Equal(t, "delivered", res.CurrentStatus)
		assert.True(t, res.IsDelivered)
	})
}

func TestOrderService_AdminSyncActiveShippingOrders(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	memberUUID := uuid.New()

	adminUser := sqlc.User{
		UserID:    1,
		UserUuid:  adminUUID,
		UserLevel: v1service.RoleAdmin,
		UserEmail: "admin@example.com",
	}

	memberUser := sqlc.User{
		UserID:    2,
		UserUuid:  memberUUID,
		UserLevel: v1service.RoleMember,
		UserEmail: "member@example.com",
	}

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.AdminSyncActiveShippingOrders(ctx, adminUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Non-Admin Forbidden - Returns 403", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return memberUser, nil
			},
		}

		svc := newTestShippingService(testShippingServiceDeps{
			userRepo: mockUserRepo,
		})

		_, err := svc.AdminSyncActiveShippingOrders(ctx, memberUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 3: Happy Path - Admin Syncs Active Orders", func(t *testing.T) {
		shippingCode := "GHN111222"
		orderUUID := uuid.New()

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetActiveShippedOrdersFunc: func(ctx context.Context, limit int32) ([]sqlc.Order, error) {
				return []sqlc.Order{
					{
						OrderID:           10,
						OrderUserID:       5,
						OrderUuid:         orderUUID,
						OrderStatus:       v1service.OrderStatusShipped,
						OrderShippingCode: &shippingCode,
					},
				}, nil
			},
			MarkDeliveredByShippingCodeFunc: func(ctx context.Context, tx pgx.Tx, sc string) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     10,
					OrderUserID: 5,
					OrderUuid:   orderUUID,
					OrderStatus: v1service.OrderStatusDelivered,
				}, nil
			},
		}

		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		now := time.Now()
		mockProvider := &testutils.MockShippingProvider{
			NameFunc: func() string { return "ghn" },
			GetTrackingInfoFunc: func(ctx context.Context, sc string) (*shipping.TrackingInfo, error) {
				return &shipping.TrackingInfo{
					Status:     "delivered",
					StatusText: "Giao thành công",
					UpdatedAt:  &now,
				}, nil
			},
		}
		factory := shipping.NewShippingFactory(mockProvider)

		svc := newTestShippingService(testShippingServiceDeps{
			orderRepo: mockOrderRepo,
			trackingRepo: mockTrackingRepo,
			shippingFactory: factory,
			notificationService: &testutils.MockNotificationService{},
			userRepo: mockUserRepo,
		})

		stats, err := svc.AdminSyncActiveShippingOrders(ctx, adminUUID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.ScannedCount)
		assert.Equal(t, 1, stats.SyncedCount)
		assert.Equal(t, 1, stats.DeliveredCount)
		assert.Equal(t, 0, stats.FailedCount)
	})
}
